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
	"github.com/subzone/Agentctl/internal/entitlement"
)

func newPackagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packages",
		Short: "Discover and install curated package bundles",
	}
	cmd.AddCommand(newPackagesListCmd())
	cmd.AddCommand(newPackagesInstallCmd())
	return cmd
}

func newPackagesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [dir...]",
		Short: "List package .md files",
		Long:  "Scans directories for Markdown files with package frontmatter.\nWith no arguments, scans ./examples/packages/ and the current directory.",
		RunE:  runPackages,
	}
}

func runPackages(_ *cobra.Command, args []string) error {
	dirs := args
	if len(dirs) == 0 {
		dirs = []string{"."}
		if info, err := os.Stat("examples/packages"); err == nil && info.IsDir() {
			dirs = append(dirs, "examples/packages")
		}
		if dir := agentRegistryDir(); dir != "" {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				dirs = append(dirs, dir)
			}
		}
	}

	var packages []packageInfo
	seen := map[string]bool{}
	for _, dir := range dirs {
		found, err := scanPackages(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", dir, err)
			continue
		}
		for _, p := range found {
			abs, _ := filepath.Abs(p.path)
			if seen[abs] {
				continue
			}
			seen[abs] = true
			packages = append(packages, p)
		}
	}

	for _, p := range scanBundledPackages() {
		abs := "(bundled)/" + p.name
		if seen[abs] {
			continue
		}
		seen[abs] = true
		packages = append(packages, p)
	}

	if len(packages) == 0 {
		fmt.Println("no packages found")
		return nil
	}

	state, _ := entitlement.Load()

	nameW, agentsW, skillsW, mcpW, entW := 4, 6, 6, 3, 11
	for _, p := range packages {
		if len(p.name) > nameW {
			nameW = len(p.name)
		}
		if len(p.agents) > agentsW {
			agentsW = len(p.agents)
		}
		if len(p.skills) > skillsW {
			skillsW = len(p.skills)
		}
		if len(p.mcp) > mcpW {
			mcpW = len(p.mcp)
		}
		if len(p.entitlements) > entW {
			entW = len(p.entitlements)
		}
	}

	fmt.Printf("  %-*s  %-*s  %-*s  %-*s  %-*s  %s\n", nameW, "NAME", agentsW, "AGENTS", skillsW, "SKILLS", mcpW, "MCP", entW, "ENTITLEMENTS", "STATUS")
	fmt.Printf("  %s  %s  %s  %s  %s  %s\n", strings.Repeat("─", nameW), strings.Repeat("─", agentsW), strings.Repeat("─", skillsW), strings.Repeat("─", mcpW), strings.Repeat("─", entW), strings.Repeat("─", 12))
	for _, p := range packages {
		status := "open"
		if len(p.entitlements) > 0 {
			if missing := state.HasAllEntitlements(p.entitlements); len(missing) > 0 {
				status = "locked"
			} else {
				status = "licensed"
			}
		}
		fmt.Printf("  %-*s  %-*d  %-*d  %-*d  %-*d  %s\n", nameW, p.name, agentsW, len(p.agents), skillsW, len(p.skills), mcpW, len(p.mcp), entW, len(p.entitlements), status)
	}
	fmt.Printf("\n  %d packages found. Install: m packages install <name>\n", len(packages))
	return nil
}

type packageInfo struct {
	name         string
	agents       []string
	skills       []string
	mcp          []string
	entitlements []string
	path         string
}

func scanPackages(dir string) ([]packageInfo, error) {
	var out []packageInfo
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
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
			return nil
		}
		pkg, ok := doc.Spec.(*config.PackageSpec)
		if !ok {
			return nil
		}
		out = append(out, packageInfo{
			name:         pkg.Name,
			agents:       append([]string(nil), pkg.Agents...),
			skills:       append([]string(nil), pkg.Skills...),
			mcp:          append([]string(nil), pkg.MCP...),
			entitlements: append([]string(nil), pkg.Entitlements...),
			path:         path,
		})
		return nil
	})
	return out, err
}

func scanBundledPackages() []packageInfo {
	var out []packageInfo
	fs.WalkDir(examples.Packages, "packages", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		data, err := examples.Packages.ReadFile(path)
		if err != nil {
			return nil
		}
		doc, err := config.Parse(data)
		if err != nil {
			return nil
		}
		pkg, ok := doc.Spec.(*config.PackageSpec)
		if !ok {
			return nil
		}
		out = append(out, packageInfo{
			name:         pkg.Name,
			agents:       append([]string(nil), pkg.Agents...),
			skills:       append([]string(nil), pkg.Skills...),
			mcp:          append([]string(nil), pkg.MCP...),
			entitlements: append([]string(nil), pkg.Entitlements...),
			path:         "(bundled)",
		})
		return nil
	})
	return out
}
