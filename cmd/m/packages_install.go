package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/subzone/Agentctl/examples"
	"github.com/subzone/Agentctl/internal/config"
	"github.com/subzone/Agentctl/internal/entitlement"
)

func skillsRegistryDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m", "skills")
}

func newPackagesInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install <package-name>",
		Short: "Install a curated package bundle to ~/.config/m/",
		Long:  "Copies agents, skills, and MCP definitions referenced by a package manifest.\nPro packages require an active license — run `m license activate` first.",
		Args:  cobra.ExactArgs(1),
		RunE:  runPackageInstall,
	}
}

func runPackageInstall(cmd *cobra.Command, args []string) error {
	name := strings.TrimSpace(args[0])
	pkg, path, err := findPackageByName(name)
	if err != nil {
		return err
	}

	state, err := entitlement.Load()
	if err != nil {
		return err
	}
	if missing := state.HasAllEntitlements(pkg.Entitlements); len(missing) > 0 {
		return fmt.Errorf("package %q requires entitlements %v — your plan is %s\nRun: m license activate <key> or upgrade at https://subzone.github.io/Agentctl/",
			pkg.Name, missing, state.Plan)
	}

	out := cmd.OutOrStdout()
	var installed []string

	for _, agent := range pkg.Agents {
		if err := installBundledFile("agents", agent+".md", agentRegistryDir()); err != nil {
			return fmt.Errorf("agent %s: %w", agent, err)
		}
		fmt.Fprintf(out, "  ✓ agent %s\n", agent)
		installed = append(installed, "agent:"+agent)
	}
	for _, skill := range pkg.Skills {
		if err := installBundledFile("skills", skill+".md", skillsRegistryDir()); err != nil {
			return fmt.Errorf("skill %s: %w", skill, err)
		}
		fmt.Fprintf(out, "  ✓ skill %s\n", skill)
		installed = append(installed, "skill:"+skill)
	}
	for _, mcp := range pkg.MCP {
		if err := installBundledFile("mcp", mcp+".md", mcpRegistryDir()); err != nil {
			return fmt.Errorf("mcp %s: %w", mcp, err)
		}
		fmt.Fprintf(out, "  ✓ mcp %s\n", mcp)
		installed = append(installed, "mcp:"+mcp)
	}
	for _, tool := range pkg.Tools {
		if err := installBundledFile("tools", tool+".md", toolsRegistryDir()); err != nil {
			return fmt.Errorf("tool %s: %w", tool, err)
		}
		fmt.Fprintf(out, "  ✓ tool %s\n", tool)
		installed = append(installed, "tool:"+tool)
	}

	if err := entitlement.RecordPackageInstall(pkg.Name); err != nil {
		return err
	}

	fmt.Fprintf(out, "\nInstalled package %q from %s (%d items).\n", pkg.Name, path, len(installed))
	fmt.Fprintln(out, "Start a New Chat in desktop (or `m chat`) to use the new extensions.")
	return nil
}

func findPackageByName(name string) (*config.PackageSpec, string, error) {
	name = strings.TrimSpace(name)
	for _, p := range scanBundledPackages() {
		if p.name == name {
			data, err := examples.Packages.ReadFile("packages/" + name + ".md")
			if err != nil {
				return nil, "", err
			}
			doc, err := config.Parse(data)
			if err != nil {
				return nil, "", err
			}
			pkg, ok := doc.Spec.(*config.PackageSpec)
			if !ok {
				return nil, "", fmt.Errorf("not a package document")
			}
			return pkg, "(bundled)", nil
		}
	}
	found, err := scanPackages(".")
	if err != nil {
		return nil, "", err
	}
	if info, err := os.Stat("examples/packages"); err == nil && info.IsDir() {
		more, _ := scanPackages("examples/packages")
		found = append(found, more...)
	}
	for _, p := range found {
		if p.name == name {
			doc, err := config.ParseFile(p.path)
			if err != nil {
				return nil, "", err
			}
			pkg, ok := doc.Spec.(*config.PackageSpec)
			if !ok {
				continue
			}
			return pkg, p.path, nil
		}
	}
	return nil, "", fmt.Errorf("package not found: %s (try: m packages)", name)
}

func installBundledFile(subdir, filename, destDir string) error {
	if destDir == "" {
		return fmt.Errorf("cannot resolve config directory")
	}
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return err
	}
	dst := filepath.Join(destDir, filename)
	if _, err := os.Stat(dst); err == nil {
		return nil // already installed
	}

	// Try bundled examples first.
	srcPath := subdir + "/" + filename
	var data []byte
	var err error
	switch subdir {
	case "agents":
		data, err = examples.Agents.ReadFile(srcPath)
	case "mcp":
		data, err = examples.MCP.ReadFile(srcPath)
	case "skills":
		data, err = examples.Skills.ReadFile(srcPath)
	default:
		data, err = examples.Packages.ReadFile(srcPath)
	}
	if err != nil {
		// Fallback: examples/ on disk (dev clone).
		disk := filepath.Join("examples", subdir, filename)
		data, err = os.ReadFile(disk)
		if err != nil {
			return fmt.Errorf("bundled file missing: %s/%s", subdir, filename)
		}
	}
	return os.WriteFile(dst, data, 0o600)
}
