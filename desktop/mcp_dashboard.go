package desktop

import (
	"strings"
	"time"
)

// MCPDashboardEntry is live status for one configured MCP server.
type MCPDashboardEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Transport   string   `json:"transport"`
	InSession   bool     `json:"inSession"`
	Reachable   bool     `json:"reachable"`
	ToolCount   int      `json:"toolCount"`
	Tools       []string `json:"tools,omitempty"`
	Error       string   `json:"error,omitempty"`
	ConfigError string   `json:"configError,omitempty"`
	CheckedAt   int64    `json:"checkedAt"`
}

// GetMCPDashboard returns configured MCP servers with session + probe status.
// probe=true runs a connectivity check per server (slower).
func (a *App) GetMCPDashboard(sessionID string, probe bool) []MCPDashboardEntry {
	docs := a.ListMCP()
	sessionNames := map[string]bool{}
	if sessionID != "" {
		for _, n := range a.GetSessionMCP(sessionID) {
			sessionNames[n] = true
		}
	}

	out := make([]MCPDashboardEntry, 0, len(docs))
	now := time.Now().Unix()
	for _, d := range docs {
		entry := MCPDashboardEntry{
			Name:        d.Name,
			Description: d.Description,
			Transport:   d.Transport,
			InSession:   sessionNames[d.Name],
			CheckedAt:   now,
			ConfigError: d.Error,
		}
		if probe && d.Error == "" && d.Name != "" {
			res := a.TestMCP(d.Name)
			entry.Reachable = res.OK
			entry.ToolCount = len(res.Tools)
			entry.Tools = res.Tools
			if res.Error != "" {
				entry.Error = res.Error
			}
		} else if entry.InSession {
			entry.Reachable = true
		}
		out = append(out, entry)
	}
	return out
}

// GetMCPStatus lists configured MCP servers (legacy API; use GetMCPDashboard).
func (a *App) GetMCPStatus() []MCPStatus {
	docs := a.ListMCP()
	out := make([]MCPStatus, 0, len(docs))
	for _, d := range docs {
		out = append(out, MCPStatus{
			Name:      d.Name,
			Transport: d.Transport,
			Installed: d.Error == "" && strings.TrimSpace(d.Name) != "",
		})
	}
	return out
}
