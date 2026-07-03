package desktop

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/subzone/Agentctl/internal/adapters"
	"github.com/subzone/Agentctl/internal/engine"
	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/ports"
	"github.com/subzone/Agentctl/internal/userconfig"
)

const sessionKeyService = "session-encryption"

type keychainKeyProvider struct{}

func (k *keychainKeyProvider) GetKey(_ context.Context) ([]byte, error) {
	raw, err := userconfig.GetAPIKey(userconfig.Provider(sessionKeyService))
	if err == nil && raw != "" {
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate session key: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	if err := userconfig.SaveAPIKey(userconfig.Provider(sessionKeyService), encoded); err != nil {
		return nil, fmt.Errorf("save session key to keychain: %w", err)
	}
	return key, nil
}

func sessionStore() (*adapters.EncryptedFileStore, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(base, "m", "sessions")
	return adapters.NewEncryptedFileStore(dir, &keychainKeyProvider{})
}

// SavedSessionSummary is a persisted chat session on disk.
type SavedSessionSummary struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Messages     int    `json:"messages"`
	Preview      string `json:"preview"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	SavedAt      int64  `json:"savedAt"`
}

// ResumeResult is returned when a saved session is loaded into a new chat.
type ResumeResult struct {
	Session  SessionInfo `json:"session"`
	Messages []Message `json:"messages"`
}

func skipSavedSessionID(id string) bool {
	return id == "_autosave" || strings.HasPrefix(id, "_auto_")
}

// ListSavedSessions reads encrypted session files from ~/.config/m/sessions.
func (a *App) ListSavedSessions() ([]SavedSessionSummary, error) {
	store, err := sessionStore()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	ids, err := store.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(ids, func(i, j int) bool {
		return parseSessionSavedAt(ids[i]) > parseSessionSavedAt(ids[j])
	})

	var out []SavedSessionSummary
	for _, id := range ids {
		if skipSavedSessionID(id) {
			continue
		}
		data, err := store.LoadSession(ctx, id)
		if err != nil || data == nil || len(data.Messages) == 0 {
			continue
		}
		out = append(out, SavedSessionSummary{
			ID:           id,
			Label:        getSessionLabel(id),
			Provider:     data.Provider,
			Model:        data.Model,
			Messages:     len(data.Messages),
			Preview:      sessionPreview(data),
			InputTokens:  data.InputTokens,
			OutputTokens: data.OutputTokens,
			SavedAt:      parseSessionSavedAt(id),
		})
		if len(out) >= 30 {
			break
		}
	}
	return out, nil
}

// ResumeSavedSession loads a disk session into a new in-memory chat.
func (a *App) ResumeSavedSession(savedID, agentName string) (*ResumeResult, error) {
	if savedID == "" {
		return nil, fmt.Errorf("session id required")
	}
	store, err := sessionStore()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	data, err := store.LoadSession(ctx, savedID)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	if agentName == "" {
		agentName = "m"
		if a.cfg != nil && a.cfg.DefaultAgent != "" {
			agentName = a.cfg.DefaultAgent
		}
	}

	info, err := a.CreateSession(agentName)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	sess, ok := a.sessions[info.ID]
	a.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("session not found after create")
	}

	engineMsgs := importSessionData(data)
	displayMsgs := sessionDataToMessages(data)

	sess.mu.Lock()
	sess.engine.RestoreMessages(engineMsgs)
	sess.engine.SetUsage(llm.Usage{
		InputTokens:  data.InputTokens,
		OutputTokens: data.OutputTokens,
	})
	sess.Messages = displayMsgs
	sess.mu.Unlock()

	return &ResumeResult{Session: *info, Messages: displayMsgs}, nil
}

// DeleteSavedSession removes a persisted session file.
func (a *App) DeleteSavedSession(savedID string) error {
	if savedID == "" {
		return fmt.Errorf("session id required")
	}
	store, err := sessionStore()
	if err != nil {
		return err
	}
	return store.DeleteSession(context.Background(), savedID)
}

func (a *App) persistSessionToDisk(s *Session) {
	if s == nil {
		return
	}
	s.mu.RLock()
	if len(s.Messages) == 0 || s.engine == nil {
		s.mu.RUnlock()
		return
	}
	usage := s.engine.Usage()
	model := s.Model
	provider := model
	if i := strings.Index(model, "/"); i > 0 {
		provider = model[:i]
	}
	data := exportEngineSession(s.engine, provider, model, usage.InputTokens, usage.OutputTokens)
	s.mu.RUnlock()

	store, err := sessionStore()
	if err != nil {
		return
	}
	_ = store.SaveSession(context.Background(), s.ID, data)
}

func exportEngineSession(sess *engine.Session, provider, model string, inTok, outTok int) *ports.SessionData {
	msgs := sess.Messages()
	out := &ports.SessionData{
		Provider:     provider,
		Model:        model,
		InputTokens:  inTok,
		OutputTokens: outTok,
		Messages:     make([]ports.Message, len(msgs)),
	}
	for i, m := range msgs {
		pm := ports.Message{Role: string(m.Role)}
		for _, b := range m.Content {
			pb := ports.ContentBlock{Type: string(b.Type), Text: b.Text}
			pb.MimeType = b.MimeType
			if b.Type == llm.BlockImage && len(b.ImageData) > 0 {
				pb.ImageB64 = base64.StdEncoding.EncodeToString(b.ImageData)
			}
			pb.ToolID = b.ToolID
			pb.ToolName = b.ToolName
			if len(b.ToolInput) > 0 {
				pb.ToolInput = string(b.ToolInput)
			}
			pb.ToolUseID = b.ToolUseID
			pb.Output = b.Output
			pb.IsError = b.IsError
			pm.Content = append(pm.Content, pb)
		}
		out.Messages[i] = pm
	}
	return out
}

func importSessionData(data *ports.SessionData) []llm.Message {
	msgs := make([]llm.Message, len(data.Messages))
	for i, pm := range data.Messages {
		m := llm.Message{Role: llm.Role(pm.Role)}
		for _, pb := range pm.Content {
			b := llm.ContentBlock{
				Type:      llm.BlockType(pb.Type),
				Text:      pb.Text,
				MimeType:  pb.MimeType,
				ToolID:    pb.ToolID,
				ToolName:  pb.ToolName,
				ToolUseID: pb.ToolUseID,
				Output:    pb.Output,
				IsError:   pb.IsError,
			}
			if pb.ImageB64 != "" {
				if decoded, err := base64.StdEncoding.DecodeString(pb.ImageB64); err == nil {
					b.ImageData = decoded
				}
			}
			if pb.ToolInput != "" {
				b.ToolInput = json.RawMessage(pb.ToolInput)
			}
			m.Content = append(m.Content, b)
		}
		msgs[i] = m
	}
	return msgs
}

func sessionDataToMessages(data *ports.SessionData) []Message {
	var out []Message
	ts := time.Now().Unix()
	for i, pm := range data.Messages {
		role := pm.Role
		if role == "" {
			continue
		}
		text := flattenContent(pm.Content)
		if text == "" && role != "assistant" {
			continue
		}
		out = append(out, Message{
			ID:        fmt.Sprintf("msg_%d_%d", ts, i),
			Role:      role,
			Content:   text,
			Timestamp: ts,
		})
	}
	return out
}

func flattenContent(blocks []ports.ContentBlock) string {
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		case "image":
			if b.MimeType != "" {
				parts = append(parts, "[image: "+b.MimeType+"]")
			} else {
				parts = append(parts, "[image]")
			}
		case "tool_use":
			if b.ToolName != "" {
				parts = append(parts, "[tool: "+b.ToolName+"]")
			}
		case "tool_result":
			if b.Output != "" {
				parts = append(parts, b.Output)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func sessionPreview(data *ports.SessionData) string {
	for _, pm := range data.Messages {
		if pm.Role != "user" {
			continue
		}
		text := flattenContent(pm.Content)
		if text == "" {
			continue
		}
		if len(text) > 60 {
			return text[:57] + "..."
		}
		return text
	}
	return "Saved session"
}

// SetSessionLabel assigns a human-readable name to a saved session.
func (a *App) SetSessionLabel(savedID, label string) error {
	if savedID == "" {
		return fmt.Errorf("session id required")
	}
	return saveSessionLabel(savedID, strings.TrimSpace(label))
}

// SearchSavedSessions filters saved sessions by label or preview text.
func (a *App) SearchSavedSessions(query string) ([]SavedSessionSummary, error) {
	all, err := a.ListSavedSessions()
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return all, nil
	}
	var out []SavedSessionSummary
	for _, s := range all {
		if strings.Contains(strings.ToLower(s.Label), q) ||
			strings.Contains(strings.ToLower(s.Preview), q) ||
			strings.Contains(strings.ToLower(s.ID), q) {
			out = append(out, s)
		}
	}
	return out, nil
}

func sessionLabelsPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m", "session_labels.json")
}

func loadSessionLabels() map[string]string {
	p := sessionLabelsPath()
	if p == "" {
		return map[string]string{}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return map[string]string{}
	}
	var labels map[string]string
	if err := json.Unmarshal(data, &labels); err != nil {
		return map[string]string{}
	}
	return labels
}

func getSessionLabel(id string) string {
	return loadSessionLabels()[id]
}

func saveSessionLabel(id, label string) error {
	labels := loadSessionLabels()
	if label == "" {
		delete(labels, id)
	} else {
		labels[id] = label
	}
	p := sessionLabelsPath()
	if p == "" {
		return fmt.Errorf("config dir unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(labels, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func parseSessionSavedAt(id string) int64 {
	if strings.HasPrefix(id, "s_") {
		var ms int64
		if _, err := fmt.Sscanf(id, "s_%d", &ms); err == nil && ms > 0 {
			return ms / 1000
		}
	}
	if len(id) >= 16 {
		if t, err := time.Parse("2006-01-02_15-04", id[:16]); err == nil {
			return t.Unix()
		}
	}
	return 0
}

// ExportSessionMarkdown renders the current session as a Markdown transcript.
func (a *App) ExportSessionMarkdown(sessionID string) (string, error) {
	a.mu.RLock()
	sess, ok := a.sessions[sessionID]
	a.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}
	sess.mu.RLock()
	defer sess.mu.RUnlock()

	var b strings.Builder
	b.WriteString("# AgentCTL chat export\n\n")
	if sess.Agent != "" {
		b.WriteString("**Agent:** ")
		b.WriteString(sess.Agent)
		b.WriteString("\n\n")
	}
	if sess.Model != "" {
		b.WriteString("**Model:** ")
		b.WriteString(sess.Model)
		b.WriteString("\n\n")
	}
	b.WriteString("---\n\n")

	if sess.engine != nil {
		for _, m := range sess.engine.Messages() {
			role := string(m.Role)
			label := role
			switch role {
			case "user":
				label = "You"
			case "assistant":
				label = sess.Agent
				if label == "" {
					label = "Agent"
				}
			case "tool":
				label = "Tool"
			}
			text := flattenLLMContent(m.Content)
			if text == "" {
				continue
			}
			b.WriteString("## ")
			b.WriteString(label)
			b.WriteString("\n\n")
			b.WriteString(text)
			b.WriteString("\n\n")
		}
		return b.String(), nil
	}
	for _, m := range sess.Messages {
		if m.Content == "" {
			continue
		}
		b.WriteString("## ")
		b.WriteString(m.Role)
		b.WriteString("\n\n")
		b.WriteString(m.Content)
		b.WriteString("\n\n")
	}
	return b.String(), nil
}

func flattenLLMContent(blocks []llm.ContentBlock) string {
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case llm.BlockText:
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		case llm.BlockToolUse:
			if b.ToolName != "" {
				parts = append(parts, "[tool: "+b.ToolName+"]")
			}
		case llm.BlockToolResult:
			if b.Output != "" {
				parts = append(parts, b.Output)
			}
		}
	}
	return strings.Join(parts, "\n")
}
