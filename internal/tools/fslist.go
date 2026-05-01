package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FSListTool lists directory contents, optionally recursive.
type FSListTool struct {
	MaxEntries int
	MaxDepth   int
}

// NewFSList returns an FSListTool with reasonable defaults.
func NewFSList() *FSListTool {
	return &FSListTool{MaxEntries: 500, MaxDepth: 5}
}

func (f *FSListTool) Name() string { return "fs_list" }

func (f *FSListTool) Description() string {
	return "List files and directories at a given path. Returns names with " +
		"trailing / for directories. Set recursive=true to walk subdirectories."
}

func (f *FSListTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Directory path to list"},
    "recursive": {"type": "boolean", "description": "Walk subdirectories (default false)"}
  },
  "required": ["path"]
}`)
}

func (f *FSListTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.Path == "" {
		return "", errors.New("path is required")
	}

	maxEntries := f.MaxEntries
	if maxEntries <= 0 {
		maxEntries = 500
	}
	maxDepth := f.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 5
	}

	if !args.Recursive {
		return f.listFlat(args.Path, maxEntries)
	}
	return f.listRecursive(args.Path, maxEntries, maxDepth)
}

func (f *FSListTool) listFlat(dir string, max int) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for i, e := range entries {
		if i >= max {
			fmt.Fprintf(&b, "... (%d more entries)\n", len(entries)-max)
			break
		}
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		b.WriteString(name)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func (f *FSListTool) listRecursive(root string, max, maxDepth int) (string, error) {
	var b strings.Builder
	count := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}
		depth := strings.Count(rel, string(filepath.Separator))
		if depth >= maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip common noise directories.
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "__pycache__", ".DS_Store":
				return filepath.SkipDir
			}
		}
		count++
		if count > max {
			return filepath.SkipAll
		}
		name := rel
		if d.IsDir() {
			name += "/"
		}
		b.WriteString(name)
		b.WriteByte('\n')
		return nil
	})
	if count > max {
		fmt.Fprintf(&b, "... (truncated at %d entries)\n", max)
	}
	return b.String(), err
}
