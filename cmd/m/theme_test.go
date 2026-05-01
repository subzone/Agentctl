package main

import (
	"testing"
)

func TestBuiltinThemes(t *testing.T) {
	for name, theme := range Builtin {
		if theme.Name != name {
			t.Errorf("theme %q has Name=%q", name, theme.Name)
		}
		s := theme.Resolve()
		// Just verify Resolve doesn't panic.
		_ = s.User.Render("test")
		_ = s.Tool.Render("test")
		_ = s.Error.Render("test")
	}
}

func TestByNameFound(t *testing.T) {
	if got := ByName("matrix"); got == nil || got.Name != "matrix" {
		t.Errorf("ByName(matrix) = %v", got)
	}
}

func TestByNameNotFound(t *testing.T) {
	if got := ByName("nonexistent"); got != nil {
		t.Errorf("ByName(nonexistent) = %v", got)
	}
}

func TestLoadDefaultsToMatrix(t *testing.T) {
	// With no theme file, Load returns matrix.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	theme := Load()
	if theme.Name != "matrix" {
		t.Errorf("Load() = %q, want matrix", theme.Name)
	}
}
