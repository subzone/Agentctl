
// Package desktop provides the bridge between the Wails GUI and the
// AgentCTL engine. It exposes engine functionality as methods that can
// be called from the frontend via Wails bindings, and emits events for
// streaming responses and tool activity.

package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/subzone/Agentctl/examples"
	"github.com/subzone/Agentctl/internal/config"
	"github.com/subzone/Agentctl/internal/engine"
	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/tools"
	"github.com/subzone/Agentctl/internal/userconfig"
)

// App manages the desktop application state.
type App struct {
	ctx      context.Context
	cfg      *userconfig.Config
	version  string
	sessions map[string]*Session
	mu       sync.RWMutex

	// pending holds in-flight tool-approval requests keyed by request id.
	// The engine blocks on the channel until the frontend answers via
	// RespondToolApproval.
	pending   map[string]chan bool
	pendingMu sync.Mutex
}

// Session wraps an engine.Session with desktop-specific state.
type Session struct {
	ID        string    `json:"id"`
	Agent     string    `json:"agent"`
	Model     string    `json:"model"`
	CreatedAt int64     `json:"createdAt"`
	Messages  []Message `json:"messages"`
	engine    *engine.Session
	cancel    context.CancelFunc
	cleanups  []func() error // e.g. MCP managers to close on session end
	mu        sync.RWMutex
}

// Message represents a chat message for the frontend.
type Message struct {
	ID        string     `json:"id"`
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Timestamp int64      `json:"timestamp"`
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
}

