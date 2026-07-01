package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/subzone/Agentctl/internal/audit"
	"github.com/subzone/Agentctl/internal/policy"
	"github.com/subzone/Agentctl/internal/ports"
	"github.com/subzone/Agentctl/internal/userconfig"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// wailsAuditSink forwards audit events to the frontend and an optional base sink.
type wailsAuditSink struct {
	ctx       context.Context
	sessionID string
	base      ports.AuditSink
}

func (w *wailsAuditSink) Emit(ctx context.Context, ev ports.AuditEvent) error {
	switch ev.Type {
	case "tool_call":
		runtime.EventsEmit(w.ctx, "toolRun", map[string]any{
			"sessionId": w.sessionID,
			"phase":     "start",
			"tool":      ev.Tool,
			"input":     string(ev.Args),
			"toolId":    ev.EventID,
		})
	case "tool_result":
		runtime.EventsEmit(w.ctx, "toolRun", map[string]any{
			"sessionId":  w.sessionID,
			"phase":      "end",
			"tool":       ev.Tool,
			"outcome":    ev.Outcome,
			"durationMs": ev.DurationMS,
			"error":      ev.Error,
			"toolId":     ev.EventID,
		})
	}
	if w.base != nil {
		return w.base.Emit(ctx, ev)
	}
	return nil
}

func (w *wailsAuditSink) Flush(ctx context.Context) error {
	if w.base != nil {
		return w.base.Flush(ctx)
	}
	return nil
}

func (w *wailsAuditSink) Close() error {
	if w.base != nil {
		return w.base.Close()
	}
	return nil
}

func buildDesktopAuditSink(appCtx context.Context, sessionID string) (ports.AuditSink, error) {
	base, err := buildAuditSink()
	if err != nil {
		return nil, err
	}
	return &wailsAuditSink{ctx: appCtx, sessionID: sessionID, base: base}, nil
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

func buildDesktopPolicyCheck() (func(context.Context, string, json.RawMessage) (string, error), error) {
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
