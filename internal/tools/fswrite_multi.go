package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FSWriteMultiTool applies multiple file writes atomically. If any write
// fails, all previously applied writes in the batch are rolled back.
type FSWriteMultiTool struct {
	confirm ConfirmFunc
	undo    *UndoStack
}

func NewFSWriteMulti(confirm ConfirmFunc, undo *UndoStack) *FSWriteMultiTool {
	return &FSWriteMultiTool{confirm: confirm, undo: undo}
}

func (f *FSWriteMultiTool) Name() string { return "fs_write_multi" }

func (f *FSWriteMultiTool) Description() string {
	return "Apply multiple file writes atomically. All succeed or all are rolled back."
}

func (f *FSWriteMultiTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"files": {
				"type": "array",
				"description": "List of file operations to apply atomically",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "File path"},
						"mode": {"type": "string", "enum": ["create", "patch"], "description": "create or patch"},
						"content": {"type": "string", "description": "Full content for create mode"},
						"old_str": {"type": "string", "description": "String to find for patch mode"},
						"new_str": {"type": "string", "description": "Replacement string for patch mode"}
					},
					"required": ["path", "mode"]
				}
			}
		},
		"required": ["files"]
	}`)
}

type multiFileOp struct {
	Path    string `json:"path"`
	Mode    string `json:"mode"`
	Content string `json:"content"`
	OldStr  string `json:"old_str"`
	NewStr  string `json:"new_str"`
}

type multiInput struct {
	Files []multiFileOp `json:"files"`
}

func (f *FSWriteMultiTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var in multiInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if len(in.Files) == 0 {
		return "", fmt.Errorf("invalid input: files array is empty")
	}

	// Build summary for confirmation.
	var summary []string
	for _, op := range in.Files {
		summary = append(summary, fmt.Sprintf("  %s %s", op.Mode, op.Path))
	}
	prompt := fmt.Sprintf("Atomic write (%d files):\n%s", len(in.Files), strings.Join(summary, "\n"))
	if f.confirm != nil {
		ok, err := f.confirm(ctx, prompt)
		if err != nil {
			return "", err
		}
		if !ok {
			return "user declined the write", nil
		}
	}

	// Push undo entries before applying (Push reads current file state).
	if f.undo != nil {
		for _, op := range in.Files {
			f.undo.Push(op.Path)
		}
	}

	// Collect backups for rollback.
	type backup struct {
		path    string
		existed bool
		data    []byte
		perm    os.FileMode
	}
	var backups []backup

	rollback := func() {
		for i := len(backups) - 1; i >= 0; i-- {
			b := backups[i]
			if !b.existed {
				os.Remove(b.path)
			} else {
				os.WriteFile(b.path, b.data, b.perm)
			}
		}
	}

	// Apply all operations.
	for _, op := range in.Files {
		// Backup current state.
		var b backup
		b.path = op.Path
		if info, err := os.Stat(op.Path); err == nil {
			b.existed = true
			b.perm = info.Mode().Perm()
			b.data, _ = os.ReadFile(op.Path)
		}
		backups = append(backups, b)

		var err error
		switch op.Mode {
		case "create":
			err = f.applyCreate(op.Path, op.Content)
		case "patch":
			err = f.applyPatch(op.Path, op.OldStr, op.NewStr)
		default:
			err = fmt.Errorf("unknown mode %q for %s", op.Mode, op.Path)
		}
		if err != nil {
			rollback()
			return "", fmt.Errorf("atomic write failed at %s: %w (all changes rolled back)", op.Path, err)
		}
	}

	// Push undo entries.
	// (Already pushed before applying — undo stack has pre-write state.)

	return fmt.Sprintf("wrote %d files atomically", len(in.Files)), nil
}

func (f *FSWriteMultiTool) applyCreate(path, content string) error {
	path = filepath.Clean(path)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func (f *FSWriteMultiTool) applyPatch(path, oldStr, newStr string) error {
	path = filepath.Clean(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)
	if !strings.Contains(content, oldStr) {
		return fmt.Errorf("old_str not found in %s", path)
	}
	content = strings.Replace(content, oldStr, newStr, 1)
	perm := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	return os.WriteFile(path, []byte(content), perm) //nolint:gosec // path cleaned above
}