// ToolCall represents a tool execution.
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Input    string `json:"input"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
	Duration int64  `json:"duration"` // milliseconds
	Status   string `json:"status"`   // "running", "done", "error", "declined"
}

// AgentInfo describes an available agent.
type AgentInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Model       string `json:"model"`
	Path        string `json:"path"`
	Builtin     bool   `json:"builtin"`
	Category    string `json:"category"` // "hub", "spoke", "standalone"
}

// MCPStatus describes an MCP server's state.
type MCPStatus struct {
	Name      string `json:"name"`
	Transport string `json:"transport"`
	Installed bool   `json:"installed"`
}

// SessionInfo is a summary for the session list.
type SessionInfo struct {
	ID        string `json:"id"`
	Agent     string `json:"agent"`
	Model     string `json:"model"`
	Messages  int    `json:"messages"`
	CreatedAt int64  `json:"createdAt"`
	Preview   string `json:"preview"` // first user message
}

// CostInfo shows token usage and cost.
type CostInfo struct {
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	Cost         float64 `json:"cost"`
	Model        string  `json:"model"`
}

func NewApp() *App {
	return &App{
		version:  "0.5.0",
		sessions: make(map[string]*Session),
		pending:  make(map[string]chan bool),
	}
}

// SetProductVersion sets the build version (from main.Version ldflags).
func (a *App) SetProductVersion(v string) {
	if v != "" && v != "dev" {
		a.version = v
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	cfg, _ := userconfig.Load()
	if cfg == nil {
		cfg = &userconfig.Config{}
	}
	a.cfg = cfg

	// Apply config: load API keys from keychain into env vars.
	if cfg.Provider != "" {
		if key, err := userconfig.GetAPIKeyWithFallback(cfg.Provider); err == nil && key != "" {
			setProviderKeyEnv(cfg.Provider, key)
		}
		if cfg.BaseURL != "" {
			switch cfg.Provider {
			case userconfig.ProviderAlibaba:
				os.Setenv("DASHSCOPE_BASE_URL", cfg.BaseURL)
			case userconfig.ProviderLiteLLM:
				os.Setenv("LITELLM_BASE_URL", cfg.BaseURL)
			case userconfig.ProviderOllama:
				os.Setenv("OLLAMA_HOST", cfg.BaseURL)
			}
		}
	}

	// The bundled default agent and the MoE example agents route across the
	// free-tier providers (Groq, Cerebras, Mistral, Gemini). Hydrate every
	// free key we have stored — independent of the primary provider — so
	// routing and fallback don't fail with "<KEY> is not set".
	for provider, env := range map[userconfig.Provider]string{
		userconfig.ProviderGroq:     "GROQ_API_KEY",
		userconfig.ProviderCerebras: "CEREBRAS_API_KEY",
		userconfig.ProviderMistral:  "MISTRAL_API_KEY",
		userconfig.ProviderGemini:   "GEMINI_API_KEY",
	} {
		if os.Getenv(env) != "" {
			continue
		}
		if key, err := userconfig.GetAPIKeyWithFallback(provider); err == nil && key != "" {
			os.Setenv(env, key)
		}
	}
}

// --- Config ---

func (a *App) GetConfig() *userconfig.Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

func (a *App) SaveConfig(provider, model, baseURL string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg := &userconfig.Config{
		Provider: userconfig.Provider(provider),
		Model:    model,
		BaseURL:  baseURL,
	}
	if err := userconfig.Save(cfg); err != nil {
		return err
	}
	a.cfg = cfg
	return nil
}

func (a *App) SaveAPIKey(provider, key string) error {
	if err := userconfig.SaveAPIKey(userconfig.Provider(provider), key); err != nil {
		return err
	}
	// Also export it into the running process so new sessions pick it up
	// without requiring an app restart.
	setProviderKeyEnv(userconfig.Provider(provider), key)
	return nil
}

// setProviderKeyEnv maps a provider to the environment variable its LLM
// adapter reads and sets it for the current process.
func setProviderKeyEnv(provider userconfig.Provider, key string) {
	envByProvider := map[userconfig.Provider]string{
		userconfig.ProviderAnthropic: "ANTHROPIC_API_KEY",
		userconfig.ProviderOpenAI:    "OPENAI_API_KEY",
		userconfig.ProviderGemini:    "GEMINI_API_KEY",
		userconfig.ProviderAlibaba:   "DASHSCOPE_API_KEY",
		userconfig.ProviderLiteLLM:   "LITELLM_API_KEY",
		userconfig.ProviderGroq:      "GROQ_API_KEY",
		userconfig.ProviderCerebras:  "CEREBRAS_API_KEY",
		userconfig.ProviderMistral:   "MISTRAL_API_KEY",
	}
	if env, ok := envByProvider[provider]; ok && key != "" {
		os.Setenv(env, key)
	}
}

// --- Agents ---

func (a *App) ListAgents() []AgentInfo {
	var agents []AgentInfo
	seen := map[string]bool{}

	// Bundled agents from embedded FS.
	fs.WalkDir(examples.Agents, "agents", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		data, _ := examples.Agents.ReadFile(path)
		if data == nil {
			return nil
		}
		doc, err := config.Parse(data)
		if err != nil {
			return nil
		}
		spec, ok := doc.Spec.(*config.AgentSpec)
		if !ok {
			return nil
		}
		seen[spec.Name] = true
		cat := "standalone"
		if len(spec.Subagents) > 0 {
			cat = "hub"
		}
		if strings.HasPrefix(spec.Name, "spoke-") {
			cat = "spoke"
		}
		// The bundled MoE router is the recommended default; surface it first.
		if spec.Name == "m" {
			cat = "default"
		}
		agents = append(agents, AgentInfo{
			Name:        spec.Name,
			Description: spec.Description,
			Model:       spec.Model,
			Path:        path,
			Builtin:     true,
			Category:    cat,
		})
		return nil
	})

	// User agents from config dir.
	if dir := userAgentsDir(); dir != "" {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			p := filepath.Join(dir, e.Name())
			doc, err := config.ParseFile(p)
			if err != nil {
				continue
			}
			spec, ok := doc.Spec.(*config.AgentSpec)
			if !ok || seen[spec.Name] {
				continue
			}
			agents = append(agents, AgentInfo{
				Name:        spec.Name,
				Description: spec.Description,
				Model:       spec.Model,
				Path:        p,
				Builtin:     false,
				Category:    "standalone",
			})
		}
	}
	return agents
}

// --- Sessions ---

func (a *App) CreateSession(agentName string) (*SessionInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Find agent spec.
	spec, body, err := a.resolveAgent(agentName)
	if err != nil {
		return nil, err
	}

	// Resolve provider.
	model := spec.Model
	if model == "" && a.cfg != nil {
		model = fmt.Sprintf("%s/%s", a.cfg.Provider, a.cfg.Model)
	}
	provider, resolvedModel, err := llm.Resolve(model)
	if err != nil {
		return nil, fmt.Errorf("resolve model: %w", err)
	}

	// Build tools. Pass a nil confirm func: the engine-level ToolConfirm
	// below is the single authoritative approval gate, so fs_write must not
	// prompt a second time. knowledge-master tools (km_search, km_index,
	// km_blast_radius, km_status) are merged in so the chat agent can query
	// and grow the same knowledge graph shown in the Knowledge tab.
	reg := tools.Merge(
		tools.Builtins(nil, &tools.UndoStack{}),
		tools.NewRegistry(tools.KMTools()...),
	)
	// User-authored tools (MD files in ~/.config/m/tools) are loaded fresh
	// each session so edits take effect on the next New Chat without a restart.
	if custom, errs := tools.LoadCommandTools(userToolsDir()); len(custom) > 0 {
		reg = tools.Merge(reg, tools.NewRegistry(custom...))
		_ = errs
	}

	// User-authored skills (MD files in ~/.config/m/skills) are composed into
	// the system prompt fresh each session, so authoring/editing a skill takes
	// effect on the next New Chat without a restart.
	system := composeAgentSystem(body, agentName)

	// Per-agent personality overlay: temperature override (text is in system).
	persona := loadPersona(agentName)
	temperature := spec.Temperature
	if persona.Temperature != nil {
		temperature = persona.Temperature
	}

	maxTokens := 0
	if spec.MaxTokens != nil {
		maxTokens = *spec.MaxTokens
	}

	id := fmt.Sprintf("s_%d", time.Now().UnixMilli())
	auditSink, _ := buildDesktopAuditSink(a.ctx, id)
	policyCheck, _ := buildDesktopPolicyCheck()

	// Create streaming writer that emits events to frontend. Output (assistant
	// tokens) and status (engine diagnostics like MoE routing) go to separate
	// events so status lines never pollute the visible assistant message.
	sw := &streamWriter{ctx: a.ctx, sessionID: id}
	stw := &statusWriter{ctx: a.ctx, sessionID: id}

	// User-configured MCP servers (~/.config/m/mcp/*.md) are opened and their
	// tools (namespaced by tool_prefix) merged in, so the agent can use them
	// just like builtins. They're torn down when the session is closed.
	var cleanups []func() error
	if mgr := a.openUserMCP(agentName, stw); mgr != nil {
		reg = tools.Merge(reg, mgr.Registry())
		cleanups = append(cleanups, mgr.Close)
	}

	sess := engine.NewSession(engine.Config{
		Provider:    provider,
		Model:       resolvedModel,
		System:      system,
		Tools:       reg,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		// Wire MoE routing and the fallback chain so the default "m" agent
		// behaves like it does in the CLI: each turn is routed to the best
		// expert model, and a 429/overload on a free tier transparently
		// fails over to the next provider instead of killing the chat.
		Routing:        spec.Routing,
		FallbackModels: spec.FallbackModels,
		ResolveModel:   llm.Resolve,
		// Interactive desktop sessions explore more than one-shot CLI runs, so
		// give the tool loop more headroom before pausing, and ask the user
		// whether to keep going instead of hard-stopping at the cap.
		MaxTurns: 40,
		ToolConfirm: func(ctx context.Context, name string, input json.RawMessage) (bool, error) {
			// Read-only tools (search/read/list/status) run without prompting;
			// only mutating or shell-executing tools need explicit approval.
			auto := readOnlyTools[name]
			approved := auto
			var err error
			if !auto {
				approved, err = a.requestToolApproval(ctx, id, name, input)
			}
			if err == nil {
				emitActivity(a.ctx, id, "tool", name, map[string]any{
					"input": string(input),
					"auto":  auto,
					"ok":    approved,
				})
			}
			return approved, err
		},
		ContinueConfirm: func(ctx context.Context, turns int) (bool, error) {
			return a.requestContinue(ctx, id, turns)
		},
		PolicyCheck:      policyCheck,
		Audit:            auditSink,
		AuditSessionID:   id,
		Out:    sw,
		Status: stw,
	})

	session := &Session{
		ID:        id,
		Agent:     spec.Name,
		Model:     model,
		CreatedAt: time.Now().Unix(),
		engine:    sess,
		cleanups:  cleanups,
	}
	a.sessions[id] = session

	return &SessionInfo{
		ID:        id,
		Agent:     spec.Name,
		Model:     model,
		CreatedAt: session.CreatedAt,
	}, nil
}

func (a *App) SendMessage(sessionID, message string) error {
	return a.runEngineStep(sessionID, message, true)
}

func (a *App) StopGeneration(sessionID string) {
	a.mu.RLock()
	sess, ok := a.sessions[sessionID]
	a.mu.RUnlock()
	if ok && sess.cancel != nil {
		sess.cancel()
	}
}

func (a *App) GetSessions() []SessionInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var infos []SessionInfo
	for _, s := range a.sessions {
		preview := ""
		for _, m := range s.Messages {
			if m.Role == "user" {
				preview = m.Content
				if len(preview) > 60 {
					preview = preview[:57] + "..."
				}
				break
			}
		}
		infos = append(infos, SessionInfo{
			ID:        s.ID,
			Agent:     s.Agent,
			Model:     s.Model,
			Messages:  len(s.Messages),
			CreatedAt: s.CreatedAt,
			Preview:   preview,
		})
	}
	return infos
}

func (a *App) GetCost(sessionID string) *CostInfo {
	a.mu.RLock()
	sess, ok := a.sessions[sessionID]
	a.mu.RUnlock()
	if !ok {
		return nil
	}
	usage := sess.engine.Usage()
	cost := calcCost(sess.Model, usage.InputTokens, usage.OutputTokens)
	return &CostInfo{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		Cost:         cost,
		Model:        sess.Model,
	}
}

// calcCost estimates cost in USD from token counts.
func calcCost(model string, in, out int) float64 {
	if i := strings.LastIndex(model, "/"); i >= 0 {
		model = model[i+1:]
	}
	type rate struct{ in, out float64 }
	pricing := map[string]rate{
		"claude-sonnet-4-6":         {3.0, 15.0},
		"claude-haiku-4-5-20251001": {1.0, 5.0},
		"claude-opus-4":             {15.0, 75.0},
		"gpt-4o":                    {2.50, 10.0},
		"gpt-4.1":                   {2.0, 8.0},
		"gpt-4o-mini":               {0.15, 0.60},
		"gemini-2.5-pro":            {1.25, 10.0},
		"gemini-2.5-flash":          {0.15, 0.60},
		"qwen-plus":                 {0.80, 2.0},
		"qwen-max":                  {2.0, 6.0},
		"qwen-turbo":                {0.30, 0.60},
		"deepseek-v3.2":             {0.27, 1.10},
		"glm-5":                     {0.50, 0.50},
		"qwen3.6-plus":              {0.80, 2.0},
		"qwen3.6-flash":             {0.15, 0.60},
	}
	p, ok := pricing[model]
	if !ok {
		return 0
	}
	return (float64(in)*p.in + float64(out)*p.out) / 1_000_000
}

// FileResult is returned by OpenFile.
type FileResult struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// OpenFile opens a native file picker and returns the file path and content.
func (a *App) OpenFile() (*FileResult, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select file to attach",
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	content := string(data)
	if len(content) > 50000 {
		content = content[:50000] + "\n[truncated]"
	}
	return &FileResult{
		Path:    path,
		Name:    filepath.Base(path),
		Content: content,
	}, nil
}

func (a *App) CloseSession(sessionID string) {
	a.mu.Lock()
	s, ok := a.sessions[sessionID]
	if ok {
		if s.cancel != nil {
			s.cancel()
		}
		for _, c := range s.cleanups {
			_ = c()
		}
	}
	delete(a.sessions, sessionID)
	a.mu.Unlock()
	if ok {
		a.persistSessionToDisk(s)
	}
}

func (a *App) SwitchAgent(sessionID, agentName string) error {
	a.mu.RLock()
	sess, ok := a.sessions[sessionID]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found")
	}

	spec, body, err := a.resolveAgent(agentName)
	if err != nil {
		return err
	}

	provider, model, err := llm.Resolve(spec.Model)
	if err != nil {
		return err
	}

	// Recompose the system prompt with user skills and this agent's persona so
	// switching agents in-place behaves like a fresh session would.
	system := composeAgentSystem(body, agentName)

	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.engine.SetModel(provider, model)
	sess.engine.SetSystem(system)
	sess.engine.SetRouting(spec.Routing)
	sess.engine.SetFallbacks(spec.FallbackModels, llm.Resolve)
	sess.Agent = spec.Name
	sess.Model = spec.Model
	return nil
}

// --- MCP ---

func (a *App) GetMCPStatus() []MCPStatus {
	// Scan examples/mcp for definitions.
	var statuses []MCPStatus
	fs.WalkDir(examples.Agents, ".", func(path string, d fs.DirEntry, err error) error {
		return fs.SkipAll // agents FS doesn't have MCP
	})
	// For now return empty — MCP definitions are on disk not embedded.
	return statuses
}

// readOnlyTools are auto-approved in the desktop: they only read state
// (filesystem reads, searches, knowledge lookups, web GETs) so prompting for
// each one is pure friction. Mutating tools (fs_write, shell, git, km_index),
// custom shell tools, and MCP tools are NOT listed here and always prompt.
var readOnlyTools = map[string]bool{
	"fs_read":         true,
	"fs_list":         true,
	"code_search":     true,
	"knowledge":       true,
	"web_fetch":       true,
	"km_search":       true,
	"km_status":       true,
	"km_blast_radius": true,
}

// --- Tool Confirmation ---

// requestToolApproval emits a tool-confirmation request to the frontend and
// blocks until the user answers (via RespondToolApproval) or ctx is
// cancelled (e.g. the user hit Stop). Returns the user's decision; a
// cancelled context surfaces as an error the engine treats as "cancelled".
func (a *App) requestToolApproval(ctx context.Context, sessionID, name string, input json.RawMessage) (bool, error) {
	reqID := fmt.Sprintf("req_%d", time.Now().UnixNano())
	ch := make(chan bool, 1)

	a.pendingMu.Lock()
	a.pending[reqID] = ch
	a.pendingMu.Unlock()

	defer func() {
		a.pendingMu.Lock()
		delete(a.pending, reqID)
		a.pendingMu.Unlock()
	}()

	runtime.EventsEmit(a.ctx, "toolConfirm", map[string]any{
		"sessionId": sessionID,
		"requestId": reqID,
		"tool":      name,
		"input":     string(input),
	})

	select {
	case ok := <-ch:
		return ok, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// requestContinue is invoked when a Step exhausts its tool-turn budget. It
// asks the frontend whether to keep going (another full batch of turns) or
// stop, reusing the same pending-channel plumbing as tool approvals so the
// frontend answers both via RespondToolApproval.
func (a *App) requestContinue(ctx context.Context, sessionID string, turns int) (bool, error) {
	reqID := fmt.Sprintf("cont_%d", time.Now().UnixNano())
	ch := make(chan bool, 1)

	a.pendingMu.Lock()
	a.pending[reqID] = ch
	a.pendingMu.Unlock()

	defer func() {
		a.pendingMu.Lock()
		delete(a.pending, reqID)
		a.pendingMu.Unlock()
	}()

	runtime.EventsEmit(a.ctx, "continueConfirm", map[string]any{
		"sessionId": sessionID,
		"requestId": reqID,
		"turns":     turns,
	})

	select {
	case ok := <-ch:
		return ok, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// RespondToolApproval is called from the frontend to answer a pending
// tool-approval request. Unknown or already-answered ids are ignored.
func (a *App) RespondToolApproval(requestID string, approved bool) {
	a.pendingMu.Lock()
	ch, ok := a.pending[requestID]
	if ok {
		delete(a.pending, requestID)
	}
	a.pendingMu.Unlock()
	if ok {
		ch <- approved
	}
}

// --- Helpers ---

func (a *App) resolveAgent(name string) (*config.AgentSpec, string, error) {
	// Try bundled first.
	path := "agents/" + name + ".md"
	if data, err := examples.Agents.ReadFile(path); err == nil {
		doc, err := config.Parse(data)
		if err != nil {
			return nil, "", err
		}
		spec, ok := doc.Spec.(*config.AgentSpec)
		if !ok {
			return nil, "", fmt.Errorf("not an agent: %s", name)
		}
		return spec, doc.Body, nil
	}

	// Try user agents dir.
	if dir := userAgentsDir(); dir != "" {
		p := filepath.Join(dir, name+".md")
		if doc, err := config.ParseFile(p); err == nil {
			spec, ok := doc.Spec.(*config.AgentSpec)
			if !ok {
				return nil, "", fmt.Errorf("not an agent: %s", name)
			}
			return spec, doc.Body, nil
		}
	}

	return nil, "", fmt.Errorf("agent %q not found", name)
}

func userAgentsDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m", "agents")
}

// userToolsDir is where user-authored tool MD files live. Each file (type:
// tool, runtime: shell) becomes a callable tool available to every session.
func userToolsDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m", "tools")
}

// streamWriter emits Wails events for each Write call.
type streamWriter struct {
	ctx       context.Context
	sessionID string
}

func (w *streamWriter) Write(p []byte) (int, error) {
	runtime.EventsEmit(w.ctx, "stream", map[string]any{
		"sessionId": w.sessionID,
		"text":      string(p),
	})
	return len(p), nil
}

// routeRe matches the engine's MoE routing status line, e.g.
// "[routed → reasoning (gemini/gemini-2.5-flash)]".
var routeRe = regexp.MustCompile(`\[routed\s*→\s*(\S+)\s*\(([^)]+)\)\]`)

// statusWriter receives engine diagnostics (separate from assistant output).
// It emits a generic "status" event and, when it detects a MoE routing line,
// a structured "route" event the UI can render as a badge.
type statusWriter struct {
	ctx       context.Context
	sessionID string
}

func (w *statusWriter) Write(p []byte) (int, error) {
	text := string(p)
	if m := routeRe.FindStringSubmatch(text); m != nil {
		runtime.EventsEmit(w.ctx, "route", map[string]any{
			"sessionId": w.sessionID,
			"category":  m[1],
			"model":     m[2],
		})
		emitActivity(w.ctx, w.sessionID, "route", m[1], map[string]any{"model": m[2]})
	}
	trimmed := strings.TrimSpace(text)
	if trimmed != "" {
		emitActivity(w.ctx, w.sessionID, "status", trimmed, nil)
	}
	runtime.EventsEmit(w.ctx, "status", map[string]any{
		"sessionId": w.sessionID,
		"text":      text,
	})
	return len(p), nil
}

func emitActivity(ctx context.Context, sessionID, kind, label string, detail map[string]any) {
	runtime.EventsEmit(ctx, "activity", map[string]any{
		"sessionId": sessionID,
		"kind":      kind,
		"label":     label,
		"detail":    detail,
		"ts":        time.Now().UnixMilli(),
	})
}

// ThemeInfo describes a desktop theme.
type ThemeInfo struct {
	Name      string `json:"name"`
	BG        string `json:"bg"`
	BGPanel   string `json:"bgPanel"`
	BGInput   string `json:"bgInput"`
	Border    string `json:"border"`
	User      string `json:"user"`
	Assistant string `json:"assistant"`
	Tool      string `json:"tool"`
	Error     string `json:"error"`
	Accent    string `json:"accent"`
	Text      string `json:"text"`
	Muted     string `json:"muted"`
}

var desktopThemes = []ThemeInfo{
	{Name: "default", BG: "#0f172a", BGPanel: "#1e293b", BGInput: "#1e293b", Border: "#334155", User: "#5f87ff", Assistant: "#d0d0d0", Tool: "#d7af00", Error: "#ff5f5f", Accent: "#3b82f6", Text: "#e2e8f0", Muted: "#64748b"},
	{Name: "matrix", BG: "#0d0d0d", BGPanel: "#111111", BGInput: "#0a0a0a", Border: "#003300", User: "#00ff00", Assistant: "#00cc00", Tool: "#008800", Error: "#ff3333", Accent: "#00ff00", Text: "#00ee00", Muted: "#005500"},
	{Name: "nord", BG: "#2e3440", BGPanel: "#3b4252", BGInput: "#3b4252", Border: "#4c566a", User: "#88c0d0", Assistant: "#d8dee9", Tool: "#81a1c1", Error: "#bf616a", Accent: "#88c0d0", Text: "#eceff4", Muted: "#4c566a"},
	{Name: "dracula", BG: "#282a36", BGPanel: "#44475a", BGInput: "#44475a", Border: "#6272a4", User: "#8be9fd", Assistant: "#f8f8f2", Tool: "#ffb86c", Error: "#ff5555", Accent: "#bd93f9", Text: "#f8f8f2", Muted: "#6272a4"},
	{Name: "gruvbox", BG: "#282828", BGPanel: "#3c3836", BGInput: "#3c3836", Border: "#504945", User: "#83a598", Assistant: "#ebdbb2", Tool: "#fe8019", Error: "#fb4934", Accent: "#b8bb26", Text: "#ebdbb2", Muted: "#928374"},
	{Name: "tokyonight", BG: "#1a1b26", BGPanel: "#24283b", BGInput: "#24283b", Border: "#414868", User: "#7dcfff", Assistant: "#c0caf5", Tool: "#e0af68", Error: "#f7768e", Accent: "#7aa2f7", Text: "#c0caf5", Muted: "#565f89"},
	{Name: "catppuccin", BG: "#1e1e2e", BGPanel: "#313244", BGInput: "#313244", Border: "#45475a", User: "#89dceb", Assistant: "#cdd6f4", Tool: "#fab387", Error: "#f38ba8", Accent: "#cba6f7", Text: "#cdd6f4", Muted: "#6c7086"},
	{Name: "solarized", BG: "#002b36", BGPanel: "#073642", BGInput: "#073642", Border: "#586e75", User: "#268bd2", Assistant: "#839496", Tool: "#b58900", Error: "#dc322f", Accent: "#2aa198", Text: "#839496", Muted: "#586e75"},
}

func (a *App) GetThemes() []ThemeInfo {
	return desktopThemes
}
