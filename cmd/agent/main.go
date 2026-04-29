package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	// Side-effect: register LLM providers.
	_ "github.com/milenkom81/m/internal/llm/anthropic"
	_ "github.com/milenkom81/m/internal/llm/ollama"
	_ "github.com/milenkom81/m/internal/llm/openai"
)

// Version is overridden at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	root := &cobra.Command{
		Use:           "agent",
		Short:         "MD-driven agent for code, infra, and automation work",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newValidateCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newChatCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
