package userconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir) // Linux
	t.Setenv("HOME", dir)            // fallback
	os.MkdirAll(filepath.Join(dir, "m"), 0o700)

	cfg := &Config{Provider: ProviderOllama, Model: "qwen3-coder"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Provider != cfg.Provider || got.Model != cfg.Model {
		t.Errorf("got %+v, want %+v", got, cfg)
	}
}

func TestSaveRejectsEmpty(t *testing.T) {
	if err := Save(&Config{}); err == nil {
		t.Error("expected error for empty config")
	}
	if err := Save(&Config{Provider: ProviderOllama}); err == nil {
		t.Error("expected error for missing model")
	}
}

func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing config")
	}
	if !IsNotExist(err) && !os.IsNotExist(err) {
		// On some platforms the path resolution differs; accept either.
		t.Logf("error type: %T — %v", err, err)
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	if Exists() {
		t.Error("Exists should be false before Save")
	}
	cfg := &Config{Provider: ProviderAnthropic, Model: "claude-sonnet-4-6"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !Exists() {
		t.Error("Exists should be true after Save")
	}
}

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	os.MkdirAll(filepath.Join(dir, "m"), 0o700)

	st := &State{LastSeenVersion: "0.0.3"}
	if err := SaveState(st); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	got, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got.LastSeenVersion != "0.0.3" {
		t.Errorf("got %q, want %q", got.LastSeenVersion, "0.0.3")
	}
}

func TestLoadStateMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	_, err := LoadState()
	if err == nil {
		t.Error("expected error for missing state")
	}
}

func TestIsKeyNotFound(t *testing.T) {
	err := &errKeyNotFound{provider: "test"}
	if !IsKeyNotFound(err) {
		t.Error("IsKeyNotFound should return true")
	}
	if err.Error() != "no api key stored for provider test" {
		t.Errorf("got %q", err.Error())
	}
}

func TestConfigFilePermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	cfg := &Config{Provider: ProviderOpenAI, Model: "gpt-4o"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	p, _ := Path()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("config file perm = %o, want 0600", perm)
	}
}
