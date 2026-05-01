package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// GitTool provides structured git operations. Wraps the git CLI rather
// than using go-git to keep dependencies minimal.
type GitTool struct {
	Timeout time.Duration
}

func NewGit() *GitTool {
	return &GitTool{Timeout: 30 * time.Second}
}

func (g *GitTool) Name() string { return "git" }

func (g *GitTool) Description() string {
	return "Run git operations: status, diff, log, add, commit, branch, checkout. " +
		"Use this instead of shell for git commands — it provides structured output."
}

func (g *GitTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "operation": {
      "type": "string",
      "enum": ["status", "diff", "log", "add", "commit", "branch", "checkout", "stash"],
      "description": "Git operation to perform"
    },
    "args": {
      "type": "string",
      "description": "Additional arguments (e.g. file paths for add, message for commit, branch name for checkout)"
    }
  },
  "required": ["operation"]
}`)
}

func (g *GitTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Operation string `json:"operation"`
		Args      string `json:"args"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	var cmdArgs []string
	switch args.Operation {
	case "status":
		cmdArgs = []string{"status", "--short"}
	case "diff":
		cmdArgs = []string{"diff"}
		if args.Args != "" {
			cmdArgs = append(cmdArgs, strings.Fields(args.Args)...)
		}
	case "log":
		cmdArgs = []string{"log", "--oneline", "-20"}
		if args.Args != "" {
			cmdArgs = append(cmdArgs, strings.Fields(args.Args)...)
		}
	case "add":
		if args.Args == "" {
			cmdArgs = []string{"add", "-A"}
		} else {
			cmdArgs = append([]string{"add"}, strings.Fields(args.Args)...)
		}
	case "commit":
		if args.Args == "" {
			return "", errors.New("commit requires a message in args")
		}
		cmdArgs = []string{"commit", "-m", args.Args}
	case "branch":
		if args.Args == "" {
			cmdArgs = []string{"branch", "--list"}
		} else {
			cmdArgs = []string{"branch", args.Args}
		}
	case "checkout":
		if args.Args == "" {
			return "", errors.New("checkout requires a branch name in args")
		}
		cmdArgs = append([]string{"checkout"}, strings.Fields(args.Args)...)
	case "stash":
		if args.Args == "" {
			cmdArgs = []string{"stash"}
		} else {
			cmdArgs = append([]string{"stash"}, strings.Fields(args.Args)...)
		}
	default:
		return "", fmt.Errorf("unknown git operation %q", args.Operation)
	}

	timeout := g.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, "git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimRight(string(out), "\n")
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Sprintf("%s\n[exit %d]", text, exitErr.ExitCode()), nil
		}
		return text, fmt.Errorf("git: %w", err)
	}
	if text == "" {
		text = "(no output)"
	}
	return text, nil
}
