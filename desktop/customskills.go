package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/subzone/Agentctl/internal/config"
)

// SkillDoc is a user-authored skill MD file surfaced to the Skills tab.
type SkillDoc struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tools       []string `json:"tools"`
	Path        string   `json:"path"`
	Content     string   `json:"content"`
	Error       string   `json:"error,omitempty"`
}

// userSkillsDir is where user-authored skill MD files live. Each skill's body
// is composed into the system prompt of new sessions.
func userSkillsDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m", "skills")
}

// ListSkills returns all user-authored skill MD files in ~/.config/m/skills.
func (a *App) ListSkills() []SkillDoc {
	dir := userSkillsDir()
	out := []SkillDoc{}
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
		sd := SkillDoc{
			Name:    strings.TrimSuffix(e.Name(), ".md"),
			Path:    p,
			Content: string(data),
		}
		if doc, err := config.Parse(data); err != nil {
			sd.Error = err.Error()
		} else if spec, ok := doc.Spec.(*config.SkillSpec); ok {
			sd.Name = spec.Name
			sd.Description = spec.Description
			sd.Tools = spec.Tools
		} else {
			sd.Error = "document type is not 'skill'"
		}
		out = append(out, sd)
	}
	return out
}

// SaveSkill validates and writes a skill MD file. oldName is the file's
// previous name (empty for new skills); a rename removes the old file.
func (a *App) SaveSkill(oldName, content string) error {
	doc, err := config.Parse([]byte(content))
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	spec, ok := doc.Spec.(*config.SkillSpec)
	if !ok {
		return errors.New("frontmatter 'type' must be 'skill'")
	}
	if issues := config.Validate(doc); len(issues) > 0 {
		msgs := make([]string, 0, len(issues))
		for _, is := range issues {
			if is.Field == "name" {
				msgs = append(msgs, "name must be lowercase letters, numbers, '.', '-' or '_' (e.g. my_skill) and start with a letter or number")
				continue
			}
			msgs = append(msgs, is.Error())
		}
		return errors.New(strings.Join(msgs, "; "))
	}
	dir := userSkillsDir()
	if dir == "" {
		return errors.New("no user config directory available")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	dst := filepath.Join(dir, spec.Name+".md")
	if oldName != spec.Name {
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("a skill named %q already exists — choose a different name", spec.Name)
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

// DeleteSkill removes a user-authored skill MD file by name.
func (a *App) DeleteSkill(name string) error {
	dir := userSkillsDir()
	if dir == "" {
		return errors.New("no user config directory available")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	return os.Remove(filepath.Join(dir, name+".md"))
}

// NewSkillTemplate returns starter MD for a new skill.
func (a *App) NewSkillTemplate() string {
	return `---
name: my_skill
type: skill
description: One-line summary of what this skill teaches the agent.
tools:
  - fs_read
  - code_search
---
Write reusable instructions here. This text is added to the agent's system
prompt for every new chat, so the agent follows these guidelines automatically.
Describe the procedure, conventions, or domain knowledge it should apply.
`
}

// composeUserSkills appends every user-authored skill body to the base system
// prompt so authored skills take effect on the next New Chat. Skills with
// parse errors are skipped. Returns base unchanged when there are none.
func composeUserSkills(base string) string {
	dir := userSkillsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return base
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		doc, err := config.ParseFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		spec, ok := doc.Spec.(*config.SkillSpec)
		if !ok {
			continue
		}
		body := strings.TrimSpace(doc.Body)
		if body == "" {
			continue
		}
		fmt.Fprintf(&b, "\n\n### Skill: %s\n", spec.Name)
		if spec.Description != "" {
			fmt.Fprintf(&b, "%s\n\n", spec.Description)
		}
		b.WriteString(body)
	}
	if b.Len() == 0 {
		return base
	}
	return base + "\n\n## Skills\nApply the following skills where relevant:" + b.String()
}
