package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ConfirmFunc asks the user to approve an action. Returns true if approved.
type ConfirmFunc func(ctx context.Context, prompt string) (bool, error)

// UndoStack tracks file states for rollback. Thread-safe.
type UndoStack struct {
	mu    sync.Mutex
	stack []undoEntry
}

type undoEntry struct {
	path    string
	content []byte // nil means file didn't exist (was created)
	existed bool
}

const maxUndoEntries = 20

// Push saves the current state of a file before modification.
func (u *UndoStack) Push(path string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	content, err := os.ReadFile(path)
	if err != nil {
		u.stack = append(u.stack, undoEntry{path: path, existed: false})
	} else {
		u.stack = append(u.stack, undoEntry{path: path, content: content, existed: true})
	}
	// Cap the stack to prevent unbounded growth.
	if len(u.stack) > maxUndoEntries {
		u.stack = u.stack[len(u.stack)-maxUndoEntries:]
	}
}

// Pop reverts the most recent file change. Returns a description or error.
func (u *UndoStack) Pop() (string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.stack) == 0 {
		return "", errors.New("nothing to undo")
	}
	e := u.stack[len(u.stack)-1]
	u.stack = u.stack[:len(u.stack)-1]
	if !e.existed {
		if err := os.Remove(e.path); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("undo: remove %s: %w", e.path, err)
		}
		return fmt.Sprintf("removed %s (was newly created)", e.path), nil
	}
	if err := os.WriteFile(e.path, e.content, 0o644); err != nil {
		return "", fmt.Errorf("undo: restore %s: %w", e.path, err)
	}
	return fmt.Sprintf("restored %s (%d bytes)", e.path, len(e.content)), nil
}

// Len returns the number of undoable operations.
func (u *UndoStack) Len() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.stack)
}

// FSWriteTool writes or patches a UTF-8 text file on disk with diff preview,
// user confirmation, and undo support.
type FSWriteTool struct {
	Confirm  ConfirmFunc
	Undo     *UndoStack
	MaxBytes int
}

func NewFSWrite(confirm ConfirmFunc, undo *UndoStack) *FSWriteTool {
	return &FSWriteTool{Confirm: confirm, Undo: undo, MaxBytes: 1 << 20}
}

func (f *FSWriteTool) Name() string { return "fs_write" }

