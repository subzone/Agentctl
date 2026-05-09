package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	var staged bool
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show all file changes made by the agent (git diff)",
		Long: `Shows a unified diff of all changes in the working directory.
Useful after an agent session to review what was modified before committing.

Examples:
  m diff            # show unstaged changes
  m diff --staged   # show staged changes (after git add)`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDiff(cmd, staged)
		},
	}
	cmd.Flags().BoolVar(&staged, "staged", false, "Show staged changes instead of unstaged")
	return cmd
}

func runDiff(cmd *cobra.Command, staged bool) error {
	out := cmd.OutOrStdout()

	// Check if we're in a git repo.
	if err := exec.Command("git", "rev-parse", "--git-dir").Run(); err != nil {
		return fmt.Errorf("not a git repository")
	}

	// Get diff.
	args := []string{"diff", "--stat"}
	if staged {
		args = append(args, "--staged")
	}
	statCmd := exec.Command("git", args...)
	statOut, err := statCmd.Output()
	if err != nil {
		return fmt.Errorf("git diff --stat: %w", err)
	}

	stat := strings.TrimSpace(string(statOut))
	if stat == "" {
		if staged {
			fmt.Fprintln(out, "No staged changes.")
		} else {
			fmt.Fprintln(out, "No unstaged changes.")
		}
		return nil
	}

	// Show stat summary.
	fmt.Fprintln(out, stat)
	fmt.Fprintln(out)

	// Show full diff.
	diffArgs := []string{"diff", "--color=always"}
	if staged {
		diffArgs = append(diffArgs, "--staged")
	}
	diffCmd := exec.Command("git", diffArgs...)
	diffCmd.Stdout = out
	diffCmd.Stderr = cmd.ErrOrStderr()
	return diffCmd.Run()
}
