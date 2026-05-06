package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/subzone/Agentctl/internal/ports"
)

func newExportCmd() *cobra.Command {
	var format string
	var output string
	cmd := &cobra.Command{
		Use:   "export <session-id>",
		Short: "Export a saved session to JSON or Markdown",
		Long: `Export a session for analysis, sharing, or backup.

Examples:
  m export 2026-05-04_15-30 --format json --output session.json
  m export _autosave --format markdown --output session.md
  m export --format json --output latest.json  # exports autosave`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExport(cmd, args, format, output)
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "Export format: json, markdown")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path (default: stdout)")
	return cmd
}

func runExport(cmd *cobra.Command, args []string, format, output string) error {
	// Determine session ID
	sessionID := "_autosave" // default to autosave
	if len(args) > 0 {
		sessionID = args[0]
	}

	// Load session
	store, err := sessionStore()
	if err != nil {
		return fmt.Errorf("open session store: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := store.LoadSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("load session %s: %w", sessionID, err)
	}

	// Export based on format
	var result string
	switch strings.ToLower(format) {
	case "json":
		result, err = exportJSON(data)
		if err != nil {
			return fmt.Errorf("export json: %w", err)
		}
	case "markdown", "md":
		result = exportMarkdown(data)
	default:
		return fmt.Errorf("unknown format: %s (use json or markdown)", format)
	}

	// Write output
	if output == "" {
		fmt.Fprint(cmd.OutOrStdout(), result)
		return nil
	}

	if err := os.WriteFile(output, []byte(result), 0644); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Exported session %s to %s (%d bytes)\n", sessionID, output, len(result))
	return nil
}

func exportJSON(data *ports.SessionData) (string, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func exportMarkdown(data *ports.SessionData) string {
	var md strings.Builder

	md.WriteString("# Session Export\n\n")
	fmt.Fprintf(&md, "**Provider:** %s\n", data.Provider)
	fmt.Fprintf(&md, "**Model:** %s\n", data.Model)
	fmt.Fprintf(&md, "**Tokens:** In=%d, Out=%d, Total=%d\n\n",
		data.InputTokens, data.OutputTokens, data.InputTokens+data.OutputTokens)
	md.WriteString("---\n\n")

	for i, m := range data.Messages {
		fmt.Fprintf(&md, "## Turn %d (%s)\n\n", i+1, m.Role)
		for _, b := range m.Content {
			switch b.Type {
			case "text":
				fmt.Fprintf(&md, "%s\n\n", b.Text)
			case "tool_use":
				fmt.Fprintf(&md, "**Tool:** `%s`\n\n", b.ToolName)
				if b.ToolInput != "" {
					fmt.Fprintf(&md, "```json\n%s\n```\n\n", b.ToolInput)
				}
			case "tool_result":
				fmt.Fprintf(&md, "**Result:** (%d bytes)\n\n", len(b.Output))
				if len(b.Output) > 500 {
					fmt.Fprintf(&md, "```\n%s...\n```\n\n", b.Output[:500])
				} else if b.Output != "" {
					fmt.Fprintf(&md, "```\n%s\n```\n\n", b.Output)
				}
				if b.IsError {
					md.WriteString("⚠️ **Error**\n\n")
				}
			}
		}
		md.WriteString("---\n\n")
	}

	return md.String()
}

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage saved sessions",
		Long: `List, export, or delete saved sessions.

Examples:
  m session list
  m session export _autosave --format json
  m session delete old-session`,
	}

	cmd.AddCommand(newSessionListCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newSessionDeleteCmd())

	return cmd
}

func newSessionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := sessionStore()
			if err != nil {
				return fmt.Errorf("open session store: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ids, err := store.ListSessions(ctx)
			if err != nil {
				return fmt.Errorf("list sessions: %w", err)
			}

			if len(ids) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No saved sessions")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Saved sessions (%d):\n", len(ids))
			for i, id := range ids {
				fmt.Fprintf(cmd.OutOrStdout(), "  %d) %s\n", i+1, id)
			}
			return nil
		},
	}
}

func newSessionDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <session-id>",
		Short: "Delete a saved session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := args[0]

			store, err := sessionStore()
			if err != nil {
				return fmt.Errorf("open session store: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := store.DeleteSession(ctx, sessionID); err != nil {
				return fmt.Errorf("delete session %s: %w", sessionID, err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Deleted session: %s\n", sessionID)
			return nil
		},
	}
}