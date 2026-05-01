package userconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// State is the on-disk record of what version's release notes the user
// has already been shown. Kept separate from Config because it's
// machine-managed, not user-edited: the wizard never writes it, and
// nothing user-visible references it.
type State struct {
	LastSeenVersion string `yaml:"last_seen_version,omitempty"`
}

// statePath returns ~/Library/Application Support/m/state.yaml on macOS
// and ~/.config/m/state.yaml on Linux — the same dir as config.yaml.
func statePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.yaml"), nil
}

// LoadState returns the persisted state. Returns fs.ErrNotExist on first
// run (the file doesn't exist yet); callers should treat that as
// "everything is unseen" rather than an error.
func LoadState() (*State, error) {
	p, err := statePath()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	st := &State{}
	if err := yaml.Unmarshal(raw, st); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return st, nil
}

// SaveState writes state atomically with mode 0644 (not secret).
func SaveState(st *State) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	out, err := yaml.Marshal(st)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "state-*.yaml")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	final, _ := statePath()
	return os.Rename(tmpPath, final)
}