func (f *FSWriteTool) Description() string {
	return "Write content to a file on disk. Shows a diff preview before " +
		"applying. The user confirms every change. Creates parent directories " +
		"if needed. Use mode 'create' for full file, 'patch' for find-and-replace. " +
		"Changes can be reverted with /undo."
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

	existing, _ := os.ReadFile(path)
	diff := unifiedDiff(path, string(existing), content)
	prompt := fmt.Sprintf("Write %s:\n%s", path, diff)

	ok, err := f.confirm(ctx, prompt)
	if err != nil {
		return "", err
	}
	if !ok {
		return "user declined the write", nil
	}

	if err := f.commitWrite(path, []byte(content)); err != nil {
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
	idx := strings.Index(src, oldStr)
	if idx < 0 {
		return "", fmt.Errorf("old_str not found in %s", path)
	}
	patched := src[:idx] + newStr + src[idx+len(oldStr):]
	if len(patched) > f.maxBytes() {
		return "", fmt.Errorf("patched file too large (%d bytes, max %d)", len(patched), f.maxBytes())
	}

	diff := unifiedDiff(path, src, patched)
	prompt := fmt.Sprintf("Patch %s:\n%s", path, diff)

	ok, err := f.confirm(ctx, prompt)
	if err != nil {
		return "", err
	}
	if !ok {
		return "user declined the patch", nil
	}

	if err := f.commitWrite(path, []byte(patched)); err != nil {
		return "", err
	}
	return fmt.Sprintf("patched %s (%d → %d bytes)", path, len(existing), len(patched)), nil
}

// commitWrite saves the current state of path to the undo stack (if any),
// creates parent directories, and writes data using a write-then-rename
// pattern so the target file is never left in a partially-written state.
// The file mode is preserved when the destination already exists; otherwise
// it defaults to 0644.
func (f *FSWriteTool) commitWrite(path string, data []byte) error {
	if f.Undo != nil {
		f.Undo.Push(path)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Determine the mode to use: preserve existing file mode, or default to 0644.
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, ".fswrite-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
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

// unifiedDiff produces a unified-diff-style preview with proper hunk markers.
// Shows multiple change regions with context, capped at reasonable size.
// This is a proper diff algorithm, not the joke that was here before.
func unifiedDiff(path, old, new string) string {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", path, path)

	// Find all differing regions (hunks)
	hunks := findHunks(oldLines, newLines, 3) // 3 lines of context

	if len(hunks) == 0 {
		b.WriteString("(no changes)\n")
		return b.String()
	}

	// Limit total output to prevent overwhelming the user
	maxTotalLines := 100
	totalLines := 0

	for _, hunk := range hunks {
		if totalLines >= maxTotalLines {
			b.WriteString("... (diff truncated, too many changes)\n")
			break
		}

		// Write hunk header: @@ -oldStart,oldCount +newStart,newCount @@
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n",
			hunk.oldStart+1, hunk.oldCount,
			hunk.newStart+1, hunk.newCount)

		// Write hunk content
		linesInHunk := 0
		maxLinesPerHunk := 40

		for i := hunk.oldStart; i < hunk.oldStart+hunk.oldCount && linesInHunk < maxLinesPerHunk; i++ {
			if i < len(oldLines) {
				fmt.Fprintf(&b, "-%s\n", oldLines[i])
				linesInHunk++
				totalLines++
			}
		}

		for i := hunk.newStart; i < hunk.newStart+hunk.newCount && linesInHunk < maxLinesPerHunk; i++ {
			if i < len(newLines) {
				fmt.Fprintf(&b, "+%s\n", newLines[i])
				linesInHunk++
				totalLines++
			}
		}

		// Show context lines after the hunk
		contextAfter := 3
		for i := hunk.newStart + hunk.newCount; i < hunk.newStart+hunk.newCount+contextAfter && i < len(newLines) && linesInHunk < maxLinesPerHunk; i++ {
			fmt.Fprintf(&b, " %s\n", newLines[i])
			linesInHunk++
			totalLines++
		}
	}

	return b.String()
}

// hunk represents a change region in a diff
type hunk struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
}

// findHunks identifies all change regions between old and new lines
// with specified context lines around each change
func findHunks(oldLines, newLines []string, context int) []hunk {
	if len(oldLines) == 0 && len(newLines) == 0 {
		return nil
	}

	// Simple LCS-based diff to find changes
	changes := findChanges(oldLines, newLines)

	if len(changes) == 0 {
		return nil
	}

	// Group changes into hunks with context
	hunks := []hunk{}
	i := 0

	for i < len(changes) {
		change := changes[i]

		// Find the extent of this hunk
		hunkStartOld := change.oldIdx - context
		if hunkStartOld < 0 {
			hunkStartOld = 0
		}
		hunkStartNew := change.newIdx - context
		if hunkStartNew < 0 {
			hunkStartNew = 0
		}

		// Find all consecutive/nearby changes
		hunkEndOld := change.oldIdx + 1
		hunkEndNew := change.newIdx + 1

		j := i + 1
		for j < len(changes) {
			nextChange := changes[j]
			// If next change is within context distance, merge into this hunk
			if nextChange.oldIdx <= hunkEndOld+context+1 && nextChange.newIdx <= hunkEndNew+context+1 {
				hunkEndOld = nextChange.oldIdx + 1
				hunkEndNew = nextChange.newIdx + 1
				j++
			} else {
				break
			}
		}

		// Add context after the hunk
		hunkEndOld += context
		if hunkEndOld > len(oldLines) {
			hunkEndOld = len(oldLines)
		}
		hunkEndNew += context
		if hunkEndNew > len(newLines) {
			hunkEndNew = len(newLines)
		}

		hunks = append(hunks, hunk{
			oldStart: hunkStartOld,
			oldCount: hunkEndOld - hunkStartOld,
			newStart: hunkStartNew,
			newCount: hunkEndNew - hunkStartNew,
		})

		i = j
	}

	return hunks
}

// change represents a single line change
type change struct {
	oldIdx int
	newIdx int
	typ    string // "delete", "insert", "replace"
}

// findChanges identifies individual line changes using LCS algorithm
func findChanges(oldLines, newLines []string) []change {
	// Build LCS table
	lcs := buildLCS(oldLines, newLines)

	// Walk back through LCS to identify changes
	changes := []change{}
	i, j := len(oldLines), len(newLines)

	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && oldLines[i-1] == newLines[j-1]:
			// Lines match - part of LCS
			i--
			j--
		case j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]):
			// Insert from new
			changes = append([]change{{oldIdx: i, newIdx: j - 1, typ: "insert"}}, changes...)
			j--
		case i > 0:
			// Delete from old
			changes = append([]change{{oldIdx: i - 1, newIdx: j, typ: "delete"}}, changes...)
			i--
		}
	}

	return changes
}

// buildLCS constructs the Longest Common Subsequence table
func buildLCS(oldLines, newLines []string) [][]int {
	m, n := len(oldLines), len(newLines)
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else {
				lcs[i][j] = max(lcs[i-1][j], lcs[i][j-1])
			}
		}
	}

	return lcs
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
