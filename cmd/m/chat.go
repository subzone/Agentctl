package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/subzone/m/internal/config"
	"github.com/subzone/m/internal/engine"
	"github.com/subzone/m/internal/llm"
)

const (
	// chatPrompt prefixes each user input line.
	chatPrompt = "» "
	// chatMaxExchanges bounds how many user-initiated exchanges the session
	// retains across turns. Beyond this, the oldest are dropped to keep
	// requests within provider context limits.
	chatMaxExchanges = 8
)

func newChatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chat <agent.md>",
		Short: "Interactive REPL with an agent",
		Args:  cobra.ExactArgs(1),
		RunE:  runChat,
	}
	return cmd
}

func runChat(cmd *cobra.Command, args []string) error {
	doc, err := config.ParseFile(args[0])
	if err != nil {
		return err
	}
	return runChatWithDoc(cmd, doc, loadCompanionDocs(args[0]))
}

// runChatWithDoc drives the chat REPL against a parsed agent document.
// docs is the companion-doc set used to resolve MCP/subagent references;
// pass nil when there is no project layout (e.g. the embedded default agent).
func runChatWithDoc(cmd *cobra.Command, doc *config.Document, docs []*config.Document) error {
	agent, ok := doc.Spec.(*config.AgentSpec)
	if !ok {
		return fmt.Errorf("not an agent (type=%s)", doc.Meta().Type)
	}
	if issues := config.Validate(doc); len(issues) > 0 {
		for _, iss := range issues {
			fmt.Fprintln(cmd.ErrOrStderr(), iss.Error())
		}
		return fmt.Errorf("agent failed validation")
	}

	provider, model, err := llm.Resolve(agent.Model)
	if err != nil {
		return err
	}

	// Extract provider name for TUI display (e.g. "anthropic" from "anthropic/claude-sonnet-4-6").
	providerName, _, _ := strings.Cut(agent.Model, "/")

	// Inject the working directory so the model knows where it is.
	system := doc.Body
	if cwd, err := os.Getwd(); err == nil {
		system += fmt.Sprintf("\n\nCurrent working directory: %s", cwd)
	}

	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	out := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	hubSpawner := &spawner{
		docs:       docs,
		out:        out,
		status:     stderr,
		confirm:    stdinConfirm(stderr, cmd.InOrStdin()),
		spawnDepth: 1,
	}
	rt, err := buildAgentRuntime(ctx, agent, docs, hubSpawner, stderr)
	if err != nil {
		return err
	}
	defer rt.close()

	maxTokens := defaultMaxTokens
	if agent.MaxTokens != nil {
		maxTokens = *agent.MaxTokens
	}

	// Branch on tty: a real terminal gets the bubbletea TUI (banner +
	// stats + scrolling viewport); piped or scripted invocations fall
	// back to the line-oriented chatLoop so logs stay clean and tests
	// keep working with bytes.Buffer.
	if isInteractiveChat(cmd.InOrStdin(), out, stderr) {
		streamCh := make(chan streamMsg, 64)
		sess := engine.NewSession(engine.Config{
			Provider:    provider,
			Model:       model,
			System:      system,
			Tools:       rt.registry,
			Temperature: agent.Temperature,
			MaxTokens:   maxTokens,
			Out:         &streamWriter{ch: streamCh},
			Status:      &streamWriter{ch: streamCh}, // surface tool activity in TUI
		})
		return runTUI(ctx, sess, streamCh, agent.Name, providerName, model)
	}

	sess := engine.NewSession(engine.Config{
		Provider:    provider,
		Model:       model,
		System:      system,
		Tools:       rt.registry,
		Temperature: agent.Temperature,
		MaxTokens:   maxTokens,
		Out:         out,
		Status:      stderr,
	})
	return chatLoop(ctx, sess, cmd.InOrStdin(), out, stderr, agent.Name)
}

