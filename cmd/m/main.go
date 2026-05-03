package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/subzone/Agentctl/internal/config"
	"github.com/subzone/Agentctl/internal/userconfig"

	// Side-effect: register LLM providers.
	_ "github.com/subzone/Agentctl/internal/llm/alibaba"
	_ "github.com/subzone/Agentctl/internal/llm/anthropic"
	_ "github.com/subzone/Agentctl/internal/llm/gemini"
	_ "github.com/subzone/Agentctl/internal/llm/litellm"
	_ "github.com/subzone/Agentctl/internal/llm/ollama"
	_ "github.com/subzone/Agentctl/internal/llm/openai"
)

//go:embed default_agent.md
var defaultAgentMD []byte

const banner = `
███╗   ███╗
████╗ ████║
██╔████╔██║
██║╚██╔╝██║
██║ ╚═╝ ██║
╚═╝     ╚═╝
`

// Version is overridden at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	root := &cobra.Command{
		Use:           "m",
		Short:         "MD-driven agent for code, infra, and automation work",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE:          runDefaultChat,
	}

	root.AddCommand(newValidateCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newChatCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newChangelogCmd())
	root.AddCommand(newTestCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// runDefaultChat is invoked when the user runs `m` with no subcommand. On
// first run (no config file present) it walks the user through the setup
// wizard, otherwise it loads the saved config, hydrates env vars from the
// keychain, and drops into the chat REPL with the embedded default agent.
//
// Precedence for the model id: M_MODEL env var > user config > embedded
// default. M_MODEL escape hatch lets users try a different provider/model
// without touching the config.
func runDefaultChat(cmd *cobra.Command, _ []string) error {
	cfg, err := loadOrCreateConfig(cmd)
	if err != nil {
		return err
	}
	if err := applyConfig(cfg); err != nil {
		return err
	}

	showReleaseNotesIfNeeded(cmd.OutOrStdout(), Version)

	// If the user configured a custom default agent, use it.
	if cfg.DefaultAgent != "" {
		doc, err := config.ParseFile(cfg.DefaultAgent)
		if err != nil {
			return fmt.Errorf("load default agent %s: %w", cfg.DefaultAgent, err)
		}
		fmt.Fprintln(cmd.OutOrStdout())
		return runChatWithDoc(cmd, doc, loadCompanionDocs(cfg.DefaultAgent))
	}

	// Fall back to the embedded default agent.
	doc, err := config.Parse(defaultAgentMD)
	if err != nil {
		return fmt.Errorf("parse embedded default agent: %w", err)
	}
	if spec, ok := doc.Spec.(*config.AgentSpec); ok {
		spec.Model = fmt.Sprintf("%s/%s", cfg.Provider, cfg.Model)
		if override := os.Getenv("M_MODEL"); override != "" {
			spec.Model = override
		}
	}
	// Append the working directory to the system prompt so the model
	// knows where it is on the filesystem.
	if cwd, err := os.Getwd(); err == nil {
		doc.Body += fmt.Sprintf("\n\nCurrent working directory: %s", cwd)
	}

	fmt.Fprintln(cmd.OutOrStdout())
	return runChatWithDoc(cmd, doc, nil)
}

func loadOrCreateConfig(cmd *cobra.Command) (*userconfig.Config, error) {
	if userconfig.Exists() {
		fmt.Fprint(cmd.OutOrStdout(), banner)
		return userconfig.Load()
	}
	return runWizard(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
}

// applyConfig reads any keychain-stored API key for the chosen provider
// and exports it as the env var that provider's factory expects. We
// deliberately don't write the key into the user's shell env — it lives
// only in this process for the duration of the chat session.
func applyConfig(cfg *userconfig.Config) error {
	switch cfg.Provider {
	case userconfig.ProviderOllama:
		if cfg.BaseURL != "" {
			os.Setenv("OLLAMA_HOST", cfg.BaseURL)
		}
	case userconfig.ProviderAnthropic:
		key, err := userconfig.GetAPIKey(cfg.Provider)
		if err != nil {
			return fmt.Errorf("read anthropic key from keychain: %w", err)
		}
		os.Setenv("ANTHROPIC_API_KEY", key)
	case userconfig.ProviderOpenAI:
		key, err := userconfig.GetAPIKey(cfg.Provider)
		if err != nil {
			return fmt.Errorf("read openai key from keychain: %w", err)
		}
		os.Setenv("OPENAI_API_KEY", key)
	case userconfig.ProviderGemini:
		key, err := userconfig.GetAPIKey(cfg.Provider)
		if err != nil {
			return fmt.Errorf("read gemini key from keychain: %w", err)
		}
		os.Setenv("GEMINI_API_KEY", key)
	case userconfig.ProviderAlibaba:
		key, err := userconfig.GetAPIKey(cfg.Provider)
		if err != nil {
			return fmt.Errorf("read alibaba key from keychain: %w", err)
		}
		os.Setenv("DASHSCOPE_API_KEY", key)
	case userconfig.ProviderLiteLLM:
		key, err := userconfig.GetAPIKey(cfg.Provider)
		if err != nil {
			return fmt.Errorf("read litellm key from keychain: %w", err)
		}
		os.Setenv("LITELLM_API_KEY", key)
		os.Setenv("LITELLM_BASE_URL", cfg.BaseURL)
	default:
		return fmt.Errorf("unknown provider %q in config", cfg.Provider)
	}
	return nil
}
