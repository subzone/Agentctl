package config

import (
	"fmt"
	"regexp"
	"strings"
)

// nameRE constrains identifiers to a safe, filesystem- and DNS-friendly form.
var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// modelRE expects "provider/model" with non-empty halves.
var modelRE = regexp.MustCompile(`^[a-z0-9_-]+/[A-Za-z0-9._:-]+$`)

// Issue is a single validation problem found in a Document.
type Issue struct {
	Path    string
	Field   string
	Message string
}

func (i Issue) Error() string {
	if i.Field != "" {
		return fmt.Sprintf("%s [%s]: %s", i.Path, i.Field, i.Message)
	}
	return fmt.Sprintf("%s: %s", i.Path, i.Message)
}

// Validate checks a single document and returns all issues found.
func Validate(doc *Document) []Issue {
	if doc == nil {
		return []Issue{{Message: "nil document"}}
	}
	var issues []Issue
	add := func(field, msg string) {
		issues = append(issues, Issue{Path: doc.Path, Field: field, Message: msg})
	}

	meta := doc.Meta()
	if meta.Name == "" {
		add("name", "is required")
	} else if !nameRE.MatchString(meta.Name) {
		add("name", "must match "+nameRE.String())
	}

	switch s := doc.Spec.(type) {
	case *AgentSpec:
		validateAgent(s, add)
	case *SkillSpec:
		// no extra required fields beyond Meta
	case *ToolSpec:
		validateTool(s, add)
	case *MCPServerSpec:
		validateMCP(s, add)
	case *PackageSpec:
		validatePackage(s, add)
	default:
		add("type", "unrecognized spec")
	}

	return issues
}

func validateAgent(s *AgentSpec, add func(field, msg string)) {
	if s.Model == "" {
		add("model", "is required (e.g. anthropic/claude-sonnet-4-6)")
	} else if !modelRE.MatchString(s.Model) {
		add("model", "must be in form provider/model")
	}
	if s.Temperature != nil {
		if *s.Temperature < 0 || *s.Temperature > 2 {
			add("temperature", "must be between 0 and 2")
		}
	}
	if s.MaxTokens != nil && *s.MaxTokens <= 0 {
		add("max_tokens", "must be positive")
	}
	for _, t := range s.Tools {
		if strings.TrimSpace(t) == "" {
			add("tools", "contains empty entry")
		}
	}
	if s.Routing != nil {
		if len(s.Routing.Experts) == 0 {
			add("routing.experts", "must have at least one expert")
		}
		for i, e := range s.Routing.Experts {
			if e.Category == "" {
				add(fmt.Sprintf("routing.experts[%d].category", i), "is required")
			}
			if e.Model == "" {
				add(fmt.Sprintf("routing.experts[%d].model", i), "is required")
			} else if !modelRE.MatchString(e.Model) {
				add(fmt.Sprintf("routing.experts[%d].model", i), "must be in form provider/model")
			}
		}
	}
}

func validateTool(s *ToolSpec, add func(field, msg string)) {
	switch s.Runtime {
	case "":
		add("runtime", "is required (builtin | mcp | shell)")
	case RuntimeBuiltin, RuntimeMCP:
		// ok
	case RuntimeShell:
		if len(s.Command) == 0 {
			add("command", "is required when runtime=shell")
		}
	default:
		add("runtime", fmt.Sprintf("unknown value %q", s.Runtime))
	}
}

func validateMCP(s *MCPServerSpec, add func(field, msg string)) {
	switch s.Transport {
	case "":
		add("transport", "is required (stdio | sse | http)")
	case TransportStdio:
		if len(s.Command) == 0 {
			add("command", "is required when transport=stdio")
		}
	case TransportSSE, TransportHTTP:
		if s.URL == "" {
			add("url", fmt.Sprintf("is required when transport=%s", s.Transport))
		}
	default:
		add("transport", fmt.Sprintf("unknown value %q", s.Transport))
	}
}

