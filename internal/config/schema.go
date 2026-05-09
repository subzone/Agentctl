package config

import "encoding/json"

// DocType identifies the kind of MD document.
type DocType string

const (
	TypeAgent     DocType = "agent"
	TypeSkill     DocType = "skill"
	TypeTool      DocType = "tool"
	TypeMCPServer DocType = "mcp_server"
)

// Meta is the common frontmatter present on every document.
type Meta struct {
	Name        string  `yaml:"name"`
	Type        DocType `yaml:"type"`
	Description string  `yaml:"description,omitempty"`
	Version     int     `yaml:"version,omitempty"`
}

// AgentSpec describes a runnable agent.
type AgentSpec struct {
	Meta           `yaml:",inline"`
	Model          string   `yaml:"model"`
	FallbackModels []string `yaml:"fallback,omitempty"`
	Tools          []string `yaml:"tools,omitempty"`
	MCP            []string `yaml:"mcp,omitempty"`
	Skills         []string `yaml:"skills,omitempty"`
	Subagents      []string `yaml:"subagents,omitempty"`
	Powers         []string `yaml:"powers,omitempty"`
	Temperature    *float64 `yaml:"temperature,omitempty"`
	MaxTokens      *int     `yaml:"max_tokens,omitempty"`
	ThinkingPhrases []string `yaml:"thinking_phrases,omitempty"`
	PIIGuard        string   `yaml:"pii_guard,omitempty"`
	ResponseSchema  any      `yaml:"response_schema,omitempty"`
}

// ResponseSchemaJSON returns the response_schema as JSON bytes suitable
// for passing to providers. Returns nil if no schema is set.
func (a *AgentSpec) ResponseSchemaJSON() json.RawMessage {
	if a.ResponseSchema == nil {
		return nil
	}
	b, err := json.Marshal(a.ResponseSchema)
	if err != nil {
		return nil
	}
	return b
}

// SkillSpec describes a reusable instruction block composed into an agent.
type SkillSpec struct {
	Meta  `yaml:",inline"`
	Tools []string `yaml:"tools,omitempty"`
}

// ToolRuntime identifies how a tool is executed.
type ToolRuntime string

const (
	RuntimeBuiltin ToolRuntime = "builtin"
	RuntimeMCP     ToolRuntime = "mcp"
	RuntimeShell   ToolRuntime = "shell"
)

// ToolSpec describes a tool definition (builtin pointer, MCP reference,
// or external shell command).
type ToolSpec struct {
	Meta    `yaml:",inline"`
	Runtime ToolRuntime       `yaml:"runtime"`
	Command []string          `yaml:"command,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
}

// MCPTransport identifies how the MCP client connects to the server.
type MCPTransport string

const (
	TransportStdio MCPTransport = "stdio"
	TransportSSE   MCPTransport = "sse"
	TransportHTTP  MCPTransport = "http"
)

// MCPServerSpec describes an MCP server the agent can connect to.
type MCPServerSpec struct {
	Meta          `yaml:",inline"`
	Transport     MCPTransport      `yaml:"transport"`
	Command       []string          `yaml:"command,omitempty"`
	URL           string            `yaml:"url,omitempty"`
	Env           map[string]string `yaml:"env,omitempty"`
	ToolPrefix    string            `yaml:"tool_prefix,omitempty"`
	AllowedAgents []string          `yaml:"allowed_agents,omitempty"`
	Install       *MCPInstall       `yaml:"install,omitempty"`
}

// MCPInstall describes how to install the MCP server binary.
type MCPInstall struct {
	Pip  string `yaml:"pip,omitempty"`
	Npm  string `yaml:"npm,omitempty"`
	Brew string `yaml:"brew,omitempty"`
}

// Document is the parsed result of a single MD file.
// Spec holds a pointer to the typed spec matching Meta.Type.
type Document struct {
	Path string
	Body string
	Spec any
}

// Meta returns the embedded Meta from the document's typed spec.
func (d *Document) Meta() Meta {
	switch s := d.Spec.(type) {
	case *AgentSpec:
		return s.Meta
	case *SkillSpec:
		return s.Meta
	case *ToolSpec:
		return s.Meta
	case *MCPServerSpec:
		return s.Meta
	default:
		return Meta{}
	}
}
