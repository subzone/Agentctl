package main

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/subzone/Agentctl/internal/engine"
	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/tools"
)

// newTestSession creates a minimal session for unit testing slash commands.
func newTestSession(t *testing.T) *engine.Session {
	t.Helper()
	return engine.NewSession(engine.Config{
		Provider: &noopProvider{},
		Model:    "test/model",
		Out:      io.Discard,
		Status:   io.Discard,
	})
}

type noopProvider struct{}

func (n *noopProvider) Stream(_ context.Context, _ llm.Request) (<-chan llm.Event, error) {
	ch := make(chan llm.Event, 1)
	ch <- llm.Event{Kind: llm.EventDone, StopReason: "end_turn"}
	close(ch)
	return ch, nil
}

func TestIsDangerousCommand(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		input string
		want  string
	}{
		{"rm -rf", "shell", `{"command":"rm -rf /tmp/old"}`, "rm -rf"},
		{"kubectl delete", "shell", `{"command":"kubectl delete pod nginx"}`, "kubectl delete"},
		{"terraform destroy", "shell", `{"command":"terraform destroy -auto-approve"}`, "terraform destroy"},
		{"git push force", "shell", `{"command":"git push --force origin main"}`, "git push --force"},
		{"git reset hard", "shell", `{"command":"git reset --hard HEAD~1"}`, "git reset --hard"},
		{"drop database", "shell", `{"command":"psql -c 'drop database prod'"}`, "drop database"},
		{"safe command", "shell", `{"command":"ls -la"}`, ""},
		{"safe git", "git", `{"command":"status"}`, ""},
		{"fs_read safe", "fs_read", `{"path":"/etc/passwd"}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDangerousCommand(tt.tool, json.RawMessage(tt.input))
			if tt.want == "" && got != "" {
				t.Errorf("expected safe, got dangerous: %q", got)
			}
			if tt.want != "" && got == "" {
				t.Errorf("expected dangerous %q, got safe", tt.want)
			}
		})
	}
}

func TestHandleSlashCommands(t *testing.T) {
	tests := []struct {
		line    string
		handled bool
		exit    bool
	}{
		{"/exit", true, true},
		{"/quit", true, true},
		{"/x", true, true},
		{"/reset", true, false},
		{"/r", true, false},
		{"/compact", true, false},
		{"/c", true, false},
		{"/help", true, false},
		{"/h", true, false},
		{"/trust", true, false},
		{"/trust off", true, false},
		{"/pii", true, false},
		{"/pii off", true, false},
		{"/debug", true, false},
		{"/debug off", true, false},
		{"/config", true, false},
		{"/unknown", true, false},
		{"hello", false, false},
		{"", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			state := &chatState{
				sess: newTestSession(t),
				undo: &tools.UndoStack{},
			}
			var buf strings.Builder
			handled, exit := handleSlash(tt.line, state, &buf)
			if handled != tt.handled {
				t.Errorf("handleSlash(%q) handled=%v, want %v", tt.line, handled, tt.handled)
			}
			if exit != tt.exit {
				t.Errorf("handleSlash(%q) exit=%v, want %v", tt.line, exit, tt.exit)
			}
		})
	}
}

func TestHandleSlashThemes(t *testing.T) {
	state := &chatState{sess: newTestSession(t)}
	var buf strings.Builder

	// /themes should list themes
	handleSlash("/themes", state, &buf)
	if !strings.Contains(buf.String(), "dracula") {
		t.Error("/themes should list dracula")
	}

	// /t should also list themes
	buf.Reset()
	handleSlash("/t", state, &buf)
	if !strings.Contains(buf.String(), "matrix") {
		t.Error("/t should list matrix")
	}
}

func TestHandleSlashSave(t *testing.T) {
	state := &chatState{sess: newTestSession(t)}
	var buf strings.Builder

	// /save should work (may fail on keychain but shouldn't panic)
	handleSlash("/save", state, &buf)
	// /s should also work
	buf.Reset()
	handleSlash("/s", state, &buf)
	// /save with name
	buf.Reset()
	handleSlash("/save test-session", state, &buf)
	// /s with name
	buf.Reset()
	handleSlash("/s my-session", state, &buf)
}

func TestHandleSlashModels(t *testing.T) {
	state := &chatState{sess: newTestSession(t)}
	var buf strings.Builder

	// /models without config — should show error or scanning message
	handleSlash("/models", state, &buf)
	// /m should also work
	buf.Reset()
	handleSlash("/m", state, &buf)
}

func TestSummarizeToolInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"path field", `{"path":"/tmp/test.go"}`, "path=/tmp/test.go"},
		{"command field", `{"command":"ls -la"}`, "command=ls -la"},
		{"url field", `{"url":"https://example.com"}`, "url=https://example.com"},
		{"long command", `{"command":"` + strings.Repeat("x", 100) + `"}`, "command=" + strings.Repeat("x", 60) + "..."},
		{"invalid json", `not json`, "not json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeToolInput(json.RawMessage(tt.input))
			if !strings.Contains(got, tt.want) {
				t.Errorf("summarizeToolInput(%s) = %q, want to contain %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFixMarkdownSpacing(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(string) bool
	}{
		{
			"bold renders",
			"this is **bold** text",
			func(s string) bool { return strings.Contains(s, "\033[1m") && strings.Contains(s, "bold") },
		},
		{
			"inline code renders",
			"use `fmt.Println` here",
			func(s string) bool { return strings.Contains(s, "\033[2m") && strings.Contains(s, "fmt.Println") },
		},
		{
			"numbered list gets newline",
			"first item2. second item",
			func(s string) bool { return strings.Contains(s, "\n2.") },
		},
		{
			"header renders bold",
			"## My Header",
			func(s string) bool { return strings.Contains(s, "\033[1m") && strings.Contains(s, "My Header") },
		},
		{
			"plain text unchanged",
			"hello world",
			func(s string) bool { return s == "hello world" },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixMarkdownSpacing(tt.input)
			if !tt.check(got) {
				t.Errorf("fixMarkdownSpacing(%q) = %q", tt.input, got)
			}
		})
	}
}

func TestVisibleLen(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{"\033[1mhello\033[0m", 5},
		{"\033[38;2;255;0;0mred\033[0m text", 8},
		{"", 0},
		{"no escapes here", 15},
	}
	for _, tt := range tests {
		got := visibleLen(tt.input)
		if got != tt.want {
			t.Errorf("visibleLen(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"0.0.30", "0.0.29", 1},
		{"0.0.29", "0.0.30", -1},
		{"0.0.30", "0.0.30", 0},
		{"0.1.0", "0.0.99", 1},
		{"1.0.0", "0.9.9", 1},
	}
	for _, tt := range tests {
		got := compareVersions(tt.a, tt.b)
		if (tt.want > 0 && got <= 0) || (tt.want < 0 && got >= 0) || (tt.want == 0 && got != 0) {
			t.Errorf("compareVersions(%q, %q) = %d, want sign %d", tt.a, tt.b, got, tt.want)
		}
	}
}