// isInteractiveChat reports whether all three IO streams are TTYs.
// Bubbletea needs both stdin (key reads) and stdout (rendering) to be
// terminals; we also require stderr so non-TUI output (the engine's
// status messages, when re-enabled later) renders cleanly. Anything
// else falls back to chatLoop.
func isInteractiveChat(in io.Reader, out, status io.Writer) bool {
	for _, w := range []any{in, out, status} {
		f, ok := w.(*os.File)
		if !ok || !term.IsTerminal(int(f.Fd())) {
			return false
		}
	}
	return true
}

// chatLoop drives an interactive REPL against sess. Extracted so it can be
// driven by scripted input in tests.
//
// Input arrives line-by-line on a reader goroutine so that ctx
// cancellation (typically Ctrl-C) interrupts the prompt as well as any
// in-flight Step. Slash commands (/exit, /reset, /help) are handled
// locally; everything else is passed to sess.Step.
func chatLoop(ctx context.Context, sess *engine.Session, in io.Reader, out, status io.Writer, name string) error {
	fmt.Fprintf(status, "chat with %s — /exit to quit, /reset to clear history, /help for more\n", name)

	lines := readLines(in)

	for {
		fmt.Fprint(out, chatPrompt)

		select {
		case <-ctx.Done():
			fmt.Fprintln(out)
			return nil
		case line, ok := <-lines:
			if !ok {
				fmt.Fprintln(out)
				return nil
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if handled, exit := handleSlash(line, sess, status); handled {
				if exit {
					return nil
				}
				continue
			}
			if err := sess.Step(ctx, line); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				fmt.Fprintln(status, "error:", err)
				continue
			}
			sess.Truncate(chatMaxExchanges)
		}
	}
}

// handleSlash interprets a line as a possible slash command. handled is
// true if the line was a command (or at least started with `/`); exit is
// true if the loop should terminate. Non-command input returns (false,
// false) and the caller should pass it to the model.
func handleSlash(line string, sess *engine.Session, status io.Writer) (handled, exit bool) {
	switch line {
	case "/exit", "/quit":
		return true, true
	case "/reset":
		sess.Reset()
		fmt.Fprintln(status, "(history cleared)")
		return true, false
	case "/compact":
		sess.Truncate(4)
		fmt.Fprintln(status, "(compacted to last 4 exchanges)")
		return true, false
	case "/config":
		fmt.Fprintln(status, "run `m config` from the shell to manage providers and models")
		return true, false
	case "/help":
		fmt.Fprintln(status, "commands: /exit /quit /reset /compact /model <provider/model> /theme [name] /config /help")
		return true, false
	}
	if strings.HasPrefix(line, "/model ") {
		newModel := strings.TrimSpace(strings.TrimPrefix(line, "/model "))
		p, model, err := llm.Resolve(newModel)
		if err != nil {
			fmt.Fprintf(status, "error: %v\n", err)
			return true, false
		}
		sess.SetModel(p, model)
		fmt.Fprintf(status, "switched to %s\n", newModel)
		return true, false
	}
	if strings.HasPrefix(line, "/theme") {
		arg := strings.TrimSpace(strings.TrimPrefix(line, "/theme"))
		if arg == "" {
			names := []string{}
			for n := range Builtin {
				names = append(names, n)
			}
			fmt.Fprintf(status, "themes: %s\n", strings.Join(names, ", "))
			return true, false
		}
		t := ByName(arg)
		if t == nil {
			fmt.Fprintf(status, "unknown theme %q\n", arg)
			return true, false
		}
		_ = Save(t)
		fmt.Fprintf(status, "theme set to %s (visible in TUI mode)\n", arg)
		return true, false
	}
	if strings.HasPrefix(line, "/") {
		fmt.Fprintf(status, "unknown command %q (try /help)\n", line)
		return true, false
	}
	return false, false
}

// readLines spawns a goroutine that scans lines from r and forwards them
// on the returned channel. The channel is closed on EOF or scanner error.
// Lines are emitted in the order received; the goroutine outlives the
// caller's ctx but exits naturally on input close.
func readLines(r io.Reader) <-chan string {
	ch := make(chan string, 1)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for scanner.Scan() {
			ch <- scanner.Text()
		}
	}()
	return ch
}
