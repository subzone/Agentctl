package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildBinary builds the m binary for integration tests.
// Returns the path to the binary.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "m")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func TestCLIVersion(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "m version") {
		t.Errorf("expected 'm version' in output, got: %s", out)
	}
}

func TestCLIHelp(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("--help failed: %v\n%s", err, out)
	}
	output := string(out)
	for _, want := range []string{"chat", "run", "list", "new", "doctor", "install", "validate", "completion"} {
		if !strings.Contains(output, want) {
			t.Errorf("--help missing subcommand %q", want)
		}
	}
}

func TestCLINew(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	out, err := exec.Command(bin, "new", "test-agent", "--dir", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("new failed: %v\n%s", err, out)
	}
	// Verify file was created
	path := filepath.Join(dir, "test-agent.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("agent file not created: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "name: test-agent") {
		t.Error("missing name in frontmatter")
	}
	if !strings.Contains(s, "type: agent") {
		t.Error("missing type in frontmatter")
	}
	if !strings.Contains(s, "tools:") {
		t.Error("missing tools in frontmatter")
	}
}

func TestCLINewAlreadyExists(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	// Create first
	exec.Command(bin, "new", "dupe", "--dir", dir).Run()
	// Try again — should fail
	out, err := exec.Command(bin, "new", "dupe", "--dir", dir).CombinedOutput()
	if err == nil {
		t.Fatal("expected error for duplicate agent")
	}
	if !strings.Contains(string(out), "already exists") {
		t.Errorf("expected 'already exists' error, got: %s", out)
	}
}

func TestCLIList(t *testing.T) {
	bin := buildBinary(t)
	// Run from repo root where examples/agents/ exists
	cmd := exec.Command(bin, "list")
	cmd.Dir = "../../" // go up from cmd/m to repo root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "agents found") {
		t.Errorf("expected 'agents found' in output, got: %s", output)
	}
}

func TestCLIValidate(t *testing.T) {
	bin := buildBinary(t)
	// Create a valid agent
	dir := t.TempDir()
	agent := filepath.Join(dir, "valid.md")
	os.WriteFile(agent, []byte(`---
name: valid
type: agent
model: ollama/test
tools:
  - shell
---
You are a test agent.
`), 0644)

	out, err := exec.Command(bin, "validate", agent).CombinedOutput()
	if err != nil {
		t.Fatalf("validate failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "0 issue") {
		t.Errorf("expected 0 issues, got: %s", out)
	}
}

func TestCLIValidateInvalid(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	agent := filepath.Join(dir, "bad.md")
	os.WriteFile(agent, []byte(`---
name: bad
type: agent
---
Missing model field.
`), 0644)

	out, _ := exec.Command(bin, "validate", agent).CombinedOutput()
	if !strings.Contains(string(out), "issue") {
		t.Errorf("expected validation issues, got: %s", out)
	}
}

func TestCLICompletion(t *testing.T) {
	bin := buildBinary(t)
	for _, shell := range []string{"bash", "zsh", "fish"} {
		out, err := exec.Command(bin, "completion", shell).CombinedOutput()
		if err != nil {
			t.Errorf("completion %s failed: %v", shell, err)
			continue
		}
		if len(out) < 100 {
			t.Errorf("completion %s output too short (%d bytes)", shell, len(out))
		}
	}
}

func TestCLIDoctor(t *testing.T) {
	bin := buildBinary(t)
	out, _ := exec.Command(bin, "doctor").CombinedOutput()
	output := string(out)
	// Doctor should at least check for git
	if !strings.Contains(output, "git") {
		t.Errorf("doctor should check for git, got: %s", output)
	}
}

func TestResolveAgentPath(t *testing.T) {
	// Test literal path
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte("test"), 0644)
	if got := resolveAgentPath(path); got != path {
		t.Errorf("resolveAgentPath(%q) = %q, want %q", path, got, path)
	}

	// Test non-existent returns original
	if got := resolveAgentPath("nonexistent-xyz-123"); got != "nonexistent-xyz-123" {
		t.Errorf("resolveAgentPath(nonexistent) = %q, want original", got)
	}
}
