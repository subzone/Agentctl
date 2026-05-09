package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/subzone/Agentctl/internal/tools"
)

// --- m search ---

func TestCLISearch(t *testing.T) {
	bin := buildBinary(t)
	// Run from repo root where examples/agents/ exists.
	cmd := exec.Command(bin, "search", "devops")
	cmd.Dir = "../../"
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("search failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "devops") {
		t.Errorf("search for 'devops' should find devops agent: %s", output)
	}
	if !strings.Contains(output, "match") {
		t.Errorf("expected 'match' in output: %s", output)
	}
}

func TestCLISearchNoResults(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "search", "zzzznonexistent999")
	cmd.Dir = "../../"
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("search failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "no agents matching") {
		t.Errorf("expected 'no agents matching': %s", out)
	}
}

func TestCLISearchFuzzy(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "search", "steva")
	cmd.Dir = "../../"
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("search failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "steva-djubre") {
		t.Errorf("fuzzy search for 'steva' should find steva-djubre: %s", output)
	}
}

// --- m run --dry-run ---

func TestCLIDryRun(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "run", "--dry-run", "devops", "test task")
	cmd.Dir = "../../"
	out, err := cmd.CombinedOutput()
	output := string(out)
	// Dry run may fail on provider resolution (no API key) but should show agent info.
	if !strings.Contains(output, "Agent:") || !strings.Contains(output, "devops") {
		t.Errorf("dry-run should show agent info: %s", output)
	}
	if !strings.Contains(output, "Model:") {
		t.Errorf("dry-run should show model: %s", output)
	}
	// Should NOT contain "calling LLM" or actual response text.
	if strings.Contains(output, "calling LLM") {
		t.Errorf("dry-run should not call LLM: %s", output)
	}
	_ = err // exit code may be non-zero if provider can't resolve
}

func TestCLIDryRunInvalidAgent(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "run", "--dry-run", "nonexistent-agent-xyz").CombinedOutput()
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
	if !strings.Contains(string(out), "no such file") && !strings.Contains(string(out), "parse") {
		t.Errorf("expected file error: %s", out)
	}
}

// --- m mcp list ---

func TestCLIMCPList(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "mcp", "list")
	cmd.Dir = "../../"
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mcp list failed: %v\n%s", err, out)
	}
	output := string(out)
	for _, want := range []string{"jira", "github", "confluence"} {
		if !strings.Contains(output, want) {
			t.Errorf("mcp list missing %q: %s", want, output)
		}
	}
}

func TestCLIMCPStatus(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "mcp", "status")
	cmd.Dir = "../../"
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mcp status failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "MCP servers") {
		t.Errorf("mcp status should show header: %s", output)
	}
}

// --- m upgrade ---

func TestCLIUpgrade(t *testing.T) {
	bin := buildBinary(t)
	out, _ := exec.Command(bin, "upgrade").CombinedOutput()
	output := string(out)
	// Should at least show current version.
	if !strings.Contains(output, "Current version:") {
		t.Errorf("upgrade should show current version: %s", output)
	}
}

// --- fs_write_multi ---

func TestFSWriteMultiAtomic(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "a.txt")
	file2 := filepath.Join(dir, "b.txt")

	tool := tools.NewFSWriteMulti(nil, nil)
	input, _ := json.Marshal(map[string]any{
		"files": []map[string]any{
			{"path": file1, "mode": "create", "content": "hello"},
			{"path": file2, "mode": "create", "content": "world"},
		},
	})

	out, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "2 files") {
		t.Errorf("output = %q", out)
	}

	got1, _ := os.ReadFile(file1)
	got2, _ := os.ReadFile(file2)
	if string(got1) != "hello" || string(got2) != "world" {
		t.Errorf("files = %q, %q", got1, got2)
	}
}

func TestFSWriteMultiRollbackOnFailure(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "exists.txt")
	os.WriteFile(file1, []byte("original"), 0o644)

	tool := tools.NewFSWriteMulti(nil, nil)
	nonexistent := filepath.Join(dir, "nonexistent.txt")
	input, _ := json.Marshal(map[string]any{
		"files": []map[string]any{
			{"path": file1, "mode": "patch", "old_str": "original", "new_str": "modified"},
			{"path": nonexistent, "mode": "patch", "old_str": "x", "new_str": "y"},
		},
	})

	_, err := tool.Run(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for failed patch")
	}
	if !strings.Contains(err.Error(), "rolled back") {
		t.Errorf("error should mention rollback: %v", err)
	}

	// First file should be rolled back to original.
	got, _ := os.ReadFile(file1)
	if string(got) != "original" {
		t.Errorf("file1 should be rolled back, got %q", got)
	}
}

func TestFSWriteMultiDeclined(t *testing.T) {
	decline := func(_ context.Context, _ string) (bool, error) { return false, nil }
	tool := tools.NewFSWriteMulti(decline, nil)
	input := json.RawMessage(`{"files": [{"path": "/tmp/x", "mode": "create", "content": "x"}]}`)

	out, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "user declined the write" {
		t.Errorf("out = %q", out)
	}
}

func TestFSWriteMultiEmpty(t *testing.T) {
	tool := tools.NewFSWriteMulti(nil, nil)
	_, err := tool.Run(context.Background(), json.RawMessage(`{"files": []}`))
	if err == nil {
		t.Fatal("expected error for empty files")
	}
}

// --- m session list ---

func TestCLISessionList(t *testing.T) {
	bin := buildBinary(t)
	out, err := exec.Command(bin, "session", "list").CombinedOutput()
	if err != nil {
		// May fail if no sessions exist, but shouldn't crash.
		if !strings.Contains(string(out), "No saved sessions") && !strings.Contains(string(out), "open session store") {
			t.Fatalf("session list failed unexpectedly: %v\n%s", err, out)
		}
	}
}

// --- fuzzyMatch ---

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		query, target string
		want          bool
	}{
		{"dev", "devops", true},
		{"dvps", "devops", true},    // subsequence
		{"steva", "steva-djubre", true},
		{"xyz", "devops", false},
		{"", "anything", true},       // empty query matches all
		{"devops", "dev", false},     // query longer than target
	}
	for _, tt := range tests {
		got := fuzzyMatch(tt.query, tt.target)
		if got != tt.want {
			t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", tt.query, tt.target, got, tt.want)
		}
	}
}

// --- helpers ---

func TestCLIRunYesFlag(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell test")
	}
	bin := buildBinary(t)
	// --yes should be accepted as a flag (even if it fails due to no API key).
	out, _ := exec.Command(bin, "run", "--yes", "../../examples/agents/hello.md", "test").CombinedOutput()
	output := string(out)
	// Should not say "unknown flag".
	if strings.Contains(output, "unknown flag") {
		t.Errorf("--yes flag not recognized: %s", output)
	}
}
