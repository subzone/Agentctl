package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/subzone/Agentctl/internal/config"
	"github.com/subzone/Agentctl/internal/mcp"
	"github.com/subzone/Agentctl/internal/userconfig"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Manage MCP server installations and connections",
		Long: `Install, configure, and check MCP servers.

Examples:
  m mcp setup jira          # install + configure Jira MCP server
  m mcp setup github        # install + configure GitHub MCP server
  m mcp setup developer-hub # install all MCPs an agent needs
  m mcp status              # check connectivity of all configured servers`,
	}

	cmd.AddCommand(newMCPSetupCmd())
	cmd.AddCommand(newMCPStatusCmd())
	cmd.AddCommand(newMCPListCmd())

	return cmd
}

func newMCPSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup <server-or-agent>",
		Short: "Install and configure an MCP server (or all servers an agent needs)",
		Args:  cobra.ExactArgs(1),
		RunE:  runMCPSetup,
	}
}

func newMCPStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check connectivity of configured MCP servers",
		Args:  cobra.NoArgs,
		RunE:  runMCPStatus,
	}
}

func newMCPListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available MCP server definitions",
		Args:  cobra.NoArgs,
		RunE:  runMCPList,
	}
}

func runMCPSetup(cmd *cobra.Command, args []string) error {
	name := args[0]
	out := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()
	in := cmd.InOrStdin()

	// Check if it's an agent — if so, setup all its MCP servers.
	path := resolveAgentPath(name)
	if doc, err := config.ParseFile(path); err == nil {
		if agent, ok := doc.Spec.(*config.AgentSpec); ok && len(agent.MCP) > 0 {
			fmt.Fprintf(out, "Agent %s requires MCP servers: %s\n\n", agent.Name, strings.Join(agent.MCP, ", "))
			for _, mcpName := range agent.MCP {
				if err := setupMCPServer(mcpName, out, stderr, in); err != nil {
					fmt.Fprintf(stderr, "  ✗ %s: %v\n", mcpName, err)
				}
			}
			return nil
		}
	}

	// Otherwise treat as an MCP server name.
	return setupMCPServer(name, out, stderr, in)
}

func setupMCPServer(name string, out, stderr io.Writer, in io.Reader) error {
	fmt.Fprintf(out, "Setting up MCP server: %s\n", name)

	// Find the MCP definition.
	spec, err := findMCPSpec(name)
	if err != nil {
		return err
	}

	// Step 1: Check if command is already installed.
	if len(spec.Command) > 0 {
		binary := spec.Command[0]
		if _, err := exec.LookPath(binary); err == nil {
			fmt.Fprintf(out, "  ✓ %s already installed\n", binary)
		} else {
			// Step 2: Install.
			if err := installMCPBinary(spec, out, stderr); err != nil {
				return fmt.Errorf("install %s: %w", binary, err)
			}
		}
	}

	// Step 3: Configure credentials.
	if err := configureMCPEnv(spec, out, in); err != nil {
		return err
	}

	// Step 4: Verify connectivity.
	fmt.Fprintf(out, "  → verifying connectivity...\n")
	if err := verifyMCPServer(spec); err != nil {
		fmt.Fprintf(out, "  ⚠ connectivity check failed: %v\n", err)
		fmt.Fprintf(out, "    (server may still work — check credentials and try again)\n")
	} else {
		fmt.Fprintf(out, "  ✓ %s is reachable\n", name)
	}

	fmt.Fprintf(out, "\n")
	return nil
}

func findMCPSpec(name string) (*config.MCPServerSpec, error) {
	// Search in examples/mcp/ and agent registry.
	dirs := []string{"examples/mcp"}
	if dir := agentRegistryDir(); dir != "" {
		parent := filepath.Dir(dir)
		mcpDir := filepath.Join(parent, "mcp")
		if info, err := os.Stat(mcpDir); err == nil && info.IsDir() {
			dirs = append(dirs, mcpDir)
		}
	}

	for _, dir := range dirs {
		path := filepath.Join(dir, name+".md")
		if doc, err := config.ParseFile(path); err == nil {
			if spec, ok := doc.Spec.(*config.MCPServerSpec); ok {
				return spec, nil
			}
		}
	}

	// Try bundled.
	bp := bundledAgentPath(name)
	if bp != "" {
		if doc, err := config.ParseFile(bp); err == nil {
			if spec, ok := doc.Spec.(*config.MCPServerSpec); ok {
				return spec, nil
			}
		}
	}

	return nil, fmt.Errorf("MCP server definition %q not found (looked in examples/mcp/ and config dir)", name)
}

func installMCPBinary(spec *config.MCPServerSpec, out, stderr io.Writer) error {
	if spec.Install == nil {
		return fmt.Errorf("%s not found on PATH and no install instructions in definition", spec.Command[0])
	}

	var cmdline []string
	switch {
	case spec.Install.Pip != "" && hasBinary("pip"):
		cmdline = []string{"pip", "install", spec.Install.Pip}
	case spec.Install.Pip != "" && hasBinary("pip3"):
		cmdline = []string{"pip3", "install", spec.Install.Pip}
	case spec.Install.Npm != "" && hasBinary("npm"):
		cmdline = []string{"npm", "install", "-g", spec.Install.Npm}
	case spec.Install.Brew != "" && hasBinary("brew"):
		cmdline = []string{"brew", "install", spec.Install.Brew}
	default:
		hints := []string{}
		if spec.Install.Pip != "" {
			hints = append(hints, "pip install "+spec.Install.Pip)
		}
		if spec.Install.Npm != "" {
			hints = append(hints, "npm install -g "+spec.Install.Npm)
		}
		if spec.Install.Brew != "" {
			hints = append(hints, "brew install "+spec.Install.Brew)
		}
		return fmt.Errorf("no supported package manager found. Install manually:\n    %s", strings.Join(hints, "\n    "))
	}

	fmt.Fprintf(out, "  → installing: %s\n", strings.Join(cmdline, " "))
	cmd := exec.Command(cmdline[0], cmdline[1:]...)
	cmd.Stdout = out
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	fmt.Fprintf(out, "  ✓ installed\n")
	return nil
}

func configureMCPEnv(spec *config.MCPServerSpec, out io.Writer, in io.Reader) error {
	if len(spec.Env) == 0 {
		return nil
	}

	sc := bufio.NewScanner(in)
	for key, tmpl := range spec.Env {
		// ${secret:name} — needs keychain storage
		if strings.HasPrefix(tmpl, "${secret:") {
			secretName := strings.TrimSuffix(strings.TrimPrefix(tmpl, "${secret:"), "}")
			// Check if already stored.
			if existing, _ := userconfig.GetAPIKeyByName(secretName); existing != "" {
				fmt.Fprintf(out, "  ✓ %s: already configured\n", key)
				continue
			}
			fmt.Fprintf(out, "  %s (secret, stored in keychain)\n", key)
			fmt.Fprintf(out, "    Paste value: ")
			if !sc.Scan() {
				return fmt.Errorf("no input for %s", key)
			}
			val := strings.TrimSpace(sc.Text())
			if val == "" {
				fmt.Fprintf(out, "    skipped\n")
				continue
			}
			if err := userconfig.SaveAPIKeyByName(secretName, val); err != nil {
				return fmt.Errorf("save %s to keychain: %w", secretName, err)
			}
			fmt.Fprintf(out, "  ✓ %s: saved to keychain\n", key)

		} else if strings.HasPrefix(tmpl, "${env:") {
			// ${env:NAME} — needs env var set
			envName := strings.TrimSuffix(strings.TrimPrefix(tmpl, "${env:"), "}")
			if os.Getenv(envName) != "" {
				fmt.Fprintf(out, "  ✓ %s: already set (%s)\n", key, envName)
				continue
			}
			fmt.Fprintf(out, "  %s (env var: %s)\n", key, envName)
			fmt.Fprintf(out, "    Value: ")
			if !sc.Scan() {
				return fmt.Errorf("no input for %s", key)
			}
			val := strings.TrimSpace(sc.Text())
			if val == "" {
				fmt.Fprintf(out, "    skipped (set %s in your shell profile)\n", envName)
				continue
			}
			os.Setenv(envName, val)
			fmt.Fprintf(out, "  ✓ %s: set for this session (add to shell profile for persistence)\n", key)
		}
	}
	return nil
}

func verifyMCPServer(spec *config.MCPServerSpec) error {
	if spec.Transport != "stdio" || len(spec.Command) == 0 {
		return nil // can't verify non-stdio or URL-based servers easily
	}

	// Try to start the server and list tools.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mgr, err := mcp.Open(ctx, []*config.MCPServerSpec{spec}, "m-setup", io.Discard)
	if err != nil {
		return err
	}
	defer mgr.Close()

	results := mgr.HealthCheck(ctx)
	for name, err := range results {
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func runMCPStatus(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()

	// Find all MCP definitions.
	specs := findAllMCPSpecs()
	if len(specs) == 0 {
		fmt.Fprintln(out, "No MCP server definitions found.")
		return nil
	}

	fmt.Fprintf(out, "MCP servers (%d):\n\n", len(specs))
	for _, spec := range specs {
		binary := "(http)"
		if len(spec.Command) > 0 {
			binary = spec.Command[0]
		}

		installed := "✗ not installed"
		if len(spec.Command) > 0 {
			if _, err := exec.LookPath(spec.Command[0]); err == nil {
				installed = "✓ installed"
			}
		} else {
			installed = "✓ (url-based)"
		}

		fmt.Fprintf(out, "  %-15s  %-20s  %s\n", spec.Name, binary, installed)
	}
	return nil
}

func runMCPList(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()
	specs := findAllMCPSpecs()
	if len(specs) == 0 {
		fmt.Fprintln(out, "No MCP server definitions found.")
		return nil
	}

	fmt.Fprintf(out, "  %-15s  %-12s  %s\n", "NAME", "TRANSPORT", "DESCRIPTION")
	fmt.Fprintf(out, "  %s  %s  %s\n", strings.Repeat("─", 15), strings.Repeat("─", 12), strings.Repeat("─", 40))
	for _, spec := range specs {
		desc := spec.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		fmt.Fprintf(out, "  %-15s  %-12s  %s\n", spec.Name, spec.Transport, desc)
	}
	return nil
}

func findAllMCPSpecs() []*config.MCPServerSpec {
	var specs []*config.MCPServerSpec
	dirs := []string{"examples/mcp"}
	if dir := agentRegistryDir(); dir != "" {
		parent := filepath.Dir(dir)
		mcpDir := filepath.Join(parent, "mcp")
		dirs = append(dirs, mcpDir)
	}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			doc, err := config.ParseFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			if spec, ok := doc.Spec.(*config.MCPServerSpec); ok {
				specs = append(specs, spec)
			}
		}
	}
	return specs
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
