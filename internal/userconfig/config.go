// Package userconfig manages the per-user CLI configuration: which provider
// the bare `m` command uses, the model id, and provider-specific connection
// details. API keys are intentionally excluded from this file — they live in
// the OS keychain (see keychain.go) so the YAML is safe to back up or sync.
package userconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Provider identifies the LLM backend the default chat uses.
type Provider string

const (
	ProviderOllama    Provider = "ollama"
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
	ProviderLiteLLM   Provider = "litellm"
)

// Config is the on-disk schema. Fields are minimal on purpose: anything
// secret stays out (see keychain.go), and anything derivable from the model
// string is not duplicated.
type Config struct {
	Provider Provider `yaml:"provider"`
	Model    string   `yaml:"model"`
	// BaseURL is set for LiteLLM (the proxy endpoint) and may be set for
	// custom Ollama hosts. Empty for Anthropic / vanilla OpenAI.
	BaseURL string `yaml:"base_url,omitempty"`
}

// Path returns the absolute path of the config file
// (~/.config/m/config.yaml on Linux/macOS).
func Path() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// configDir returns ~/.config/m, creating nothing.
func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "m"), nil
}

// Exists reports whether a config file is present. It does not validate it.
func Exists() bool {
	p, err := Path()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Load reads and parses the config file. Returns fs.ErrNotExist when the
// file is missing so callers can distinguish "first run" from corruption.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	if cfg.Provider == "" || cfg.Model == "" {
		return nil, fmt.Errorf("%s: provider and model are required", p)
	}
	return cfg, nil
}

// Save writes the config atomically with mode 0600. Atomic write avoids a
// half-truncated file if the process is killed mid-write.
func Save(cfg *Config) error {
	if cfg.Provider == "" || cfg.Model == "" {
		return errors.New("config: provider and model are required")
	}
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "config-*.yaml")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	final := filepath.Join(dir, "config.yaml")
	return os.Rename(tmpPath, final)
}

// IsNotExist reports whether err indicates the config file is missing.
func IsNotExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }
