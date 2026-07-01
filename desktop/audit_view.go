package desktop

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/subzone/Agentctl/internal/audit"
	"github.com/subzone/Agentctl/internal/policy"
	"github.com/subzone/Agentctl/internal/userconfig"
)

// PolicyRuleView is one policy rule for the desktop Security panel.
type PolicyRuleView struct {
	Name            string   `json:"name"`
	Tool            string   `json:"tool,omitempty"`
	DenyPattern     string   `json:"denyPattern,omitempty"`
	AllowPathPrefix []string `json:"allowPathPrefix,omitempty"`
	Message         string   `json:"message,omitempty"`
}

// AuditEventView is a trimmed audit row for the UI.
type AuditEventView struct {
	Type       string `json:"type"`
	SessionID  string `json:"sessionId,omitempty"`
	TS         string `json:"ts,omitempty"`
	Tool       string `json:"tool,omitempty"`
	Outcome    string `json:"outcome,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"durationMs,omitempty"`
	Agent      string `json:"agent,omitempty"`
	Model      string `json:"model,omitempty"`
}

// AuditConfigView describes where audit logs are written.
type AuditConfigView struct {
	Backend string `json:"backend"`
	Path    string `json:"path"`
	Active  bool   `json:"active"`
}

// GetPolicyRules returns merged policy rules from home and project config.
func (a *App) GetPolicyRules() ([]PolicyRuleView, error) {
	homeCfg, err := userconfig.Path()
	if err != nil {
		return nil, err
	}
	cwd, _ := os.Getwd()
	projectCfg := policy.FindProjectConfigPath(cwd)
	rules, err := policy.LoadRules(homeCfg, projectCfg)
	if err != nil {
		return nil, err
	}
	out := make([]PolicyRuleView, 0, len(rules))
	for _, r := range rules {
		out = append(out, PolicyRuleView{
			Name:            r.Name,
			Tool:            r.Tool,
			DenyPattern:     r.DenyPattern,
			AllowPathPrefix: r.AllowPathPrefix,
			Message:         r.Message,
		})
	}
	return out, nil
}

// GetAuditConfig returns the effective audit backend configuration.
func (a *App) GetAuditConfig() (AuditConfigView, error) {
	homeCfg, err := userconfig.Path()
	if err != nil {
		return AuditConfigView{}, err
	}
	cwd, _ := os.Getwd()
	projectCfg := audit.FindProjectConfigPath(cwd)
	cfg, err := audit.LoadConfig(homeCfg, projectCfg)
	if err != nil {
		return AuditConfigView{}, err
	}
	path := cfg.Path
	if path == "" && (cfg.Backend == "" || cfg.Backend == "file") {
		base, err := os.UserConfigDir()
		if err == nil {
			path = filepath.Join(base, "m", "audit.jsonl")
		}
	}
	backend := cfg.Backend
	if backend == "" {
		backend = "none"
	}
	return AuditConfigView{
		Backend: backend,
		Path:    path,
		Active:  backend != "" && backend != "none",
	}, nil
}

// ListAuditEvents reads the tail of the audit JSONL file.
func (a *App) ListAuditEvents(limit int) ([]AuditEventView, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	cfg, err := a.GetAuditConfig()
	if err != nil || !cfg.Active || cfg.Path == "" {
		return nil, nil
	}
	f, err := os.Open(cfg.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}

	out := make([]AuditEventView, 0, len(lines))
	for _, line := range lines {
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		ev := AuditEventView{
			Type:      strField(raw, "type"),
			SessionID: strField(raw, "session_id"),
			Tool:      strField(raw, "tool"),
			Outcome:   strField(raw, "outcome"),
			Error:     strField(raw, "error"),
			Agent:     strField(raw, "agent"),
			Model:     strField(raw, "model"),
		}
		if ts, ok := raw["ts"].(string); ok {
			ev.TS = ts
		}
		if d, ok := raw["duration_ms"].(float64); ok {
			ev.DurationMS = int64(d)
		}
		out = append(out, ev)
	}
	return out, nil
}

func strField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
