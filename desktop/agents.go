package desktop

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/subzone/Agentctl/examples"
	"github.com/subzone/Agentctl/internal/config"
)

// AgentDoc is an agent MD file for the Studio UI.
type AgentDoc struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Model       string `json:"model"`
	Path        string `json:"path"`
	Content     string `json:"content"`
	Builtin     bool   `json:"builtin"`
	Error       string `json:"error,omitempty"`
}

// ListAgentDocs returns bundled and user agents with full MD content.
func (a *App) ListAgentDocs() []AgentDoc {
	var out []AgentDoc
	seen := map[string]bool{}

	_ = fs.WalkDir(examples.Agents, "agents", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		data, _ := examples.Agents.ReadFile(path)
		if data == nil {
			return nil
		}
		ad := agentDocFromBytes(path, data, true)
		if ad.Name == "" {
			return nil
		}
		seen[ad.Name] = true
		out = append(out, ad)
		return nil
	})

	dir := userAgentsDir()
	if dir != "" {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			p := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			ad := agentDocFromBytes(p, data, false)
			if ad.Name == "" || seen[ad.Name] {
				continue
			}
			out = append(out, ad)
		}
	}
	return out
}

func agentDocFromBytes(path string, data []byte, builtin bool) AgentDoc {
	ad := AgentDoc{Path: path, Content: string(data), Builtin: builtin}
	doc, err := config.Parse(data)
	if err != nil {
		ad.Error = err.Error()
		ad.Name = strings.TrimSuffix(filepath.Base(path), ".md")
		return ad
	}
	spec, ok := doc.Spec.(*config.AgentSpec)
	if !ok {
		ad.Error = "document type is not 'agent'"
		return ad
	}
	ad.Name = spec.Name
	ad.Description = spec.Description
	ad.Model = spec.Model
	return ad
}

// SaveAgent validates and writes a user agent MD file.
func (a *App) SaveAgent(oldName, content string) error {
	doc, err := config.Parse([]byte(content))
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	spec, ok := doc.Spec.(*config.AgentSpec)
	if !ok {
		return errors.New("frontmatter 'type' must be 'agent'")
	}
	if issues := config.Validate(doc); len(issues) > 0 {
		msgs := make([]string, 0, len(issues))
		for _, is := range issues {
			msgs = append(msgs, is.Error())
		}
		return errors.New(strings.Join(msgs, "; "))
	}
	dir := userAgentsDir()
	if dir == "" {
		return errors.New("no user config directory available")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	dst := filepath.Join(dir, spec.Name+".md")
	if oldName != spec.Name {
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("an agent named %q already exists — choose a different name", spec.Name)
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

// DeleteAgent removes a user-authored agent (builtins cannot be deleted).
func (a *App) DeleteAgent(name string) error {
	dir := userAgentsDir()
	if dir == "" {
		return errors.New("no user config directory available")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	p := filepath.Join(dir, name+".md")
	if _, err := os.Stat(p); err != nil {
		return fmt.Errorf("user agent %q not found", name)
	}
	return os.Remove(p)
}

// NewAgentTemplate returns starter MD for a custom agent.
func (a *App) NewAgentTemplate() string {
	return `---
name: my_agent
type: agent
description: One-line summary of what this agent does.
model: groq/llama-3.3-70b-versatile
fallback:
  - gemini/gemini-2.5-flash
tools:
  - shell
  - fs_read
  - fs_write
  - fs_list
  - git
skills: []
mcp: []
temperature: 0.3
---
You are a helpful agent. Describe your role, constraints, and how you should
approach tasks. This body becomes the core of your system prompt.
`
}

// BuiltinToolNames lists built-in tools available for agent Studio pickers.
func (a *App) BuiltinToolNames() []string {
	return []string{
		"shell", "fs_read", "fs_write", "fs_write_multi", "fs_list", "fs_delete",
		"git", "test_run", "code_search", "web_fetch", "knowledge",
	}
}
