package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/subzone/Agentctl/examples"
	"github.com/subzone/Agentctl/internal/config"
	"github.com/subzone/Agentctl/internal/entitlement"
)

func installPackageByName(name string) error {
	name = strings.TrimSpace(name)
	data, err := examples.Packages.ReadFile("packages/" + name + ".md")
	if err != nil {
		return fmt.Errorf("package not found: %s", name)
	}
	doc, err := config.Parse(data)
	if err != nil {
		return err
	}
	pkg, ok := doc.Spec.(*config.PackageSpec)
	if !ok {
		return fmt.Errorf("not a package document")
	}

	state, err := entitlement.Load()
	if err != nil {
		return err
	}
	if missing := state.HasAllEntitlements(pkg.Entitlements); len(missing) > 0 {
		return fmt.Errorf("requires entitlements %v — activate Pro license in Settings", missing)
	}

	for _, agent := range pkg.Agents {
		if err := copyBundledToConfig("agents", agent+".md", userAgentsDir()); err != nil {
			return fmt.Errorf("agent %s: %w", agent, err)
		}
	}
	for _, skill := range pkg.Skills {
		if err := copyBundledToConfig("skills", skill+".md", userSkillsDir()); err != nil {
			return fmt.Errorf("skill %s: %w", skill, err)
		}
	}
	for _, mcp := range pkg.MCP {
		if err := copyBundledToConfig("mcp", mcp+".md", userMCPDir()); err != nil {
			return fmt.Errorf("mcp %s: %w", mcp, err)
		}
	}
	return entitlement.RecordPackageInstall(pkg.Name)
}

func copyBundledToConfig(subdir, filename, destDir string) error {
	name := filepath.Base(filename)
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("invalid filename: %s", filename)
	}
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return err
	}
	dst := filepath.Join(destDir, name)
	rel, err := filepath.Rel(destDir, dst)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("invalid install path: %s", filename)
	}
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	srcPath := subdir + "/" + name
	var data []byte
	switch subdir {
	case "agents":
		data, err = examples.Agents.ReadFile(srcPath)
	case "mcp":
		data, err = examples.MCP.ReadFile(srcPath)
	case "skills":
		data, err = examples.Skills.ReadFile(srcPath)
	default:
		return fmt.Errorf("unknown subdir %s", subdir)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600) // #nosec G703 -- filepath.Base + Rel guard keeps dst under destDir
}

// ListPackageOffers returns bundled packages with lock/install state.
func (a *App) ListPackageOffers() []PackageOffer {
	state, _ := entitlement.Load()
	installed := map[string]bool{}
	for _, p := range state.Packages {
		installed[p] = true
	}

	entries, _ := examples.Packages.ReadDir("packages")
	var out []PackageOffer
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := examples.Packages.ReadFile("packages/" + e.Name())
		if err != nil {
			continue
		}
		doc, err := config.Parse(data)
		if err != nil {
			continue
		}
		pkg, ok := doc.Spec.(*config.PackageSpec)
		if !ok {
			continue
		}
		locked := len(pkg.Entitlements) > 0 && len(state.HasAllEntitlements(pkg.Entitlements)) > 0
		out = append(out, PackageOffer{
			Name:         pkg.Name,
			Description:  pkg.Description,
			Entitlements: pkg.Entitlements,
			Locked:       locked,
			Installed:    installed[pkg.Name],
		})
	}
	return out
}
