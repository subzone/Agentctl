package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/subzone/Agentctl/internal/config"
	"github.com/subzone/Agentctl/internal/engine"
	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/tools"
	"github.com/subzone/Agentctl/internal/userconfig"
)

const (
	chatPrompt       = "» "
	chatMaxExchanges = 8
)

// readOnlyTUITools are auto-approved in TUI mode (no y/n prompt).
// Destructive tools (shell, fs_write, git) still require confirmation.
var readOnlyTUITools = map[string]bool{
	"fs_read":   true,
	"fs_list":   true,
	"web_fetch": true,
	"test_run":  true,
	"delegate":  true,
}

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

	undoStack := &tools.UndoStack{}
	hubSpawner := &spawner{
		docs:       docs,
		out:        out,
		status:     stderr,
		confirm:    stdinConfirm(stderr, cmd.InOrStdin()),
		undo:       undoStack,
		spawnDepth: 1,
	}

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
		statusWriter := &streamWriter{ch: streamCh}
		// In TUI mode, auto-approve writes but log them to the viewport
		// so the user sees what happened. stdin is owned by bubbletea so
		// we can't prompt interactively.
		hubSpawner.confirm = func(_ context.Context, prompt string) (bool, error) {
			statusWriter.Write([]byte("→ " + prompt + " [auto-approved in TUI]\n"))
			return true, nil
		}
		rt, err := buildAgentRuntime(ctx, agent, docs, hubSpawner, stderr)
		if err != nil {
			return err
		}
		defer rt.close()
		confirmCh := make(chan bool, 1)
		// TUI tool confirmation: auto-approve read-only tools, prompt for
		// destructive ones via the confirm channel.
		tuiToolConfirm := func(_ context.Context, name string, input json.RawMessage) (bool, error) {
			if readOnlyTUITools[name] {
				streamCh <- streamMsg{chunk: fmt.Sprintf("→ %s %s\n", name, summarizeToolInput(input))}
				return true, nil
			}
			streamCh <- streamMsg{chunk: fmt.Sprintf("→ Allow %s %s? [y/n] ", name, summarizeToolInput(input))}
			return <-confirmCh, nil
		}
		sess := engine.NewSession(engine.Config{
			Provider:         provider,
			Model:            model,
			System:           system,
			Tools:            rt.registry,
			Temperature:      agent.Temperature,
			MaxTokens:        maxTokens,
			MaxContextTokens: modelContextWindow[model],
			ToolConfirm:      tuiToolConfirm,
			ContinueConfirm: func(_ context.Context, turns int) (bool, error) {
				streamCh <- streamMsg{chunk: fmt.Sprintf("→ Agent worked for %d turns. Continue? [y/n] ", turns)}
				return <-confirmCh, nil
			},
			Out:         &streamWriter{ch: streamCh},
			Status:      &streamWriter{ch: streamCh},
		})
		return runTUI(ctx, sess, streamCh, agent.Name, providerName, model, undoStack, confirmCh)
	}

	// REPL mode: stdin-based confirmation for fs_write.
	rt, err := buildAgentRuntime(ctx, agent, docs, hubSpawner, stderr)
	if err != nil {
		return err
	}
	defer rt.close()

	replToolConfirm := stdinToolConfirm(stderr, cmd.InOrStdin())
	sess := engine.NewSession(engine.Config{
		Provider:         provider,
		Model:            model,
		System:           system,
		Tools:            rt.registry,
		Temperature:      agent.Temperature,
		MaxTokens:        maxTokens,
		MaxContextTokens: modelContextWindow[model],
		ToolConfirm:      replToolConfirm,
		ContinueConfirm: func(_ context.Context, turns int) (bool, error) {
			fmt.Fprintf(stderr, "Agent worked for %d turns. Continue? [Y/n]: ", turns)
			sc := bufio.NewScanner(cmd.InOrStdin())
			if !sc.Scan() {
				return false, nil
			}
			ans := strings.TrimSpace(strings.ToLower(sc.Text()))
			return ans == "" || ans == "y" || ans == "yes", nil
		},
		Out:         out,
		Status:      stderr,
	})
	return chatLoop(ctx, sess, undoStack, cmd.InOrStdin(), out, stderr, agent.Name)
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
func chatLoop(ctx context.Context, sess *engine.Session, undo *tools.UndoStack, in io.Reader, out, status io.Writer, name string) error {
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
			if handled, exit := handleSlash(line, sess, undo, status); handled {
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
			snap := exportSession(sess, "", sess.Model())
			go func() { autoSaveSnapshot(snap) }()
		}
	}
}

