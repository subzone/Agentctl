package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/subzone/m/internal/llm"
	"github.com/subzone/m/internal/userconfig"
)

func newTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test [provider/model...]",
		Short: "Smoke test providers — sends a minimal request to verify setup",
		Long: "Tests one or more provider/model combinations. With no arguments,\n" +
			"tests the configured default. Pass specific models to test them:\n" +
			"  m test anthropic/claude-sonnet-4-6 ollama/qwen2.5-coder:7b",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProviderTests(cmd.OutOrStdout(), cmd.ErrOrStderr(), args)
		},
	}
}

func runProviderTests(out, status io.Writer, models []string) error {
	if len(models) == 0 {
		// Test the configured default.
		cfg, err := loadDefaultConfig()
		if err != nil {
			return fmt.Errorf("no config found — run `m init` first: %w", err)
		}
		models = []string{fmt.Sprintf("%s/%s", cfg.Provider, cfg.Model)}
	}

	fmt.Fprintf(out, "Testing %d model(s)...\n\n", len(models))

	passed, failed := 0, 0
	for _, model := range models {
		result := testModel(model)
		if result.err != nil {
			fmt.Fprintf(out, "  ✗ %-40s %s (%s)\n", model, result.err, result.duration)
			failed++
		} else {
			fmt.Fprintf(out, "  ✓ %-40s %d tokens in %s\n", model, result.tokens, result.duration)
			if result.tools {
				fmt.Fprintf(out, "    tools: ✓\n")
			}
			passed++
		}
	}

	fmt.Fprintf(out, "\n%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		return fmt.Errorf("%d test(s) failed", failed)
	}
	return nil
}

type testResult struct {
	err      error
	tokens   int
	tools    bool
	duration time.Duration
}

func testModel(model string) testResult {
	start := time.Now()

	provider, bare, err := llm.Resolve(model)
	if err != nil {
		return testResult{err: err, duration: time.Since(start)}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test 1: basic text generation.
	events, err := provider.Stream(ctx, llm.Request{
		Model:     bare,
		System:    "Reply with exactly: OK",
		Messages:  []llm.Message{llm.TextMessage(llm.RoleUser, "test")},
		MaxTokens: 10,
	})
	if err != nil {
		return testResult{err: err, duration: time.Since(start)}
	}

	var text strings.Builder
	var tokens int
	for ev := range events {
		switch ev.Kind {
		case llm.EventText:
			text.WriteString(ev.Text)
		case llm.EventUsage:
			tokens = ev.Usage.InputTokens + ev.Usage.OutputTokens
		case llm.EventError:
			return testResult{err: ev.Err, duration: time.Since(start)}
		}
	}

	if text.Len() == 0 {
		return testResult{err: fmt.Errorf("no text returned"), duration: time.Since(start)}
	}

	// Test 2: tool calling (quick check).
	toolsWork := testToolCalling(ctx, provider, bare)

	return testResult{
		tokens:   tokens,
		tools:    toolsWork,
		duration: time.Since(start),
	}
}

func testToolCalling(ctx context.Context, provider llm.Provider, model string) bool {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	events, err := provider.Stream(ctx, llm.Request{
		Model:    model,
		System:   "Call the test_tool with value 42.",
		Messages: []llm.Message{llm.TextMessage(llm.RoleUser, "go")},
		Tools: []llm.ToolSchema{{
			Name:        "test_tool",
			Description: "A test tool. Call it with the value parameter.",
			InputSchema: []byte(`{"type":"object","properties":{"value":{"type":"integer"}},"required":["value"]}`),
		}},
		MaxTokens: 50,
	})
	if err != nil {
		return false
	}

	for ev := range events {
		if ev.Kind == llm.EventToolCall {
			return true
		}
		if ev.Kind == llm.EventError {
			return false
		}
	}
	return false
}

func loadDefaultConfig() (*userconfig.Config, error) {
	cfg, err := userconfig.Load()
	if err != nil {
		return nil, err
	}
	if err := applyConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
