package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/subzone/Agentctl/internal/config"
	"github.com/subzone/Agentctl/internal/engine"
	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/userconfig"
)

func newPipeCmd() *cobra.Command {
	var model string
	cmd := &cobra.Command{
		Use:   "pipe [instruction]",
		Short: "One-shot: read stdin, apply instruction, write to stdout",
		Long: `Reads stdin, prepends it to the instruction, sends to the LLM,
and writes the response to stdout. No REPL, no TUI — pure pipe.

Examples:
  cat error.log | m pipe "explain this error"
  git diff | m pipe "write a commit message"
  kubectl get pods | m pipe "which pods are unhealthy?"
  cat main.go | m pipe "find bugs" > review.md`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipe(cmd, args, model)
		},
	}
	cmd.Flags().StringVarP(&model, "model", "m", "", "Override model (e.g. anthropic/claude-sonnet-4-6)")
	return cmd
}

func runPipe(cmd *cobra.Command, args []string, modelOverride string) error {
	instruction := strings.Join(args, " ")

	// Read stdin.
	stdinData, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	input := strings.TrimSpace(string(stdinData))

	// Build the task: stdin content + instruction.
	var task string
	if input != "" {
		task = fmt.Sprintf("INPUT:\n```\n%s\n```\n\nINSTRUCTION: %s", input, instruction)
	} else {
		task = instruction
	}

	// Resolve model.
	modelID := modelOverride
	if modelID == "" {
		cfg, err := userconfig.Load()
		if err != nil {
			return fmt.Errorf("no model configured: %w (use --model or run `m init`)", err)
		}
		if err := applyConfig(cfg); err != nil {
			return err
		}
		modelID = fmt.Sprintf("%s/%s", cfg.Provider, cfg.Model)
	}
	if override := os.Getenv("M_MODEL"); override != "" {
		modelID = override
	}

	provider, model, err := llm.Resolve(modelID)
	if err != nil {
		return err
	}

	// Parse embedded default agent for system prompt.
	doc, _ := config.Parse(defaultAgentMD)
	system := "You are a helpful assistant. Respond concisely and directly. No preamble."
	if doc != nil {
		system = doc.Body
	}

	return engine.Run(cmd.Context(), engine.Config{
		Provider: provider,
		Model:    model,
		System:   system,
		Out:      cmd.OutOrStdout(),
		Status:   io.Discard,
	}, task)
}
