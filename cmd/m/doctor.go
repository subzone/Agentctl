package main

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"github.com/subzone/Agentctl/internal/userconfig"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check if AgentCTL is configured correctly",
		Args:  cobra.NoArgs,
		RunE:  runDoctor,
	}
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()
	ok := true

	fmt.Fprintln(out, "AgentCTL Doctor")
	fmt.Fprintln(out, "===============")
	fmt.Fprintln(out)

	// 1. Config file
	cfg, err := userconfig.Load()
	if err != nil {
		fmt.Fprintln(out, "✗ Config: not found — run `m` to start the setup wizard")
		ok = false
	} else {
		fmt.Fprintf(out, "✓ Config: %s/%s\n", cfg.Provider, cfg.Model)
		if cfg.DefaultAgent != "" {
			fmt.Fprintf(out, "  Default agent: %s\n", cfg.DefaultAgent)
		}
		if cfg.BaseURL != "" {
			fmt.Fprintf(out, "  Base URL: %s\n", cfg.BaseURL)
		}
	}

	// 2. API key
	if cfg != nil {
		key, kerr := userconfig.GetAPIKeyWithFallback(cfg.Provider)
		if kerr != nil || key == "" {
			fmt.Fprintf(out, "✗ API key for %s: not found in keychain or env\n", cfg.Provider)
			ok = false
		} else {
			fmt.Fprintf(out, "✓ API key for %s: found (%s...)\n", cfg.Provider, key[:min(8, len(key))])
		}
	}

	// 3. Model reachable (quick ping)
	if cfg != nil && cfg.Provider != userconfig.ProviderOllama {
		fmt.Fprintf(out, "  Checking model reachability...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		baseURL := "https://api.anthropic.com"
		switch cfg.Provider {
		case userconfig.ProviderOpenAI:
			baseURL = "https://api.openai.com"
		case userconfig.ProviderGemini:
			baseURL = "https://generativelanguage.googleapis.com"
		case userconfig.ProviderAlibaba:
			if cfg.BaseURL != "" {
				baseURL = cfg.BaseURL
			} else {
				baseURL = "https://dashscope-intl.aliyuncs.com"
			}
		case userconfig.ProviderGroq:
			baseURL = "https://api.groq.com"
		case userconfig.ProviderCerebras:
			baseURL = "https://api.cerebras.ai"
		case userconfig.ProviderMistral:
			baseURL = "https://api.mistral.ai"
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodHead, baseURL, nil)
		resp, rerr := http.DefaultClient.Do(req)
		if rerr != nil {
			fmt.Fprintf(out, " ✗ unreachable (%v)\n", rerr)
			ok = false
		} else {
			resp.Body.Close()
			fmt.Fprintln(out, " ✓ reachable")
		}
	}

	// 4. Ollama check
	if cfg != nil && cfg.Provider == userconfig.ProviderOllama {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:11434/api/tags", nil)
		resp, rerr := http.DefaultClient.Do(req)
		if rerr != nil {
			fmt.Fprintln(out, "✗ Ollama: not running (start with `ollama serve`)")
			ok = false
		} else {
			resp.Body.Close()
			fmt.Fprintln(out, "✓ Ollama: running")
		}
	}

	// 5. Tools
	fmt.Fprintln(out)
	checkTool(out, "git", "git")
	checkTool(out, "ripgrep (optional, faster code_search)", "rg")
	checkTool(out, "grep", "grep")

	// 6. Session encryption
	fmt.Fprintln(out)
	_, serr := sessionStore()
	if serr != nil {
		fmt.Fprintf(out, "✗ Session store: %v\n", serr)
		ok = false
	} else {
		fmt.Fprintln(out, "✓ Session store: ready")
	}

	fmt.Fprintln(out)
	if ok {
		fmt.Fprintln(out, "All checks passed. You're good to go!")
	} else {
		fmt.Fprintln(out, "Some checks failed. Fix the issues above and run `m doctor` again.")
	}
	return nil
}

func checkTool(out interface{ Write([]byte) (int, error) }, label, bin string) {
	if _, err := exec.LookPath(bin); err != nil {
		fmt.Fprintf(out, "✗ %s: not found\n", label)
	} else {
		fmt.Fprintf(out, "✓ %s: found\n", label)
	}
}
