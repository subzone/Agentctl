package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/subzone/Agentctl/internal/filecontent"
)

// FSReadTool reads text files and PDFs from disk, capped at MaxBytes.
type FSReadTool struct {
	MaxBytes int64
}

// NewFSRead returns an FSReadTool with reasonable defaults.
func NewFSRead() *FSReadTool {
	return &FSReadTool{MaxBytes: 256 * 1024}
}

// Name implements Tool.
func (f *FSReadTool) Name() string { return "fs_read" }

// Description implements Tool.
func (f *FSReadTool) Description() string {
	return "Read a UTF-8 text file or PDF from disk and return extracted text. " +
		"PDFs must have a text layer (scanned image-only PDFs are not supported). " +
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
	max := f.MaxBytes
	if max <= 0 {
		max = 256 * 1024
	}
	return filecontent.Read(args.Path, max)
}
