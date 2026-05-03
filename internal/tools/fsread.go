package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// FSReadTool reads a UTF-8 text file from disk, capped at MaxBytes. The
// path is resolved against the agent process's working directory; no
// sandboxing is performed.
type FSReadTool struct {
	MaxBytes int64
}

// NewFSRead returns an FSReadTool with reasonable defaults.
func NewFSRead() *FSReadTool {
	return &FSReadTool{MaxBytes: 256 * 1024}
}

// Name implements Tool. Underscore (not dot) keeps the name within the
// `^[a-zA-Z0-9_-]{1,64}$` regex that Anthropic and OpenAI enforce on
// tool names sent in their `tools` arrays.
func (f *FSReadTool) Name() string { return "fs_read" }

// Description implements Tool.
func (f *FSReadTool) Description() string {
	return "Read a UTF-8 text file from disk and return its contents. " +
		"Output is truncated past a size limit."
}

// InputSchema implements Tool.
func (f *FSReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Filesystem path to read"}
  },
  "required": ["path"]
}`)
}

// Run implements Tool.
func (f *FSReadTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.Path == "" {
		return "", errors.New("path is required")
	}

	file, err := os.Open(args.Path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Check for binary file by reading first 512 bytes.
	probe := make([]byte, 512)
	pn, _ := file.Read(probe)
	for _, b := range probe[:pn] {
		if b == 0 {
			return "", fmt.Errorf("%s is a binary file (not UTF-8 text)", args.Path)
		}
	}
	// Seek back to start after probe.
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}

	max := f.MaxBytes
	if max <= 0 {
		max = 64 * 1024 // 64KB default — keeps context manageable
	}

	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(file, max))
	if err != nil {
		return "", err
	}

	out := buf.String()
	if n == max {
		var probe [1]byte
		if m, _ := file.Read(probe[:]); m > 0 {
			out += "\n[output truncated]"
		}
	}
	return out, nil
}
