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

// AgentForm is a form-friendly view of an agent MD document (routing stays in Advanced MD).
type AgentForm struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Model          string   `json:"model"`
	FallbackLines  string   `json:"fallbackLines"` // one model per line
	Tools          []string `json:"tools"`
	Skills         []string `json:"skills"`
	MCP            []string `json:"mcp"`
	Temperature    *float64 `json:"temperature"`
	Body           string   `json:"body"`
	HasRouting     bool     `json:"hasRouting"` // true when MoE routing block exists — edit in Advanced MD
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

// ParseAgentForm extracts editable fields from agent MD content.
func (a *App) ParseAgentForm(content string) (AgentForm, error) {
	doc, err := config.Parse([]byte(content))
	if err != nil {
		return AgentForm{}, fmt.Errorf("parse: %w", err)
	}
	spec, ok := doc.Spec.(*config.AgentSpec)
	if !ok {
		return AgentForm{}, errors.New("document type is not 'agent'")
	}
	fb := strings.Join(spec.FallbackModels, "\n")
	var temp *float64
	if spec.Temperature != nil {
		v := *spec.Temperature
		temp = &v
	}
	return AgentForm{
		Name:          spec.Name,
		Description:   spec.Description,
		Model:         spec.Model,
		FallbackLines: fb,
		Tools:         append([]string(nil), spec.Tools...),
		Skills:        append([]string(nil), spec.Skills...),
		MCP:           append([]string(nil), spec.MCP...),
		Temperature:   temp,
		Body:          doc.Body,
		HasRouting:    spec.Routing != nil && len(spec.Routing.Experts) > 0,
	}, nil
}

// ComposeAgentForm builds agent MD from form fields (does not preserve MoE routing blocks).
func (a *App) ComposeAgentForm(form AgentForm) (string, error) {
	if form.HasRouting {
		return "", errors.New("this agent has MoE routing — edit with Advanced MD to preserve it")
	}
	name := strings.TrimSpace(form.Name)
	if name == "" {
		return "", errors.New("name is required")
	}
	var fallback []string
	for _, line := range strings.Split(form.FallbackLines, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			fallback = append(fallback, t)
		}
	}
	spec := &config.AgentSpec{
		Meta: config.Meta{
			Name:        name,
			Type:        config.TypeAgent,
			Description: strings.TrimSpace(form.Description),
		},
		Model:          strings.TrimSpace(form.Model),
		FallbackModels: fallback,
		Tools:          form.Tools,
		Skills:         form.Skills,
		MCP:            form.MCP,
		Temperature:    form.Temperature,
	}
	return composeYAML(spec, strings.TrimSpace(form.Body)), nil
}

// MCPForm is a form-friendly view of an MCP server MD document.
type MCPForm struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Transport   string   `json:"transport"`
	Command     []string `json:"command"`
	URL         string   `json:"url"`
	ToolPrefix  string   `json:"toolPrefix"`
	Body        string   `json:"body"`
}

// ParseMCPForm extracts editable fields from MCP server MD content.
func (a *App) ParseMCPForm(content string) (MCPForm, error) {
	doc, err := config.Parse([]byte(content))
	if err != nil {
		return MCPForm{}, fmt.Errorf("parse: %w", err)
	}
	spec, ok := doc.Spec.(*config.MCPServerSpec)
	if !ok {
		return MCPForm{}, errors.New("document type is not 'mcp_server'")
	}
	return MCPForm{
		Name:        spec.Name,
		Description: spec.Description,
		Transport:   string(spec.Transport),
		Command:     append([]string(nil), spec.Command...),
		URL:         spec.URL,
		ToolPrefix:  spec.ToolPrefix,
		Body:        doc.Body,
	}, nil
}

// ComposeMCPForm builds MCP server MD from form fields.
func (a *App) ComposeMCPForm(form MCPForm) (string, error) {
	transport := config.MCPTransport(strings.TrimSpace(form.Transport))
	if transport == "" {
		transport = config.TransportStdio
	}
	name := strings.TrimSpace(form.Name)
	if name == "" {
		return "", errors.New("name is required")
	}
	spec := &config.MCPServerSpec{
		Meta: config.Meta{
			Name:        name,
			Type:        config.TypeMCPServer,
			Description: strings.TrimSpace(form.Description),
		},
		Transport:  transport,
		Command:    trimCommand(form.Command),
		URL:        strings.TrimSpace(form.URL),
		ToolPrefix: strings.TrimSpace(form.ToolPrefix),
	}
	return composeYAML(spec, strings.TrimSpace(form.Body)), nil
}
