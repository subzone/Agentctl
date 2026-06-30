package desktop

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/subzone/Agentctl/internal/config"
)

// AgentContextPreview is what the Context Inspector shows: the composed system
// prompt and metadata about overlays (skills, persona, MCP) for an agent.
type AgentContextPreview struct {
	Agent      string   `json:"agent"`
	Model      string   `json:"model"`
	System     string   `json:"system"`
	CharCount  int      `json:"charCount"`
	Skills     []string `json:"skills"`
	Persona    Persona  `json:"persona"`
	ToolCount  int      `json:"toolCount"`
	MCPServers []string `json:"mcpServers"`
}

// PreviewContext returns the system prompt that would be used on the next New
// Chat with this agent (agent body + skills + persona).
func (a *App) PreviewContext(agentName string) (*AgentContextPreview, error) {
	spec, body, err := a.resolveAgent(agentName)
	if err != nil {
		return nil, err
	}
	system := composeAgentSystem(body, agentName)

	model := spec.Model
	if model == "" && a.cfg != nil {
		model = string(a.cfg.Provider) + "/" + a.cfg.Model
	}

	toolCount := len(kmToolNames())
	for _, t := range a.ListTools() {
		if t.Error == "" {
			toolCount++
		}
	}
	// Builtins (fs_*, shell, git, etc.) are always available; not counted here.

	var mcpNames []string
	for _, m := range a.ListMCP() {
		if m.Error == "" {
			mcpNames = append(mcpNames, m.Name)
		}
	}

	return &AgentContextPreview{
		Agent:      agentName,
		Model:      model,
		System:     system,
		CharCount:  len(system),
		Skills:     listSkillNames(),
		Persona:    loadPersona(agentName),
		ToolCount:  toolCount,
		MCPServers: mcpNames,
	}, nil
}

// composeAgentSystem builds the full system string (shared by CreateSession,
// SwitchAgent, and PreviewContext).
func composeAgentSystem(agentBody, agentName string) string {
	system := composeUserSkills(agentBody)
	if blk := loadPersona(agentName).compose(); blk != "" {
		system += "\n\n" + blk
	}
	return system
}

func listSkillNames() []string {
	dir := userSkillsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		doc, err := config.ParseFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		spec, ok := doc.Spec.(*config.SkillSpec)
		if !ok || strings.TrimSpace(doc.Body) == "" {
			continue
		}
		names = append(names, spec.Name)
	}
	return names
}

func kmToolNames() []string {
	return []string{"km_search", "km_index", "km_blast_radius", "km_status"}
}
