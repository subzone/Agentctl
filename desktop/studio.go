package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/subzone/Agentctl/internal/config"
	"gopkg.in/yaml.v3"
)

// ToolForm is a form-friendly view of a shell tool MD document.
type ToolForm struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Command        []string `json:"command"`
	TimeoutSec     int      `json:"timeoutSec"`
	ParametersJSON string   `json:"parametersJson"` // JSON Schema object as string
	Body           string   `json:"body"`
}

// SkillForm is a form-friendly view of a skill MD document.
type SkillForm struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

// ParseToolForm extracts editable fields from tool MD content.
func (a *App) ParseToolForm(content string) (ToolForm, error) {
	doc, err := config.Parse([]byte(content))
	if err != nil {
		return ToolForm{}, fmt.Errorf("parse: %w", err)
	}
	spec, ok := doc.Spec.(*config.ToolSpec)
	if !ok {
		return ToolForm{}, errors.New("document type is not 'tool'")
	}
	params := "{}"
	if spec.Parameters != nil {
		b, err := json.MarshalIndent(spec.Parameters, "", "  ")
		if err != nil {
			return ToolForm{}, fmt.Errorf("parameters: %w", err)
		}
		params = string(b)
	}
	return ToolForm{
		Name:           spec.Name,
		Description:    spec.Description,
		Command:        append([]string(nil), spec.Command...),
		TimeoutSec:     spec.TimeoutSec,
		ParametersJSON: params,
		Body:           doc.Body,
	}, nil
}

// ComposeToolForm builds tool MD from form fields.
func (a *App) ComposeToolForm(form ToolForm) (string, error) {
	var params any
	if strings.TrimSpace(form.ParametersJSON) == "" {
		params = map[string]any{"type": "object", "properties": map[string]any{}}
	} else if !json.Valid([]byte(form.ParametersJSON)) {
		return "", errors.New("parameters JSON is invalid")
	} else if err := json.Unmarshal([]byte(form.ParametersJSON), &params); err != nil {
		return "", fmt.Errorf("parameters: %w", err)
	}
	spec := &config.ToolSpec{
		Meta: config.Meta{
			Name:        strings.TrimSpace(form.Name),
			Type:        config.TypeTool,
			Description: strings.TrimSpace(form.Description),
		},
		Runtime:    config.RuntimeShell,
		Command:    trimCommand(form.Command),
		Parameters: params,
		TimeoutSec: form.TimeoutSec,
	}
	if spec.Name == "" {
		return "", errors.New("name is required")
	}
	return composeYAML(spec, strings.TrimSpace(form.Body)), nil
}

// ParseSkillForm extracts editable fields from skill MD content.
func (a *App) ParseSkillForm(content string) (SkillForm, error) {
	doc, err := config.Parse([]byte(content))
	if err != nil {
		return SkillForm{}, fmt.Errorf("parse: %w", err)
	}
	spec, ok := doc.Spec.(*config.SkillSpec)
	if !ok {
		return SkillForm{}, errors.New("document type is not 'skill'")
	}
	return SkillForm{
		Name:        spec.Name,
		Description: spec.Description,
		Body:        doc.Body,
	}, nil
}

// ComposeSkillForm builds skill MD from form fields.
func (a *App) ComposeSkillForm(form SkillForm) (string, error) {
	name := strings.TrimSpace(form.Name)
	if name == "" {
		return "", errors.New("name is required")
	}
	spec := &config.SkillSpec{
		Meta: config.Meta{
			Name:        name,
			Type:        config.TypeSkill,
			Description: strings.TrimSpace(form.Description),
		},
	}
	return composeYAML(spec, strings.TrimSpace(form.Body)), nil
}

func composeYAML(spec any, body string) string {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	_ = enc.Encode(spec)
	_ = enc.Close()
	buf.WriteString("---\n")
	if body != "" {
		buf.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			buf.WriteByte('\n')
		}
	}
	return buf.String()
}

func trimCommand(cmd []string) []string {
	out := make([]string, 0, len(cmd))
	for _, c := range cmd {
		if strings.TrimSpace(c) != "" {
			out = append(out, c)
		}
	}
	return out
}
