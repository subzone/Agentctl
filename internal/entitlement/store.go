package entitlement

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Path returns ~/.config/m/entitlement.json.
func Path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "m", "entitlement.json"), nil
}

// Load reads entitlement state from disk. Missing file => free tier.
func Load() (State, error) {
	p, err := Path()
	if err != nil {
		return Default(), err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Default(), err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return Default(), fmt.Errorf("parse entitlement: %w", err)
	}
	if s.Plan == "" {
		s.Plan = PlanFree
	}
	if s.IsExpired() {
		return Default(), nil
	}
	return s, nil
}

// Save persists entitlement state.
func Save(s State) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// Clear removes entitlement file (revert to free).
func Clear() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// RecordPackageInstall appends an installed package name to state.
func RecordPackageInstall(name string) error {
	s, err := Load()
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("package name required")
	}
	for _, p := range s.Packages {
		if p == name {
			return nil
		}
	}
	s.Packages = append(s.Packages, name)
	return Save(s)
}
