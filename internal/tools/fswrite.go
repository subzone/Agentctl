package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ConfirmFunc asks the user to approve an action. Returns true if approved.
// Implementations live in the CLI layer (stdin prompt, TUI dialog, etc.).
// A nil ConfirmFunc auto-approves everything (useful for non-interactive runs).
type ConfirmFunc func(ctx context.Context, prompt string) (bool, error)

// FSWriteTool writes or patches a UTF-8 text file on disk. Every write is
// gated by a ConfirmFunc so the user sees the proposed change before it
// lands. If Confirm is nil, writes are auto-approved.
type FSWriteTool struct {
	Confirm  ConfirmFunc
	MaxBytes int
}

// NewFSWrite returns an FSWriteTool. Pass a ConfirmFunc to gate writes;
// nil means auto-approve.
func NewFSWrite(confirm ConfirmFunc) *FSWriteTool {
	return &FSWriteTool{Confirm: confirm, MaxBytes: 1 << 20}
}

func (f *FSWriteTool) Name() string { return "fs_write" }

func (f *FSWriteTool) Description() string {
	return "Write content to a file on disk. The user will be asked to confirm " +
		"the proposed change before it is applied. Creates parent directories " +
		"if needed. Use mode 'create' to write the full file, or 'patch' to " +
		"replace a specific substring."
}

func (f *FSWriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Filesystem path to write"},
    "mode": {"type": "string", "enum": ["create", "patch"], "description": "create = write full file content; patch = replace old_str with new_str"},
    "content": {"type": "string", "description": "Full file content (mode=create)"},
    "old_str": {"type": "string", "description": "Exact substring to find (mode=patch)"},
    "new_str": {"type": "string", "description": "Replacement string (mode=patch)"}
  },
  "required": ["path", "mode"]
}`)
}

func (f *FSWriteTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Mode    string `json:"mode"`
		Content string `json:"content"`
		OldStr  string `json:"old_str"`
		NewStr  string `json:"new_str"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.Path == "" {
		return "", errors.New("path is required")
	}

	switch args.Mode {
	case "create":
		return f.runCreate(ctx, args.Path, args.Content)
	case "patch":
		return f.runPatch(ctx, args.Path, args.OldStr, args.NewStr)
	default:
		return "", fmt.Errorf("mode must be \"create\" or \"patch\", got %q", args.Mode)
	}
}

func (f *FSWriteTool) runCreate(ctx context.Context, path, content string) (string, error) {
	if len(content) > f.maxBytes() {
		return "", fmt.Errorf("content too large (%d bytes, max %d)", len(content), f.maxBytes())
	}

	prompt := fmt.Sprintf("Write %d bytes to %s?", len(content), path)
	if existing, err := os.ReadFile(path); err == nil {
		prompt = fmt.Sprintf("Overwrite %s (%d → %d bytes)?", path, len(existing), len(content))
	}

	ok, err := f.confirm(ctx, prompt)
	if err != nil {
		return "", err
	}
	if !ok {
		return "user declined the write", nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(content), path), nil
}

func (f *FSWriteTool) runPatch(ctx context.Context, path, oldStr, newStr string) (string, error) {
	if oldStr == "" {
		return "", errors.New("old_str is required for mode=patch")
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	src := string(existing)

	idx := indexOf(src, oldStr)
	if idx < 0 {
		return "", fmt.Errorf("old_str not found in %s", path)
	}

	patched := src[:idx] + newStr + src[idx+len(oldStr):]
	if len(patched) > f.maxBytes() {
		return "", fmt.Errorf("patched file too large (%d bytes, max %d)", len(patched), f.maxBytes())
	}

	prompt := fmt.Sprintf("Patch %s: replace %d chars with %d chars?", path, len(oldStr), len(newStr))
	ok, err := f.confirm(ctx, prompt)
	if err != nil {
		return "", err
	}
	if !ok {
		return "user declined the patch", nil
	}

	if err := os.WriteFile(path, []byte(patched), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("patched %s (%d → %d bytes)", path, len(existing), len(patched)), nil
}

func (f *FSWriteTool) confirm(ctx context.Context, prompt string) (bool, error) {
	if f.Confirm == nil {
		return true, nil
	}
	return f.Confirm(ctx, prompt)
}

func (f *FSWriteTool) maxBytes() int {
	if f.MaxBytes <= 0 {
		return 1 << 20
	}
	return f.MaxBytes
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
