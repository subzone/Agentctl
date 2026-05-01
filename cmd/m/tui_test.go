package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
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
