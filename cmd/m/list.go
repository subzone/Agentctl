package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/subzone/Agentctl/examples"
	"github.com/subzone/Agentctl/internal/config"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [dir...]",
		Short: "Discover agent .md files",
		Long: "Scans directories for Markdown files with agent frontmatter.\n" +
			"With no arguments, scans ./examples/agents/ and the current directory.",
		RunE: runList,
	}
}

func runList(_ *cobra.Command, args []string) error {
	dirs := args
	if len(dirs) == 0 {
		dirs = []string{"."}
		if info, err := os.Stat("examples/agents"); err == nil && info.IsDir() {
			dirs = append(dirs, "examples/agents")
		}
		if dir := agentRegistryDir(); dir != "" {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				dirs = append(dirs, dir)
			}
		}
	}

	var agents []agentInfo
	seen := map[string]bool{}
	namesSeen := map[string]bool{}
	for _, dir := range dirs {
		found, err := scanAgents(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", dir, err)
			continue
		}
		for _, a := range found {
			abs, _ := filepath.Abs(a.path)
			if seen[abs] {
				continue
			}
			seen[abs] = true
			namesSeen[a.name] = true
			agents = append(agents, a)
		}
	}

	// Also include bundled agents not yet on disk.
	for _, a := range scanBundledAgents() {
		if !namesSeen[a.name] {
			agents = append(agents, a)
		}
	}

	if len(agents) == 0 {
		fmt.Println("no agents found")
		return nil
	}

	// Print table.
	nameW, modelW := 4, 5
	for _, a := range agents {
		if len(a.name) > nameW {
			nameW = len(a.name)
		}
		if len(a.model) > modelW {
			modelW = len(a.model)
		}
	}
	fmt.Printf("  %-*s  %-*s  %s\n", nameW, "NAME", modelW, "MODEL", "PATH")
	fmt.Printf("  %s  %s  %s\n", strings.Repeat("─", nameW), strings.Repeat("─", modelW), strings.Repeat("─", 40))
	for _, a := range agents {
		fmt.Printf("  %-*s  %-*s  %s\n", nameW, a.name, modelW, a.model, a.path)
	}
	fmt.Printf("\n  %d agents found. Run with: m chat <path>\n", len(agents))
	return nil
}

type agentInfo struct {
	name  string
	model string
	path  string
}

func scanAgents(dir string) ([]agentInfo, error) {
	var out []agentInfo
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "node_modules" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		doc, err := config.ParseFile(path)
		if err != nil {
			return nil // not a valid agent file
		}
		agent, ok := doc.Spec.(*config.AgentSpec)
		if !ok {
			return nil
		}
		out = append(out, agentInfo{
			name:  agent.Name,
			model: agent.Model,
			path:  path,
		})
		return nil
	})
	return out, err
}

func scanBundledAgents() []agentInfo {
	var out []agentInfo
	fs.WalkDir(examples.Agents, "agents", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		data, err := examples.Agents.ReadFile(path)
		if err != nil {
			return nil
		}
		doc, err := config.Parse(data)
		if err != nil {
			return nil
		}
		agent, ok := doc.Spec.(*config.AgentSpec)
		if !ok {
			return nil
		}
		out = append(out, agentInfo{
			name:  agent.Name,
			model: agent.Model,
			path:  "(bundled)",
		})
		return nil
	})
	return out
}
