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
	ProviderGemini    Provider = "gemini"
	ProviderAlibaba   Provider = "alibaba"
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
	// DefaultAgent is an optional path to a custom .md agent file used by
	// bare `m` instead of the embedded default. Empty means use the builtin.
	DefaultAgent string `yaml:"default_agent,omitempty"`
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

// envVarForProvider returns the environment variable name that holds the
// API key for a given provider. Returns empty string for providers that
// don't use an API key (e.g., local Ollama).
func envVarForProvider(p Provider) string {
	switch p {
	case ProviderAnthropic:
		return "ANTHROPIC_API_KEY"
	case ProviderOpenAI:
		return "OPENAI_API_KEY"
	case ProviderGemini:
		return "GEMINI_API_KEY"
	case ProviderAlibaba:
		return "DASHSCOPE_API_KEY"
	case ProviderLiteLLM:
		return "LITELLM_API_KEY"
	case ProviderOllama:
		return "" // Local, no API key needed
	default:
		return ""
	}
}

// GetAPIKeyWithFallback tries the keychain first, then falls back to the
// corresponding environment variable. This allows users without keychain
// tooling to simply export the API key in their shell.
//
// Returns the key string if found in either location, or an error if neither
// exists. For providers that don't require an API key (e.g., local Ollama),
// returns empty string and no error.
func GetAPIKeyWithFallback(provider Provider) (string, error) {
	// For providers that don't need an API key, short-circuit.
	envVar := envVarForProvider(provider)
	if envVar == "" {
		return "", nil
	}

	// Try keychain first.
	key, err := GetAPIKey(provider)
	if err == nil {
		return key, nil
	}

	// If keychain error is anything other than "not found", it might be
	// a tool-missing error on Linux. In that case, still try env var.
	// Also try env var if the key simply wasn't stored.
	if !IsKeyNotFound(err) {
		// Keychain tool missing or broken; try env var as fallback.
		if val := os.Getenv(envVar); val != "" {
			return val, nil
		}
		// Return original error to preserve context about keychain failure.
		return "", err
	}

	// Key not found in keychain — try environment variable.
	if val := os.Getenv(envVar); val != "" {
		return val, nil
	}

	// Neither keychain nor env var has the key.
	return "", fmt.Errorf("no API key found for %s: neither in keychain nor in $%s", provider, envVar)
}