func validatePackage(s *PackageSpec, add func(field, msg string)) {
	hasContents := false
	for _, ref := range s.Agents {
		if strings.TrimSpace(ref) == "" {
			add("agents", "contains empty entry")
			continue
		}
		hasContents = true
	}
	for _, ref := range s.Skills {
		if strings.TrimSpace(ref) == "" {
			add("skills", "contains empty entry")
			continue
		}
		hasContents = true
	}
	for _, ref := range s.Tools {
		if strings.TrimSpace(ref) == "" {
			add("tools", "contains empty entry")
			continue
		}
		hasContents = true
	}
	for _, ref := range s.MCP {
		if strings.TrimSpace(ref) == "" {
			add("mcp", "contains empty entry")
			continue
		}
		hasContents = true
	}
	for _, ref := range s.Entitlements {
		if strings.TrimSpace(ref) == "" {
			add("entitlements", "contains empty entry")
			continue
		}
	}
	if !hasContents {
		add("package", "must include at least one agent, skill, tool, or mcp server")
	}
}

// ValidateDocs validates a collection of documents and additionally checks
// cross-references: agent.tools / agent.mcp / agent.skills / agent.subagents
// must resolve to known documents in the same set, with the right type.
func ValidateDocs(docs []*Document) []Issue {
	var issues []Issue

	// Index by name+type for cross-ref lookups.
	byName := make(map[string]*Document, len(docs))
	dupes := make(map[string]int)
	for _, d := range docs {
		issues = append(issues, Validate(d)...)
		name := d.Meta().Name
		if name == "" {
			continue
		}
		key := string(d.Meta().Type) + "/" + name
		if existing, ok := byName[key]; ok {
			dupes[key]++
			issues = append(issues, Issue{
				Path:    d.Path,
				Field:   "name",
				Message: fmt.Sprintf("duplicate %s %q (also at %s)", d.Meta().Type, name, existing.Path),
			})
			continue
		}
		byName[key] = d
	}

	for _, d := range docs {
		agent, ok := d.Spec.(*AgentSpec)
		if !ok {
			continue
		}
		for _, ref := range agent.Skills {
			if _, ok := byName["skill/"+ref]; !ok {
				issues = append(issues, Issue{
					Path: d.Path, Field: "skills",
					Message: fmt.Sprintf("references unknown skill %q", ref),
				})
			}
		}
		for _, ref := range agent.MCP {
			if _, ok := byName["mcp_server/"+ref]; !ok {
				issues = append(issues, Issue{
					Path: d.Path, Field: "mcp",
					Message: fmt.Sprintf("references unknown mcp_server %q", ref),
				})
			}
		}
		for _, ref := range agent.Subagents {
			if _, ok := byName["agent/"+ref]; !ok {
				issues = append(issues, Issue{
					Path: d.Path, Field: "subagents",
					Message: fmt.Sprintf("references unknown agent %q", ref),
				})
			}
		}
	}

	for _, d := range docs {
		pkg, ok := d.Spec.(*PackageSpec)
		if !ok {
			continue
		}
		for _, ref := range pkg.Agents {
			if _, ok := byName["agent/"+ref]; !ok {
				issues = append(issues, Issue{
					Path: d.Path, Field: "agents",
					Message: fmt.Sprintf("references unknown agent %q", ref),
				})
			}
		}
		for _, ref := range pkg.Skills {
			if _, ok := byName["skill/"+ref]; !ok {
				issues = append(issues, Issue{
					Path: d.Path, Field: "skills",
					Message: fmt.Sprintf("references unknown skill %q", ref),
				})
			}
		}
		for _, ref := range pkg.MCP {
			if _, ok := byName["mcp_server/"+ref]; !ok {
				issues = append(issues, Issue{
					Path: d.Path, Field: "mcp",
					Message: fmt.Sprintf("references unknown mcp_server %q", ref),
				})
			}
		}
	}

	return issues
}
