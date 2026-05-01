package config

import (
	"errors"
	"strings"
	"testing"
)

func TestParseAgent(t *testing.T) {
	src := `---
name: hello
type: agent
model: anthropic/claude-sonnet-4-6
tools: [shell, fs.read]
---
You are a helpful agent.
`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	spec, ok := doc.Spec.(*AgentSpec)
	if !ok {
		t.Fatalf("got spec %T, want *AgentSpec", doc.Spec)
	}
	if spec.Name != "hello" || spec.Model != "anthropic/claude-sonnet-4-6" {
		t.Errorf("unexpected spec: %+v", spec)
	}
	if len(spec.Tools) != 2 || spec.Tools[0] != "shell" {
		t.Errorf("tools = %v", spec.Tools)
	}
	if !strings.Contains(doc.Body, "helpful agent") {
		t.Errorf("body missing prompt: %q", doc.Body)
	}
}

func TestParseRejectsUnknownField(t *testing.T) {
	// `runtime` is a tool field, not an agent field — strict mode catches it.
	src := `---
name: oops
type: agent
model: anthropic/claude-sonnet-4-6
runtime: builtin
---
`
	_, err := Parse([]byte(src))
	if err == nil {
		t.Fatal("expected error for unknown field 'runtime' on agent")
	}
}

func TestParseMissingFrontmatter(t *testing.T) {
	_, err := Parse([]byte("just a markdown body, no fences"))
	if !errors.Is(err, ErrMissingFrontmatter) {
		t.Errorf("got %v, want ErrMissingFrontmatter", err)
	}
}

func TestParseUnknownType(t *testing.T) {
	src := `---
name: x
type: unicorn
---
`
	_, err := Parse([]byte(src))
	if !errors.Is(err, ErrUnknownType) {
		t.Errorf("got %v, want ErrUnknownType", err)
	}
}

func TestParseMCPServer(t *testing.T) {
	src := `---
name: jira
type: mcp_server
transport: stdio
command: ["mcp-jira"]
tool_prefix: jira
---
Jira MCP server.
`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	spec, ok := doc.Spec.(*MCPServerSpec)
	if !ok {
		t.Fatalf("got %T", doc.Spec)
	}
	if spec.Transport != TransportStdio || spec.Command[0] != "mcp-jira" {
		t.Errorf("unexpected: %+v", spec)
	}
}

func TestParseAgentWithResponseSchema(t *testing.T) {
	src := `---
name: spoke
type: agent
model: anthropic/claude-sonnet-4-6
response_schema:
  type: object
  properties:
    answer:
      type: string
  required: [answer]
---
You are a spoke.
`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	spec, ok := doc.Spec.(*AgentSpec)
	if !ok {
		t.Fatalf("got %T", doc.Spec)
	}
	if spec.ResponseSchema == nil {
		t.Fatal("ResponseSchema is nil")
	}
	json := spec.ResponseSchemaJSON()
	if json == nil {
		t.Fatal("ResponseSchemaJSON returned nil")
	}
	if !strings.Contains(string(json), `"answer"`) {
		t.Errorf("JSON missing answer field: %s", json)
	}
}

func TestResponseSchemaJSONNil(t *testing.T) {
	spec := &AgentSpec{}
	if got := spec.ResponseSchemaJSON(); got != nil {
		t.Errorf("expected nil, got %s", got)
	}
}
