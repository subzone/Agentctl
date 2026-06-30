package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	cmdoutput "github.com/subzone/Agentctl/cmd/m/output"
	"github.com/subzone/Agentctl/internal/audit"
	"github.com/subzone/Agentctl/internal/config"
	"github.com/subzone/Agentctl/internal/engine"
	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/policy"
	"github.com/subzone/Agentctl/internal/ports"
	"github.com/subzone/Agentctl/internal/tools"
	"github.com/subzone/Agentctl/internal/userconfig"
)

// defaultMaxTokens is used when an agent's frontmatter doesn't specify one.
// Zero means the provider picks its own default (e.g. Anthropic uses 16384,
// OpenAI and Ollama use their server-side defaults).
const defaultMaxTokens = 0

const (
	runExitAgentError      = 1
	runExitPolicyViolation = 2
	runExitBudgetExceeded  = 3
	runExitTimeout         = 4
)

func newRunCmd() *cobra.Command {
	var yes bool
	var dryRun bool
	var ciMode bool
	var outputMode string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "run <agent.md> [task...]",
		Short: "Run an agent once and stream the reply to stdout",
		Long: `Execute a one-shot task with an agent. Output streams to stdout.

Examples:
  m run devops "review the Dockerfile"
  m run --yes devops "apply the terraform plan"
  m run --dry-run coder "refactor auth"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return runDryRun(cmd, args)
			}
			return runAgent(cmd, args, yes, ciMode, outputMode, timeout)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Auto-approve all tool calls (dangerous commands still blocked)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate agent and show config without calling the LLM")
	cmd.Flags().BoolVar(&ciMode, "ci", false, "Enable CI mode (implies --yes and JSON output by default)")
	cmd.Flags().StringVar(&outputMode, "output", "text", "Output mode: text|json")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Maximum run duration, e.g. 30s, 5m")
	return cmd
}

func runAgent(cmd *cobra.Command, args []string, autoApprove bool, ciMode bool, outputMode string, timeout time.Duration) error {
	if ciMode {
		autoApprove = true
		if outputMode == "text" {
			outputMode = "json"
		}
		if timeout == 0 {
			timeout = 15 * time.Minute
		}
	}
	if outputMode != "text" && outputMode != "json" {
		return exitError(runExitAgentError, fmt.Errorf("invalid --output %q (expected text or json)", outputMode))
	}
	if timeout < 0 {
		return exitError(runExitAgentError, fmt.Errorf("--timeout cannot be negative"))
	}

	path := resolveAgentPath(args[0])
	doc, err := config.ParseFile(path)
	if err != nil {
		return exitError(runExitAgentError, err)
	}
	agent, ok := doc.Spec.(*config.AgentSpec)
	if !ok {
		return exitError(runExitAgentError, fmt.Errorf("%s: not an agent (type=%s)", args[0], doc.Meta().Type))
	}
	if issues := config.Validate(doc); len(issues) > 0 {
		for _, iss := range issues {
			fmt.Fprintln(cmd.ErrOrStderr(), iss.Error())
		}
		return exitError(runExitAgentError, fmt.Errorf("agent failed validation"))
	}

	task, err := readTask(cmd, args[1:])
	if err != nil {
		return exitError(runExitAgentError, err)
	}

	provider, model, err := llm.Resolve(agent.Model)
	if err != nil {
		return exitError(runExitAgentError, err)
	}

	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	rawOut := cmd.OutOrStdout()
	rawErr := cmd.ErrOrStderr()
	out := rawOut
	stderr := rawErr

	sessionID := fmt.Sprintf("s%d", time.Now().UTC().UnixNano())
	auditSink, err := buildAuditSink()
	if err != nil {
		return exitError(runExitAgentError, err)
	}
	defer auditSink.Close()
	auditSessionStart(ctx, auditSink, sessionID, agent.Name, model)

	var jsonEmitter *cmdoutput.JSONEmitter
	var statusTap io.Writer
	if outputMode == "json" {
		jsonEmitter = cmdoutput.NewJSONEmitter(rawOut)
		jsonEmitter.SessionStart(sessionID, agent.Name)
		statusTap = cmdoutput.NewStatusEventWriter(jsonEmitter)
		out = io.Discard
		stderr = io.Discard
	} else {
		statusTap = io.Discard
	}

	statusOut := io.MultiWriter(stderr, statusTap)

	docs := loadCompanionDocs(path)

	hubSpawner := &spawner{
		docs:       docs,
		out:        out,
		status:     statusOut,
		confirm:    stdinConfirm(statusOut, os.Stdin),
		undo:       &tools.UndoStack{},
		spawnDepth: 1, // children of the hub run at depth 1
	}
	if autoApprove {
		hubSpawner.confirm = func(_ context.Context, prompt string) (bool, error) {
			fmt.Fprintf(statusOut, "→ %s [auto-approved]\n", prompt)
			return true, nil
		}
	}

	rt, err := buildAgentRuntime(ctx, agent, docs, hubSpawner, statusOut)
	if err != nil {
		return exitError(runExitAgentError, err)
	}
	defer rt.close()

	maxTokens := defaultMaxTokens
	if agent.MaxTokens != nil {
		maxTokens = *agent.MaxTokens
	}

	var toolConfirm func(context.Context, string, json.RawMessage) (bool, error)
	if autoApprove {
		toolConfirm = func(_ context.Context, name string, input json.RawMessage) (bool, error) {
			if pattern := isDangerousCommand(name, input); pattern != "" {
				fmt.Fprintf(statusOut, "\n⚠️  DANGEROUS: \"%s\" in %s — skipped in --yes mode\n", pattern, name)
				return false, nil
			}
			fmt.Fprintf(statusOut, "→ %s %s [auto-approved]\n", name, summarizeToolInput(input))
			return true, nil
		}
	}

	policyCheck, err := buildPolicyCheck()
	if err != nil {
		return exitError(runExitAgentError, err)
	}

	sess := engine.NewSession(engine.Config{
		Provider:       provider,
		Model:          model,
		System:         systemWithCwd(doc.Body),
		Tools:          rt.registry,
		Temperature:    agent.Temperature,
		MaxTokens:      maxTokens,
		ToolConfirm:    toolConfirm,
		PolicyCheck:    policyCheck,
		Audit:          auditSink,
		AuditSessionID: sessionID,
		Out:            out,
		Status:         statusOut,
	})
	err = sess.Step(ctx, task)
	usage := sess.Usage()
	cost := estimateCost(usage, model)
	if jsonEmitter != nil {
		jsonEmitter.LLMResponse(usage.InputTokens, usage.OutputTokens, cost)
	}
	if err != nil {
		auditSessionEnd(ctx, auditSink, sessionID, "failure", model, usage, cost, err)
		if jsonEmitter != nil {
			jsonEmitter.SessionEnd(sessionID, "failure", len(sess.Messages()), cost)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return exitError(runExitTimeout, err)
		}
		var pErr *engine.PolicyViolationError
		if errors.As(err, &pErr) {
			return exitError(runExitPolicyViolation, err)
		}
		return exitError(runExitAgentError, err)
	}
	auditSessionEnd(ctx, auditSink, sessionID, "success", model, usage, cost, nil)
	if jsonEmitter != nil {
		jsonEmitter.SessionEnd(sessionID, "success", len(sess.Messages()), cost)
	}
	return nil
}

func buildAuditSink() (ports.AuditSink, error) {
	homeCfg, err := userconfig.Path()
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	projectCfg := audit.FindProjectConfigPath(cwd)
	cfg, err := audit.LoadConfig(homeCfg, projectCfg)
	if err != nil {
		return nil, fmt.Errorf("load audit config: %w", err)
	}
	if cfg.Backend == "" || cfg.Backend == "none" {
		return audit.NewNoopSink(), nil
	}
	if cfg.Backend == "splunk" {
		base, err := audit.NewSplunkSink(cfg.Endpoint, cfg.Token, cfg.TLSVerify)
		if err != nil {
			return nil, err
		}
		return audit.NewBatchSink(base, cfg.BatchSize, cfg.FlushInterval)
	}
	if cfg.Backend != "file" {
		return nil, fmt.Errorf("unsupported audit backend %q", cfg.Backend)
	}
	path := cfg.Path
	if path == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(base, "m", "audit.jsonl")
	}
	base, err := audit.NewFileSink(path, cfg.HMACSecret)
	if err != nil {
		return nil, err
	}
	return audit.NewBatchSink(base, cfg.BatchSize, cfg.FlushInterval)
}

func auditSessionStart(ctx context.Context, sink ports.AuditSink, sessionID string, agent string, model string) {
	if sink == nil {
		return
	}
	_ = sink.Emit(ctx, ports.AuditEvent{
		Type:      "session_start",
		SessionID: sessionID,
		Agent:     agent,
		Model:     model,
	})
}

func auditSessionEnd(ctx context.Context, sink ports.AuditSink, sessionID string, outcome string, model string, usage llm.Usage, cost float64, runErr error) {
	if sink == nil {
		return
	}
	ev := ports.AuditEvent{
		Type:         "session_end",
		SessionID:    sessionID,
		Model:        model,
		Outcome:      outcome,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		CostUSD:      cost,
	}
	if runErr != nil {
		ev.Error = runErr.Error()
	}
	_ = sink.Emit(ctx, ev)
	_ = sink.Flush(ctx)
}

func buildPolicyCheck() (func(context.Context, string, json.RawMessage) (string, error), error) {
	homeCfg, err := userconfig.Path()
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	projectCfg := policy.FindProjectConfigPath(cwd)
	rules, err := policy.LoadRules(homeCfg, projectCfg)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("load policy rules: %w", err)
	}
	if len(rules) == 0 {
		return nil, nil
	}
	eng, err := policy.NewInlineEngine(rules)
	if err != nil {
		return nil, err
	}
	return func(_ context.Context, toolName string, input json.RawMessage) (string, error) {
		return eng.Check(toolName, input), nil
	}, nil
}

// loadCompanionDocs returns every parseable MD doc in the agent's project
// root. Parse failures are silently dropped (the user surfaces them via
// `agent validate`); resolve-time misses are reported individually.
func loadCompanionDocs(agentPath string) []*config.Document {
	var docs []*config.Document
	seen := map[string]bool{}
	seenMCP := map[string]bool{}

	// 1. Scan the agent's project root.
	if root, err := projectRoot(agentPath); err == nil {
		if found, _ := config.LoadDir(root); len(found) > 0 {
			for _, d := range found {
				switch s := d.Spec.(type) {
				case *config.AgentSpec:
					seen[s.Name] = true
				case *config.MCPServerSpec:
					seenMCP[s.Name] = true
				}
				docs = append(docs, d)
			}
		}
	}

	// 2. Also scan the user's agent registry dir (bundled/installed agents).
	if dir := agentRegistryDir(); dir != "" {
		if found, _ := config.LoadDir(dir); len(found) > 0 {
			for _, d := range found {
				if a, ok := d.Spec.(*config.AgentSpec); ok && !seen[a.Name] {
					seen[a.Name] = true
					docs = append(docs, d)
				}
			}
		}
	}

	// 3. Scan the installed MCP registry so agents can resolve MCP servers
	// (e.g. knowledge-master) by name even when invoked outside a project.
	if dir := mcpRegistryDir(); dir != "" {
		if found, _ := config.LoadDir(dir); len(found) > 0 {
			for _, d := range found {
				if m, ok := d.Spec.(*config.MCPServerSpec); ok && !seenMCP[m.Name] {
					seenMCP[m.Name] = true
					docs = append(docs, d)
				}
			}
		}
	}

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

// systemWithCwd appends the current working directory to a system prompt.
func systemWithCwd(body string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return body
	}
	result := body + fmt.Sprintf("\n\nCurrent working directory: %s", cwd)
	if ctx := detectProjectContext(cwd); ctx != "" {
		result += ctx
	}
	return result
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

// summarizeToolInput returns a short preview of tool input for confirmation prompts.
func summarizeToolInput(input json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(input, &m); err != nil {
		s := string(input)
		if len(s) > 60 {
			return s[:60] + "..."
		}
		return s
	}
	var parts []string
	for _, key := range []string{"path", "command", "url", "query", "name", "file"} {
		if v, ok := m[key]; ok {
			s := fmt.Sprintf("%v", v)
			if len(s) > 60 {
				s = s[:60] + "..."
			}
			parts = append(parts, key+"="+s)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	// Fallback: show first key=value
	for k, v := range m {
		s := fmt.Sprintf("%v", v)
		if len(s) > 40 {
			s = s[:40] + "..."
		}
		return k + "=" + s
	}
	return string(input)
}

// runDryRun validates the agent, resolves the provider, and prints the
// resolved configuration without making any LLM calls.
func runDryRun(cmd *cobra.Command, args []string) error {
	path := resolveAgentPath(args[0])
	doc, err := config.ParseFile(path)
	if err != nil {
		return err
	}
	agent, ok := doc.Spec.(*config.AgentSpec)
	if !ok {
		return fmt.Errorf("%s: not an agent (type=%s)", args[0], doc.Meta().Type)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Agent:       %s\n", agent.Name)
	fmt.Fprintf(out, "Path:        %s\n", path)
	fmt.Fprintf(out, "Model:       %s\n", agent.Model)
	if len(agent.FallbackModels) > 0 {
		fmt.Fprintf(out, "Fallbacks:   %s\n", strings.Join(agent.FallbackModels, ", "))
	}
	if agent.Temperature != nil {
		fmt.Fprintf(out, "Temperature: %.1f\n", *agent.Temperature)
	}
	fmt.Fprintf(out, "Tools:       %s\n", strings.Join(agent.Tools, ", "))

	if issues := config.Validate(doc); len(issues) > 0 {
		fmt.Fprintf(out, "\nValidation issues:\n")
		for _, iss := range issues {
			fmt.Fprintf(out, "  ⚠ %s\n", iss.Error())
		}
		return fmt.Errorf("%d validation issue(s)", len(issues))
	}
	fmt.Fprintf(out, "\n✓ Agent valid\n")

	// Try resolving the provider.
	_, model, err := llm.Resolve(agent.Model)
	if err != nil {
		fmt.Fprintf(out, "✗ Provider: %v\n", err)
		return err
	}
	fmt.Fprintf(out, "✓ Provider resolved (model: %s)\n", model)

	if len(args) > 1 {
		task := strings.Join(args[1:], " ")
		fmt.Fprintf(out, "\nTask:        %s\n", task)
		fmt.Fprintf(out, "System:      %d chars\n", len(doc.Body))
	}

	fmt.Fprintf(out, "\nDry run complete — no LLM calls made.\n")
	return nil
}
