package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/subzone/m/internal/config"
	"github.com/subzone/m/internal/engine"
	"github.com/subzone/m/internal/llm"
	"github.com/subzone/m/internal/tools"
)

// defaultMaxTokens is used when an agent's frontmatter doesn't specify one.
// Zero means the provider picks its own default (e.g. Anthropic uses 16384,
// OpenAI and Ollama use their server-side defaults).
const defaultMaxTokens = 0

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <agent.md> [task...]",
		Short: "Run an agent once and stream the reply to stdout",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runAgent,
	}
	return cmd
}

func runAgent(cmd *cobra.Command, args []string) error {
	doc, err := config.ParseFile(args[0])
	if err != nil {
		return err
	}
	agent, ok := doc.Spec.(*config.AgentSpec)
	if !ok {
		return fmt.Errorf("%s: not an agent (type=%s)", args[0], doc.Meta().Type)
	}
	if issues := config.Validate(doc); len(issues) > 0 {
		for _, iss := range issues {
			fmt.Fprintln(cmd.ErrOrStderr(), iss.Error())
		}
		return fmt.Errorf("agent failed validation")
	}

	task, err := readTask(cmd, args[1:])
	if err != nil {
		return err
	}

	provider, model, err := llm.Resolve(agent.Model)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	out := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	docs := loadCompanionDocs(args[0])

	hubSpawner := &spawner{
		docs:       docs,
		out:        out,
		status:     stderr,
		confirm:    stdinConfirm(stderr, os.Stdin),
		spawnDepth: 1, // children of the hub run at depth 1
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

	return engine.Run(ctx, engine.Config{
		Provider:    provider,
		Model:       model,
		System:      doc.Body,
		Tools:       rt.registry,
		Temperature: agent.Temperature,
		MaxTokens:   maxTokens,
		Out:         out,
		Status:      stderr,
	}, task)
}

// loadCompanionDocs returns every parseable MD doc in the agent's project
// root. Parse failures are silently dropped (the user surfaces them via
// `agent validate`); resolve-time misses are reported individually.
func loadCompanionDocs(agentPath string) []*config.Document {
	root, err := projectRoot(agentPath)
	if err != nil {
		return nil
	}
	docs, _ := config.LoadDir(root)
	return docs
}

// projectRoot returns the directory we'll scan for companion docs. If the
// agent file lives in a project layout with sibling subdirectories
// (examples/agents/foo.md, examples/mcp/bar.md), use the parent. Otherwise
// use the agent's own directory.
func projectRoot(agentPath string) (string, error) {
	abs, err := filepath.Abs(agentPath)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(abs)
	parent := filepath.Dir(dir)
	if parent == dir {
		return dir, nil
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		return dir, nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		full := filepath.Join(parent, e.Name())
		if full != dir {
			return parent, nil
		}
	}
	return dir, nil
}

// readTask returns the task string, preferring positional args. If none are
// given and stdin is piped, it reads stdin instead.
func readTask(cmd *cobra.Command, rest []string) (string, error) {
	if joined := strings.TrimSpace(strings.Join(rest, " ")); joined != "" {
		return joined, nil
	}
	if !stdinIsPipe(cmd.InOrStdin()) {
		return "", fmt.Errorf("no task provided (pass as args or pipe via stdin)")
	}
	b, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	t := strings.TrimSpace(string(b))
	if t == "" {
		return "", fmt.Errorf("stdin was empty")
	}
	return t, nil
}

// stdinConfirm returns a ConfirmFunc that prints the prompt to w and reads
// y/n from r. Used by both `m run` and the line-oriented `m chat` REPL.
func stdinConfirm(w io.Writer, r io.Reader) tools.ConfirmFunc {
	sc := bufio.NewScanner(r)
	return func(_ context.Context, prompt string) (bool, error) {
		fmt.Fprintf(w, "%s [y/N]: ", prompt)
		if !sc.Scan() {
			return false, nil
		}
		ans := strings.TrimSpace(strings.ToLower(sc.Text()))
		return ans == "y" || ans == "yes", nil
	}
}

// stdinIsPipe reports whether r is a piped *os.File (i.e. not a terminal).
// For non-*os.File readers (test fakes) we assume it's pipe-like.
func stdinIsPipe(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return true
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) == 0
}

