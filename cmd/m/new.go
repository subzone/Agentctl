package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newNewCmd() *cobra.Command {
	var model, dir string
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a new agent .md file",
		Long: `Creates a new agent Markdown file with boilerplate frontmatter.

Examples:
  m new my-agent              # creates my-agent.md in current directory
  m new devops --model ollama/qwen3-coder
  m new reviewer --dir ./agents/`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runNew(args[0], model, dir)
		},
	}
	cmd.Flags().StringVar(&model, "model", "anthropic/claude-sonnet-4-6", "default model")
	cmd.Flags().StringVar(&dir, "dir", ".", "output directory")
	return cmd
}

func runNew(name, model, dir string) error {
	name = strings.TrimSuffix(name, ".md")

	filename := name + ".md"
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		filename = dir + "/" + filename
	}

	if _, err := os.Stat(filename); err == nil {
		return fmt.Errorf("%s already exists", filename)
	}

	content := fmt.Sprintf(`---
name: %s
type: agent
description: TODO — describe what this agent does.
version: 1
model: %s
fallback:
  - anthropic/claude-haiku-4-5-20251001
  - openai/gpt-4.1
tools:
  - shell
  - fs_read
  - fs_write
  - fs_list
  - git
  - test_run
  - web_fetch
  - code_search
temperature: 0.3
---
You are a helpful assistant. Describe your role and expertise here.

When the user asks for help:

1. Explore the project with fs_list and fs_read to understand the codebase.
2. Use code_search to find relevant files and symbols.
3. Make targeted changes with fs_write.
4. Run tests with test_run to verify changes.
5. Use git to commit when done.
`, name, model)

	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		return err
	}

	fmt.Printf("created %s\n", filename)
	fmt.Printf("edit the system prompt, then run: m chat %s\n", filename)
	return nil
}
