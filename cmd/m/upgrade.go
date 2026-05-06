package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade m to the latest version",
		Long: `Upgrades m using the platform's package manager.

macOS:   brew upgrade subzone/tap/m
Linux:   downloads latest .deb or tarball
Source:  go install github.com/subzone/Agentctl/cmd/m@latest`,
		RunE: runUpgrade,
	}
}

func runUpgrade(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()

	fmt.Fprintf(out, "Current version: %s\n", Version)

	// Check for newer version.
	latest := fetchLatestVersion()
	switch {
	case latest == "":
		fmt.Fprintln(out, "Could not check for updates.")
	case latest == Version || "v"+latest == "v"+Version:
		fmt.Fprintln(out, "✓ Already on the latest version.")
		return nil
	default:
		fmt.Fprintf(out, "Latest version:  %s\n\n", latest)
	}

	switch {
	case brewInstalled():
		fmt.Fprintln(out, "Upgrading via Homebrew...")
		c := exec.Command("brew", "upgrade", "subzone/tap/m")
		c.Stdout = out
		c.Stderr = stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("brew upgrade failed: %w", err)
		}
		fmt.Fprintln(out, "\n✓ Upgraded successfully.")

	case runtime.GOOS == "linux" && dpkgInstalled():
		fmt.Fprintln(out, "To upgrade on Linux (deb):")
		fmt.Fprintf(out, "  curl -sLO https://github.com/subzone/Agentctl/releases/latest/download/m_%s_linux_amd64.deb\n", strings.TrimPrefix(latest, "v"))
		fmt.Fprintln(out, "  sudo dpkg -i m_*_linux_amd64.deb")

	case goInstalled():
		fmt.Fprintln(out, "Upgrading via go install...")
		c := exec.Command("go", "install", "github.com/subzone/Agentctl/cmd/m@latest")
		c.Stdout = out
		c.Stderr = stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("go install failed: %w", err)
		}
		fmt.Fprintln(out, "\n✓ Upgraded successfully.")

	default:
		fmt.Fprintln(out, "No supported package manager found.")
		fmt.Fprintln(out, "Download manually: https://github.com/subzone/Agentctl/releases/latest")
	}
	return nil
}

func brewInstalled() bool {
	_, err := exec.LookPath("brew")
	return err == nil
}

func dpkgInstalled() bool {
	_, err := exec.LookPath("dpkg")
	return err == nil
}

func goInstalled() bool {
	_, err := exec.LookPath("go")
	return err == nil
}
