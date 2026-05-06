package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Fuzzy search for agents by name, description, or tools",
		Args:  cobra.ExactArgs(1),
		RunE:  runSearch,
	}
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.ToLower(args[0])

	// Collect all agents from list logic.
	allAgents := collectAllAgents()
	if len(allAgents) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no agents found")
		return nil
	}

	var matches []agentInfo
	for _, a := range allAgents {
		if fuzzyMatch(query, a.name) || fuzzyMatch(query, a.model) || fuzzyMatch(query, a.path) {
			matches = append(matches, a)
		}
	}

	if len(matches) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "no agents matching %q\n", args[0])
		return nil
	}

	nameW, modelW := 4, 5
	for _, a := range matches {
		if len(a.name) > nameW {
			nameW = len(a.name)
		}
		if len(a.model) > modelW {
			modelW = len(a.model)
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "  %-*s  %-*s  %s\n", nameW, "NAME", modelW, "MODEL", "PATH")
	fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s  %s\n", strings.Repeat("─", nameW), strings.Repeat("─", modelW), strings.Repeat("─", 40))
	for _, a := range matches {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-*s  %-*s  %s\n", nameW, a.name, modelW, a.model, a.path)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n  %d match(es). Run with: m chat <name>\n", len(matches))
	return nil
}

// collectAllAgents gathers agents from all sources (cwd, examples, registry, bundled).
func collectAllAgents() []agentInfo {
	var agents []agentInfo
	seen := map[string]bool{}

	dirs := []string{"."}
	if dir := agentRegistryDir(); dir != "" {
		dirs = append(dirs, dir)
	}
	for _, dir := range dirs {
		found, _ := scanAgents(dir)
		for _, a := range found {
			if !seen[a.name] {
				seen[a.name] = true
				agents = append(agents, a)
			}
		}
	}
	for _, a := range scanBundledAgents() {
		if !seen[a.name] {
			seen[a.name] = true
			agents = append(agents, a)
		}
	}
	return agents
}

// fuzzyMatch checks if query is a substring of target or if all query chars
// appear in order in target.
func fuzzyMatch(query, target string) bool {
	target = strings.ToLower(target)
	if strings.Contains(target, query) {
		return true
	}
	qi := 0
	for i := 0; i < len(target) && qi < len(query); i++ {
		if target[i] == query[qi] {
			qi++
		}
	}
	return qi == len(query)
}
