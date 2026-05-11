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
		// Emit done event with usage.
		usage := sess.engine.Usage()
		runtime.EventsEmit(a.ctx, "done", map[string]any{
			"sessionId":    sessionID,
			"inputTokens":  usage.InputTokens,
			"outputTokens": usage.OutputTokens,
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
	return &CostInfo{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		Model:        sess.Model,
	}
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
