package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/subzone/Agentctl/internal/ports"
)

func newCostCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cost",
		Short: "Show token usage and estimated cost for recent sessions",
		RunE:  runCost,
	}
}

func runCost(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()

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
		fmt.Fprintln(out, "No sessions found.")
		return nil
	}

	fmt.Fprintf(out, "  %-25s  %-25s  %8s  %8s  %10s\n", "SESSION", "MODEL", "IN", "OUT", "COST")
	fmt.Fprintf(out, "  %s  %s  %s  %s  %s\n",
		strings.Repeat("─", 25), strings.Repeat("─", 25),
		strings.Repeat("─", 8), strings.Repeat("─", 8), strings.Repeat("─", 10))

	var totalCost float64
	var totalIn, totalOut int

	// Show last 10 sessions.
	start := 0
	if len(ids) > 10 {
		start = len(ids) - 10
	}

	for _, id := range ids[start:] {
		data, err := store.LoadSession(ctx, id)
		if err != nil {
			continue
		}
		cost := estimateSessionCost(data)
		totalCost += cost
		totalIn += data.InputTokens
		totalOut += data.OutputTokens

		model := data.Model
		if len(model) > 25 {
			model = model[:22] + "..."
		}
		sessID := id
		if len(sessID) > 25 {
			sessID = sessID[:22] + "..."
		}

		fmt.Fprintf(out, "  %-25s  %-25s  %8s  %8s  %10s\n",
			sessID, model,
			formatTokens(data.InputTokens), formatTokens(data.OutputTokens),
			formatCost(cost))
	}

	fmt.Fprintf(out, "\n  Total: %s in, %s out — %s\n",
		formatTokens(totalIn), formatTokens(totalOut), formatCost(totalCost))

	return nil
}

func estimateSessionCost(data *ports.SessionData) float64 {
	model := data.Model
	// Strip provider prefix if present.
	if i := strings.LastIndex(model, "/"); i >= 0 {
		model = model[i+1:]
	}
	p, ok := modelPricing[model]
	if !ok {
		return 0
	}
	return (float64(data.InputTokens)*p.input + float64(data.OutputTokens)*p.output) / 1_000_000
}

// (formatTokens and formatCost are defined in tui.go)
