package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"

	"github.com/subzone/m/internal/llm"
)

// TestAppendHistorySurvivesValueCopy locks in the fix for the v0.0.3
// crash: bubbletea's Update has a value receiver, so the model is
// copied on every call. A bare strings.Builder field panics on the
// next WriteString after the copy. Storing a *Builder makes the copies
// share one backing buffer.
func TestAppendHistorySurvivesValueCopy(t *testing.T) {
	m := tuiModel{
		history:  &strings.Builder{},
		viewport: viewport.New(80, 10),
	}
	m.appendHistory("hello ")
	m2 := m // simulate bubbletea's value-copy on Update
	m2.appendHistory("world")
	if got := m2.history.String(); got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
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

	// Unknown model returns 0.
	if got := estimateCost(u, "ollama/qwen3-coder"); got != 0 {
		t.Errorf("unknown model cost = %f, want 0", got)
	}
}
