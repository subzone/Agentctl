package main

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"

	"github.com/subzone/Agentctl/internal/llm"
)

// TestAppendHistorySurvivesValueCopy locks in the fix for the v0.0.3
// crash: bubbletea's Update has a value receiver, so the model is
// copied on every call. A bare strings.Builder field panics on the
// next WriteString after the copy. Storing a *Builder makes the copies
// share one backing buffer.
func TestAppendHistorySurvivesValueCopy(t *testing.T) {
	m := tuiModel{
		history:        &strings.Builder{},
		viewport:       viewport.New(80, 10),
		lastUpdateTime: time.Now().Add(-100 * time.Millisecond),
	}
	m.appendHistory("hello ")
	m.appendHistory("world\n")
	m2 := m // simulate bubbletea's value-copy on Update
	if !strings.Contains(m2.history.String(), "hello world") {
		t.Errorf("got %q, want to contain %q", m2.history.String(), "hello world")
	}
}

func TestWordWrapEmpty(t *testing.T) {
	// Test wordWrap with empty string
	result := wordWrap("", 80)
	if result != "" {
		t.Errorf("wordWrap(\"\", 80) = %q, want \"\"", result)
	}
}

func TestWordWrapShort(t *testing.T) {
	result := wordWrap("short line", 80)
	if result != "short line" {
		t.Errorf("wordWrap(\"short line\", 80) = %q, want \"short line\"", result)
	}
}



func TestFormatTokens(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{12345, "12.3k"},
		{1000000, "1.0M"},
		{2500000, "2.5M"},
	}
	for _, tt := range tests {
		if got := formatTokens(tt.n); got != tt.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		v    float64
		want string
	}{
		{0.0001, "$0.0001"},
		{0.005, "$0.0050"},
		{0.01, "$0.01"},
		{1.50, "$1.50"},
	}
	for _, tt := range tests {
		if got := formatCost(tt.v); got != tt.want {
			t.Errorf("formatCost(%f) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestEstimateCost(t *testing.T) {
	u := llm.Usage{InputTokens: 1000, OutputTokens: 500}
	// claude-sonnet-4-6: $3/M in, $15/M out
	cost := estimateCost(u, "claude-sonnet-4-6")
	// 1000 * 3/1M + 500 * 15/1M = 0.003 + 0.0075 = 0.0105
	if cost < 0.0104 || cost > 0.0106 {
		t.Errorf("cost = %f, want ~0.0105", cost)
	}

	// GLM-5: $0.50/M in, $0.50/M out
	cost = estimateCost(u, "glm-5")
	// 1000 * 0.5/1M + 500 * 0.5/1M = 0.0005 + 0.00025 = 0.00075
	if cost < 0.0007 || cost > 0.0008 {
		t.Errorf("GLM-5 cost = %f, want ~0.00075", cost)
	}

	// GLM-4-flash: $0.01/M in, $0.01/M out (very cheap!)
	cost = estimateCost(u, "glm-4-flash")
	// 1000 * 0.01/1M + 500 * 0.01/1M = 0.00001 + 0.000005 = 0.000015
	if cost < 0.000014 || cost > 0.000016 {
		t.Errorf("GLM-4-flash cost = %f, want ~0.000015", cost)
	}

	// Unknown model returns 0.
	if got := estimateCost(u, "qwen3-coder"); got != 0 {
		t.Errorf("unknown model cost = %f, want 0", got)
	}
}


func TestContextPercent(t *testing.T) {
	// Known model.
	if got := contextPercent(20000, "claude-sonnet-4-6"); got != 10 {
		t.Errorf("contextPercent(20000, claude-sonnet-4-6) = %d, want 10", got)
	}
	// Half full.
	if got := contextPercent(100000, "claude-sonnet-4-6"); got != 50 {
		t.Errorf("contextPercent(100000, claude-sonnet-4-6) = %d, want 50", got)
	}
	// GLM-5 model.
	if got := contextPercent(12800, "glm-5"); got != 10 {
		t.Errorf("contextPercent(12800, glm-5) = %d, want 10", got)
	}
	// GLM-4-plus model.
	if got := contextPercent(64000, "glm-4-plus"); got != 50 {
		t.Errorf("contextPercent(64000, glm-4-plus) = %d, want 50", got)
	}
	// Unknown model returns -1.
	if got := contextPercent(5000, "qwen3-coder"); got != -1 {
		t.Errorf("contextPercent for unknown model = %d, want -1", got)
	}
	// Zero tokens returns -1.
	if got := contextPercent(0, "claude-sonnet-4-6"); got != -1 {
		t.Errorf("contextPercent(0) = %d, want -1", got)
	}
}
