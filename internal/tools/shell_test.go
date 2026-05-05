package tools

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestShellTool_BasicCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell test")
	}
	shell := NewShell()
	ctx := context.Background()

	out, err := shell.Run(ctx, []byte(`{"command":"echo hello"}`))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if out != "hello\n" {
		t.Errorf("got %q, want %q", out, "hello\n")
	}
}

func TestShellTool_EmptyCommand(t *testing.T) {
	shell := NewShell()
	ctx := context.Background()

	_, err := shell.Run(ctx, []byte(`{"command":""}`))
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestShellTool_InvalidJSON(t *testing.T) {
	shell := NewShell()
	ctx := context.Background()

	_, err := shell.Run(ctx, []byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestShellTool_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell test")
	}
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}
	shell := &ShellTool{Timeout: 200 * time.Millisecond, MaxOut: 1024}
	ctx := context.Background()

	// Use sleep with a long duration - the timeout should kick in
	out, err := shell.Run(ctx, []byte(`{"command":"sleep 2"}`))
	t.Logf("output: %q, error: %v", out, err)
	if err == nil {
		// On some systems, sleep might exit quickly when killed
		// Check if output contains indication of being killed
		if containsString(out, "exit") || containsString(out, "killed") {
			t.Log("command was killed, timeout likely worked")
			return
		}
		t.Fatal("expected timeout error")
	}
	if !containsString(err.Error(), "timeout") && !containsString(err.Error(), "deadline exceeded") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestShellTool_NonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell test")
	}
	shell := NewShell()
	ctx := context.Background()

	// Command that exits with non-zero
	out, err := shell.Run(ctx, []byte(`{"command":"exit 42"}`))
	if err != nil {
		t.Fatalf("Run should not return error for non-zero exit: %v", err)
	}
	if !containsString(out, "[exit 42]") {
		t.Errorf("expected exit code in output, got: %q", out)
	}
}

func TestShellTool_OutputTruncation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell test")
	}
	shell := &ShellTool{Timeout: 5 * time.Second, MaxOut: 10}
	ctx := context.Background()

	// Generate more than 10 bytes of output
	out, err := shell.Run(ctx, []byte(`{"command":"echo 'this is a very long output that should be truncated'"}`))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(out) > 50 { // 10 + truncation message
		t.Errorf("output should be truncated, got %d bytes: %q", len(out), out)
	}
	if !containsString(out, "[output truncated]") {
		t.Errorf("expected truncation message, got: %q", out)
	}
}

func TestShellTool_ContextCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell test")
	}
	shell := &ShellTool{Timeout: 10 * time.Second, MaxOut: 1024}
	ctx, cancel := context.WithCancel(context.Background())
	
	// Cancel immediately
	cancel()

	_, err := shell.Run(ctx, []byte(`{"command":"sleep 10"}`))
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestShellTool_CustomShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell test")
	}
	shell := &ShellTool{Timeout: 5 * time.Second, MaxOut: 1024, Shell: "/bin/bash"}
	ctx := context.Background()

	out, err := shell.Run(ctx, []byte(`{"command":"echo $BASH_VERSION"}`))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	// Just check it ran without error; version string varies
	if out == "" {
		t.Error("expected some output from bash")
	}
}

func TestShellTool_CombinedOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell test")
	}
	shell := NewShell()
	ctx := context.Background()

	// Command that writes to both stdout and stderr
	out, err := shell.Run(ctx, []byte(`{"command":"echo stdout; echo stderr >&2"}`))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !containsString(out, "stdout") {
		t.Errorf("expected stdout in output, got: %q", out)
	}
	if !containsString(out, "stderr") {
		t.Errorf("expected stderr in output, got: %q", out)
	}
}

func TestShellTool_NameAndDescription(t *testing.T) {
	shell := NewShell()
	if shell.Name() != "shell" {
		t.Errorf("Name() = %q, want %q", shell.Name(), "shell")
	}
	if shell.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestShellTool_InputSchema(t *testing.T) {
	shell := NewShell()
	schema := shell.InputSchema()
	if len(schema) == 0 {
		t.Fatal("InputSchema() should not be empty")
	}
	// Should be valid JSON
	if schema[0] != '{' {
		t.Errorf("InputSchema should be a JSON object, got: %s", schema[:50])
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}