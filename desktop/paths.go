package desktop

import (
	"os"
	"path/filepath"
)

// ConfigPath describes one on-disk config location for the Settings panel.
type ConfigPath struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

func userConfigRoot() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m")
}

// ConfigPaths returns well-known AgentCTL config directories.
func (a *App) ConfigPaths() []ConfigPath {
	root := userConfigRoot()
	if root == "" {
		return nil
	}
	return []ConfigPath{
		{ID: "root", Label: "Config root", Path: root},
		{ID: "agents", Label: "Custom agents", Path: filepath.Join(root, "agents")},
		{ID: "tools", Label: "Custom tools", Path: userToolsDir()},
		{ID: "skills", Label: "Skills", Path: userSkillsDir()},
		{ID: "mcp", Label: "MCP servers", Path: userMCPDir()},
		{ID: "personas", Label: "Personas", Path: personasDir()},
		{ID: "sessions", Label: "Saved sessions", Path: filepath.Join(root, "sessions")},
	}
}
