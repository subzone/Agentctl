
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
	sessions map[string]*Session
	mu       sync.RWMutex
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
	return &App{sessions: make(map[string]*Session)}
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
			switch cfg.Provider {
			case userconfig.ProviderAnthropic:
				os.Setenv("ANTHROPIC_API_KEY", key)
			case userconfig.ProviderOpenAI:
				os.Setenv("OPENAI_API_KEY", key)
			case userconfig.ProviderGemini:
				os.Setenv("GEMINI_API_KEY", key)
			case userconfig.ProviderAlibaba:
				os.Setenv("DASHSCOPE_API_KEY", key)
			case userconfig.ProviderLiteLLM:
				os.Setenv("LITELLM_API_KEY", key)
			}
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
	return userconfig.SaveAPIKey(userconfig.Provider(provider), key)
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

	// Build tools.
	reg := tools.Builtins(
		func(_ context.Context, prompt string) (bool, error) {
			return a.requestToolConfirm(prompt)
		},
		&tools.UndoStack{},
	)

	maxTokens := 0
	if spec.MaxTokens != nil {
		maxTokens = *spec.MaxTokens
	}

	// Create streaming writer that emits events to frontend.
	id := fmt.Sprintf("s_%d", time.Now().UnixMilli())
	sw := &streamWriter{ctx: a.ctx, sessionID: id}

	sess := engine.NewSession(engine.Config{
		Provider:    provider,
		Model:       resolvedModel,
		System:      body,
		Tools:       reg,
		Temperature: spec.Temperature,
		MaxTokens:   maxTokens,
		ToolConfirm: func(ctx context.Context, name string, input json.RawMessage) (bool, error) {
			return a.requestToolApproval(id, name, input)
		},
		Out:    sw,
		Status: sw,
	})

	session := &Session{
		ID:        id,
		Agent:     spec.Name,
		Model:     model,
		CreatedAt: time.Now().Unix(),
		engine:    sess,
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
	a.mu.RLock()
	sess, ok := a.sessions[sessionID]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	// Add user message.
	sess.Messages = append(sess.Messages, Message{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixMilli()),
		Role:      "user",
		Content:   message,
		Timestamp: time.Now().Unix(),
	})

	// Emit user message event.
	runtime.EventsEmit(a.ctx, "message", map[string]any{
		"sessionId": sessionID,
		"role":      "user",
		"content":   message,
	})

	// Run engine step (streams via events).
	ctx, cancel := context.WithCancel(a.ctx)
	sess.cancel = cancel
	go func() {
		defer cancel()
		if err := sess.engine.Step(ctx, message); err != nil {
			runtime.EventsEmit(a.ctx, "error", map[string]any{
				"sessionId": sessionID,
				"error":     err.Error(),
			})
		}
		// Emit done event with usage and cost.
		usage := sess.engine.Usage()
		cost := calcCost(sess.Model, usage.InputTokens, usage.OutputTokens)
		runtime.EventsEmit(a.ctx, "done", map[string]any{
			"sessionId":    sessionID,
			"inputTokens":  usage.InputTokens,
			"outputTokens": usage.OutputTokens,
			"cost":         cost,
		})
	}()

	return nil
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
	defer a.mu.Unlock()
	if s, ok := a.sessions[sessionID]; ok && s.cancel != nil {
		s.cancel()
	}
	delete(a.sessions, sessionID)
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

	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.engine.SetModel(provider, model)
	sess.engine.SetSystem(body)
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

// --- Tool Confirmation ---

func (a *App) requestToolConfirm(prompt string) (bool, error) {
	// Emit event and wait for response from frontend.
	runtime.EventsEmit(a.ctx, "confirm", map[string]any{"prompt": prompt})
	// TODO: implement response channel from frontend
	return true, nil
}

func (a *App) requestToolApproval(sessionID, name string, input json.RawMessage) (bool, error) {
	runtime.EventsEmit(a.ctx, "toolConfirm", map[string]any{
		"sessionId": sessionID,
		"tool":      name,
		"input":     string(input),
	})
	// TODO: implement response channel from frontend
	return true, nil
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
