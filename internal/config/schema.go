package config

import "encoding/json"

// DocType identifies the kind of MD document.
type DocType string

const (
	TypeAgent     DocType = "agent"
	TypeSkill     DocType = "skill"
	TypeTool      DocType = "tool"
	TypeMCPServer DocType = "mcp_server"
	TypePackage   DocType = "package"
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
	Routing        *RoutingConfig `yaml:"routing,omitempty"`
}

// RoutingConfig defines MoE (Mixture of Experts) routing at the engine level.
// The engine classifies each user turn and overrides the model before calling
// the LLM — zero extra LLM calls for routing.
type RoutingConfig struct {
	Experts []ExpertRoute `yaml:"experts"`
}

// ExpertRoute maps a category to a model and optional system prompt override.
type ExpertRoute struct {
	Category string   `yaml:"category"`          // e.g. "reasoning", "speed", "longctx"
	Model    string   `yaml:"model"`             // provider/model
	Fallback []string `yaml:"fallback,omitempty"`
	System   string   `yaml:"system,omitempty"`  // override system prompt for this category
	MaxTokens *int    `yaml:"max_tokens,omitempty"`
	// Classification signals (any match → this expert)
	Keywords    []string `yaml:"keywords,omitempty"`     // substring match in input
	MinLength   int      `yaml:"min_length,omitempty"`   // input char count threshold
	MaxLength   int      `yaml:"max_length,omitempty"`   // upper bound for this expert
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

// PackageSpec describes a curated bundle of agents, skills, MCP servers, and
// other product entitlements.
type PackageSpec struct {
	Meta         `yaml:",inline"`
	Agents       []string `yaml:"agents,omitempty"`
	Skills       []string `yaml:"skills,omitempty"`
	Tools        []string `yaml:"tools,omitempty"`
	MCP          []string `yaml:"mcp,omitempty"`
	Entitlements []string `yaml:"entitlements,omitempty"`
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
	// Parameters is a JSON-Schema "object" describing the tool's inputs,
	// advertised to the model. The model-provided JSON is delivered to a
	// shell tool on stdin. Optional; omit for tools that take no input.
	Parameters any `yaml:"parameters,omitempty"`
	// TimeoutSec bounds a shell tool's execution. Zero uses a default.
	TimeoutSec int `yaml:"timeout_sec,omitempty"`
}

// ParametersJSON returns the parameters as a JSON-Schema object suitable for
// advertising to the model. Falls back to an empty object schema when unset.
func (t *ToolSpec) ParametersJSON() json.RawMessage {
	if t.Parameters == nil {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	b, err := json.Marshal(t.Parameters)
	if err != nil {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return b
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
	case *PackageSpec:
		return s.Meta
	default:
		return Meta{}
	}
}
