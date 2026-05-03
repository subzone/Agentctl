package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/subzone/Agentctl/internal/userconfig"
)

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Manage providers and models (interactive)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfigManager(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
}

func runConfigManager(in io.Reader, out, status io.Writer) error {
	w := newWiz(in, out, status)

	cfg, err := userconfig.Load()
	if err != nil {
		fmt.Fprintln(out, "No config found. Run `m init` first or `m` to start the setup wizard.")
		return err
	}

	for {
		fmt.Fprintf(out, "\nCurrent default: %s/%s\n", cfg.Provider, cfg.Model)
		if cfg.DefaultAgent != "" {
			fmt.Fprintf(out, "Default agent:   %s\n", cfg.DefaultAgent)
		}
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Actions:")
		fmt.Fprintln(out, "  s) Scan available models    — query your provider for models")
		fmt.Fprintln(out, "  d) Set default model        — change provider/model for bare `m`")
		fmt.Fprintln(out, "  g) Set default agent        — use a custom agent .md file for bare `m`")
		fmt.Fprintln(out, "  a) Add provider             — set up a new provider + API key")
		fmt.Fprintln(out, "  q) Done")
		fmt.Fprintln(out)

		choice, err := w.prompt("Choice: ")
		if err != nil {
			return nil
		}
		switch strings.ToLower(choice) {
		case "s":
			scanModels(w, cfg)
		case "d":
			if newCfg, err := setDefaultModel(w, cfg); err == nil {
				cfg = newCfg
			} else {
				fmt.Fprintf(out, "error: %v\n", err)
			}
		case "g":
			if newCfg, err := setDefaultAgent(w, cfg); err == nil {
				cfg = newCfg
			} else {
				fmt.Fprintf(out, "error: %v\n", err)
			}
		case "a":
			if newCfg, err := runWizard(in, out, status); err == nil {
				cfg = newCfg
			} else {
				fmt.Fprintf(out, "error: %v\n", err)
			}
		case "q", "":
			return nil
		default:
			fmt.Fprintf(out, "unknown choice %q\n", choice)
		}
	}
}

func setDefaultModel(w *wiz, cfg *userconfig.Config) (*userconfig.Config, error) {
	model, err := w.prompt("New default (provider/model, e.g. anthropic/claude-sonnet-4-6): ")
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(model, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("must be in provider/model format")
	}
	cfg.Provider = userconfig.Provider(parts[0])
	cfg.Model = parts[1]
	if err := userconfig.Save(cfg); err != nil {
		return nil, err
	}
	fmt.Fprintf(w.out, "Default set to %s/%s\n", cfg.Provider, cfg.Model)
	return cfg, nil
}


func setDefaultAgent(w *wiz, cfg *userconfig.Config) (*userconfig.Config, error) {
	path, err := w.prompt("Path to agent .md file (or empty to clear): ")
	if err != nil {
		return nil, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		cfg.DefaultAgent = ""
		if err := userconfig.Save(cfg); err != nil {
			return nil, err
		}
		fmt.Fprintln(w.out, "Default agent cleared (will use built-in)")
		return cfg, nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	cfg.DefaultAgent = path
	if err := userconfig.Save(cfg); err != nil {
		return nil, err
	}
	fmt.Fprintf(w.out, "Default agent set to %s\n", path)
	return cfg, nil
}

func scanModels(w *wiz, cfg *userconfig.Config) {
	fmt.Fprintf(w.out, "\nScanning %s for available models...\n", cfg.Provider)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	models, err := discoverModels(ctx, cfg)
	if err != nil {
		fmt.Fprintf(w.out, "error: %v\n", err)
		return
	}
	if len(models) == 0 {
		fmt.Fprintln(w.out, "no models found")
		return
	}

	fmt.Fprintf(w.out, "\nAvailable models (%d):\n", len(models))
	for i, m := range models {
		marker := "  "
		if m == cfg.Model {
			marker = "* "
		}
		fmt.Fprintf(w.out, "  %s%d) %s\n", marker, i+1, m)
	}
	fmt.Fprintln(w.out, "\n  * = current default")
}

// discoverModels queries the provider's API for available models.
func discoverModels(ctx context.Context, cfg *userconfig.Config) ([]string, error) {
	switch cfg.Provider {
	case userconfig.ProviderAnthropic:
		return discoverAnthropic(ctx)
	case userconfig.ProviderOpenAI:
		return discoverOpenAI(ctx, "https://api.openai.com", "OPENAI_API_KEY")
	case userconfig.ProviderGemini:
		return discoverGemini(ctx)
	case userconfig.ProviderAlibaba:
		key, err := apiKeyFromEnvOrKeychain("DASHSCOPE_API_KEY")
		if err != nil {
			return nil, err
		}
		return discoverDashScope(ctx, key)
	case userconfig.ProviderOllama:
		return discoverOllama(ctx)
	case userconfig.ProviderLiteLLM:
		return nil, fmt.Errorf("LiteLLM model discovery not supported — add models manually with /model")
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}

func discoverOllama(ctx context.Context) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:11434/api/tags", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama not reachable: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range result.Models {
		out = append(out, m.Name)
	}
	return out, nil
}