// handleSlash interprets a line as a possible slash command. handled is
// true if the line was a command (or at least started with `/`); exit is
// true if the loop should terminate. Non-command input returns (false,
// false) and the caller should pass it to the model.
func handleSlash(line string, sess *engine.Session, undo *tools.UndoStack, status io.Writer) (handled, exit bool) {
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
	case "/undo":
		if undo == nil {
			fmt.Fprintln(status, "undo not available")
			return true, false
		}
		msg, err := undo.Pop()
		if err != nil {
			fmt.Fprintf(status, "undo: %v\n", err)
		} else {
			fmt.Fprintln(status, msg)
		}
		return true, false
	case "/config":
		fmt.Fprintln(status, "run `m config` from the shell to manage providers and models")
		return true, false
	case "/spec":
		fmt.Fprintln(status, "spec-driven workflow: run `m chat examples/agents/spec.md` to use the spec agent")
		fmt.Fprintln(status, "or tell the current agent: 'follow the spec workflow for this task'")
		return true, false
	case "/help":
		fmt.Fprintln(status, "commands: /exit /quit /reset /compact /undo /model <provider/model> /models /theme [name] /themes /save /sessions /resume <id> /config /help")
		return true, false
	}
	if line == "/themes" || strings.HasPrefix(line, "/themes ") {
		themeDescriptions := []string{
			"  default    - clean blue accents",
			"  minimal    - no colors",
			"  matrix     - green on black",
			"  nord       - arctic bluish-gray",
			"  dracula    - dark, high contrast",
			"  gruvbox    - warm retro",
			"  tokyonight - clean dark",
			"  catppuccin - soothing pastel",
			"  solarized  - precision terminal",
		}
		fmt.Fprintln(status, "available themes:")
		for _, d := range themeDescriptions {
			fmt.Fprintln(status, d)
		}
		fmt.Fprintln(status, "use /theme <name> to switch")
		return true, false
	}
	if line == "/save" {
		handleSessionSave(sess, status)
		return true, false
	}
	if line == "/sessions" {
		handleSessionList(status)
		return true, false
	}
	if strings.HasPrefix(line, "/resume ") {
		handleSessionResume(line, sess, status)
		return true, false
	}
	if line == "/models" || strings.HasPrefix(line, "/models ") {
		handleModelsCommand(line, sess, status)
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

// cachedModels stores the last /models listing so /models N doesn't
// need to re-discover (expensive for token plan probing).
var cachedModels struct {
	provider string
	models   []string
}

// handleModelsCommand implements /models (list) and /models N (pick).
func handleModelsCommand(line string, sess *engine.Session, status io.Writer) {
	cfg, err := userconfig.Load()
	if err != nil {
		fmt.Fprintf(status, "error: %v\n", err)
		return
	}

	arg := strings.TrimSpace(strings.TrimPrefix(line, "/models"))

	// /models N — pick from cached listing
	if arg != "" {
		n, err := strconv.Atoi(arg)
		if err != nil {
			fmt.Fprintf(status, "usage: /models (list) or /models <number> (pick)\n")
			return
		}
		if len(cachedModels.models) == 0 || cachedModels.provider != string(cfg.Provider) {
			fmt.Fprintln(status, "run /models first to see available models")
			return
		}
		if n < 1 || n > len(cachedModels.models) {
			fmt.Fprintf(status, "pick a number between 1 and %d\n", len(cachedModels.models))
			return
		}
		picked := cachedModels.models[n-1]
		full := string(cfg.Provider) + "/" + picked
		p, model, err := llm.Resolve(full)
		if err != nil {
			fmt.Fprintf(status, "error: %v\n", err)
			return
		}
		sess.SetModel(p, model)
		fmt.Fprintf(status, "switched to %s\n", full)
		return
	}

	// /models — list available
	fmt.Fprintf(status, "scanning %s for models...\n", cfg.Provider)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	models, err := discoverModels(ctx, cfg)
	if err != nil {
		fmt.Fprintf(status, "error: %v\n", err)
		return
	}
	if len(models) == 0 {
		fmt.Fprintln(status, "no models found")
		return
	}
	// Cache for /models N
	cachedModels.provider = string(cfg.Provider)
	cachedModels.models = models
	for i, m := range models {
		marker := "  "
		if m == cfg.Model {
			marker = "* "
		}
		fmt.Fprintf(status, "  %s%d) %s/%s\n", marker, i+1, cfg.Provider, m)
	}
	fmt.Fprintln(status, "\n  * = current | /models <number> to switch")
}

func handleSessionSave(sess *engine.Session, status io.Writer) {
	store, err := sessionStore()
	if err != nil {
		fmt.Fprintf(status, "error: %v\n", err)
		return
	}
	cfg, _ := userconfig.Load()
	provider, model := "", sess.Model()
	if cfg != nil {
		provider = string(cfg.Provider)
	}
	id := sessionID("")
	data := exportSession(sess, provider, model)
	if err := store.SaveSession(context.Background(), id, data); err != nil {
		fmt.Fprintf(status, "error saving session: %v\n", err)
		return
	}
	fmt.Fprintf(status, "session saved: %s (%d messages)\n", id, len(data.Messages))
}

func handleSessionList(status io.Writer) {
	store, err := sessionStore()
	if err != nil {
		fmt.Fprintf(status, "error: %v\n", err)
		return
	}
	ids, err := store.ListSessions(context.Background())
	if err != nil {
		fmt.Fprintf(status, "error: %v\n", err)
		return
	}
	if len(ids) == 0 {
		fmt.Fprintln(status, "no saved sessions")
		return
	}
	fmt.Fprintf(status, "saved sessions (%d):\n", len(ids))
	for i, id := range ids {
		fmt.Fprintf(status, "  %d) %s\n", i+1, id)
	}
	fmt.Fprintln(status, "use /resume <id or number> to restore")
}

func handleSessionResume(line string, sess *engine.Session, status io.Writer) {
	arg := strings.TrimSpace(strings.TrimPrefix(line, "/resume"))
	if arg == "" {
		fmt.Fprintln(status, "usage: /resume <session-id or number>")
		return
	}
	store, err := sessionStore()
	if err != nil {
		fmt.Fprintf(status, "error: %v\n", err)
		return
	}
	// Allow picking by number from /sessions listing.
	if n, nerr := strconv.Atoi(arg); nerr == nil {
		ids, lerr := store.ListSessions(context.Background())
		if lerr != nil {
			fmt.Fprintf(status, "error: %v\n", lerr)
			return
		}
		if n < 1 || n > len(ids) {
			fmt.Fprintf(status, "pick a number between 1 and %d\n", len(ids))
			return
		}
		arg = ids[n-1]
	}
	data, err := store.LoadSession(context.Background(), arg)
	if err != nil {
		fmt.Fprintf(status, "error: %v\n", err)
		return
	}
	msgs := importSession(data)
	sess.RestoreMessages(msgs)
	sess.SetUsage(llm.Usage{InputTokens: data.InputTokens, OutputTokens: data.OutputTokens})
	fmt.Fprintf(status, "resumed session %s (%d messages, %s/%s)\n", arg, len(msgs), data.Provider, data.Model)
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
