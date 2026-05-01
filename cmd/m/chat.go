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

	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	out := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	hubSpawner := &spawner{
		docs:       docs,
		out:        out,
		status:     stderr,
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

	sess := engine.NewSession(engine.Config{
		Provider:    provider,
		Model:       model,
		System:      doc.Body,
		Tools:       rt.registry,
		Temperature: agent.Temperature,
		MaxTokens:   maxTokens,
		Out:         out,
		Status:      stderr,
	})

	return chatLoop(ctx, sess, cmd.InOrStdin(), out, stderr, agent.Name)
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
	case "/help":
		fmt.Fprintln(status, "commands: /exit, /quit, /reset, /help")
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
