package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"runtime"
	"time"
)

// TestRunTool runs a test command and returns structured output indicating
// pass/fail with the failure output. The model can use this in a loop:
// edit → test → read failures → edit → test until green.
type TestRunTool struct {
	Timeout time.Duration
	MaxOut  int
}

func NewTestRun() *TestRunTool {
	return &TestRunTool{Timeout: 120 * time.Second, MaxOut: 32 * 1024}
}

func (t *TestRunTool) Name() string { return "test_run" }

func (t *TestRunTool) Description() string {
	return "Run a test command and report pass/fail with output. Use this " +
		"after making code changes to verify they work. Returns structured " +
		"output: passed=true/false + the test output. If tests fail, read " +
		"the output and fix the code, then run again."
}

func (t *TestRunTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "command": {
      "type": "string",
      "description": "Test command to run (e.g. 'go test ./...', 'npm test', 'pytest')"
    }
  },
  "required": ["command"]
}`)
}

func (t *TestRunTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.Command == "" {
		return "", errors.New("command is required")
	}

	timeout := t.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cctx, "cmd.exe", "/c", args.Command)
	} else {
		cmd = exec.CommandContext(cctx, "/bin/sh", "-c", args.Command)
	}
	out, err := cmd.CombinedOutput()
	text := string(out)
	if max := t.MaxOut; max > 0 && len(text) > max {
		text = text[:max] + "\n[output truncated]"
	}

	passed := err == nil
	status := "PASSED"
	if !passed {
		status = "FAILED"
	}

	result := fmt.Sprintf("[%s]\n%s", status, strings.TrimRight(text, "\n"))
	if !passed {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result += fmt.Sprintf("\n[exit %d]", exitErr.ExitCode())
		}
	}
	return result, nil
}
