package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// ShellTool runs commands via /bin/sh -c. Output is combined stdout+stderr,
// truncated at MaxOut bytes. A non-zero exit is reported as part of the
// output (not a tool error) so the model can read it and react.
type ShellTool struct {
	Timeout time.Duration
	MaxOut  int
	Shell   string // path to shell; defaults to /bin/sh
}

// NewShell returns a ShellTool with reasonable defaults.
// On Windows, uses cmd.exe; on Unix, /bin/sh.
func NewShell() *ShellTool {
	shell := "/bin/sh"
	if runtime.GOOS == "windows" {
		shell = "cmd.exe"
	}
	return &ShellTool{Timeout: 120 * time.Second, MaxOut: 16 * 1024, Shell: shell}
}

// Name implements Tool.
func (s *ShellTool) Name() string { return "shell" }

// Description implements Tool.
func (s *ShellTool) Description() string {
	return "Run a shell command via /bin/sh -c. Returns combined stdout+stderr. " +
		"Subject to a hard time limit; output is truncated if very large. " +
		"A non-zero exit code is appended to the output as '[exit N]'."
}

// InputSchema implements Tool.
func (s *ShellTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "command": {"type": "string", "description": "Shell command to execute via /bin/sh -c"}
  },
  "required": ["command"]
}`)
}

// Run implements Tool.
func (s *ShellTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.Command == "" {
		return "", errors.New("command is required")
	}

	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shell := s.Shell
	if shell == "" {
		shell = "/bin/sh"
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cctx, shell, "/c", args.Command)
	} else {
		cmd = exec.CommandContext(cctx, shell, "-c", args.Command)
	}
	out, err := cmd.CombinedOutput()
	text := string(out)
	if max := s.MaxOut; max > 0 && len(text) > max {
		text = text[:max] + "\n[output truncated]"
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Sprintf("%s\n[exit %d]", text, exitErr.ExitCode()), nil
		}
		if cctx.Err() == context.DeadlineExceeded {
			return text, fmt.Errorf("shell timed out after %s", timeout)
		}
		return text, fmt.Errorf("shell: %w", err)
	}
	return text, nil
}