func discoverOpenAI(ctx context.Context, baseURL, envKey string) ([]string, error) {
	key, err := apiKeyFromEnvOrKeychain(envKey)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %s", resp.Status)
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range result.Data {
		// Filter to chat-capable models (skip embeddings, whisper, etc.)
		if strings.Contains(m.ID, "gpt") || strings.Contains(m.ID, "o3") ||
			strings.Contains(m.ID, "o4") || strings.Contains(m.ID, "qwen") {
			out = append(out, m.ID)
		}
	}
	return out, nil
}

func discoverAnthropic(ctx context.Context) ([]string, error) {
	key, err := apiKeyFromEnvOrKeychain("ANTHROPIC_API_KEY")
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/v1/models", nil)
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %s", resp.Status)
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range result.Data {
		out = append(out, m.ID)
	}
	return out, nil
}

func discoverGemini(ctx context.Context) ([]string, error) {
	key, err := apiKeyFromEnvOrKeychain("GEMINI_API_KEY")
	if err != nil {
		return nil, err
	}
	url := "https://generativelanguage.googleapis.com/v1beta/models?key=" + key
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %s", resp.Status)
	}
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range result.Models {
		// name is "models/gemini-2.5-pro" — strip prefix
		name := strings.TrimPrefix(m.Name, "models/")
		if strings.Contains(name, "gemini") {
			out = append(out, name)
		}
	}
	return out, nil
}

// discoverDashScope queries the Alibaba DashScope models endpoint.
// Returns all model IDs without filtering.
func discoverDashScope(ctx context.Context, key string) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://dashscope-intl.aliyuncs.com/compatible-mode/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DashScope API returned %s", resp.Status)
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range result.Data {
		out = append(out, m.ID)
	}
	return out, nil
}

// apiKeyFromEnvOrKeychain tries the env var first, then the keychain.
func apiKeyFromEnvOrKeychain(envVar string) (string, error) {
	if key := os.Getenv(envVar); key != "" {
		return key, nil
	}
	// Map env var to provider name for keychain lookup.
	providerMap := map[string]userconfig.Provider{
		"ANTHROPIC_API_KEY": userconfig.ProviderAnthropic,
		"OPENAI_API_KEY":    userconfig.ProviderOpenAI,
		"GEMINI_API_KEY":    userconfig.ProviderGemini,
		"DASHSCOPE_API_KEY": userconfig.ProviderAlibaba,
		"LITELLM_API_KEY":   userconfig.ProviderLiteLLM,
	}
	if p, ok := providerMap[envVar]; ok {
		key, err := userconfig.GetAPIKey(p)
		if err == nil {
			return key, nil
		}
	}
	return "", fmt.Errorf("%s not set and no key in keychain", envVar)
}
