package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/subzone/Agentctl/internal/config"
)

// ToolDoc is a user-authored tool MD file surfaced to the Tools tab.
type ToolDoc struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Runtime     string `json:"runtime"`
	Path        string `json:"path"`
	Content     string `json:"content"`
	Error       string `json:"error,omitempty"` // parse error, if any
}

// ListTools returns all user-authored tool MD files in ~/.config/m/tools.
func (a *App) ListTools() []ToolDoc {
	dir := userToolsDir()
	out := []ToolDoc{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		td := ToolDoc{
			Name:    strings.TrimSuffix(e.Name(), ".md"),
			Path:    p,
			Content: string(data),
		}
		if doc, err := config.Parse(data); err != nil {
			td.Error = err.Error()
		} else if spec, ok := doc.Spec.(*config.ToolSpec); ok {
			td.Name = spec.Name
			td.Description = spec.Description
			td.Runtime = string(spec.Runtime)
		} else {
			td.Error = "document type is not 'tool'"
		}
		out = append(out, td)
	}
	return out
}

// SaveTool validates and writes a tool MD file. oldName is the file's previous
// name (empty for new tools); if the tool was renamed, the old file is removed.
// Returns a descriptive error when the content doesn't parse or validate.
func (a *App) SaveTool(oldName, content string) error {
	doc, err := config.Parse([]byte(content))
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	spec, ok := doc.Spec.(*config.ToolSpec)
	if !ok {
		return errors.New("frontmatter 'type' must be 'tool'")
	}
	if issues := config.Validate(doc); len(issues) > 0 {
		msgs := make([]string, 0, len(issues))
		for _, is := range issues {
			if is.Field == "name" {
				// The raw regex is opaque to users; explain it plainly.
				msgs = append(msgs, "name must be lowercase letters, numbers, '.', '-' or '_' (e.g. my_tool) and start with a letter or number")
				continue
			}
			msgs = append(msgs, is.Error())
		}
		return errors.New(strings.Join(msgs, "; "))
	}
	dir := userToolsDir()
	if dir == "" {
		return errors.New("no user config directory available")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	dst := filepath.Join(dir, spec.Name+".md")
	// Guard against clobbering a different tool: only allow writing over an
	// existing file when it's the one being edited (oldName == spec.Name).
	if oldName != spec.Name {
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("a tool named %q already exists — choose a different name", spec.Name)
		}
	}
	if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
		return err
	}
	if oldName != "" && oldName != spec.Name {
		_ = os.Remove(filepath.Join(dir, oldName+".md"))
	}
	return nil
}

// DeleteTool removes a user-authored tool MD file by name.
func (a *App) DeleteTool(name string) error {
	dir := userToolsDir()
	if dir == "" {
		return errors.New("no user config directory available")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	return os.Remove(filepath.Join(dir, name+".md"))
}

// NewToolTemplate returns starter MD for a new shell tool.
func (a *App) NewToolTemplate() string {
	return `---
name: my_tool
type: tool
description: One-line summary the model uses to decide when to call this tool.
runtime: shell
command:
  - bash
  - -c
  - cat
parameters:
  type: object
  properties:
    text:
      type: string
      description: Example input field; your script reads the full JSON on stdin.
  required:
    - text
timeout_sec: 30
---
Optional longer instructions for the model: describe what this tool does, its
inputs and outputs, and when to use it. The model receives the JSON arguments
on stdin and sees whatever your command prints to stdout.
`
}
