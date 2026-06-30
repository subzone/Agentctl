package desktop

import (
	"os/exec"
	"strconv"

	"github.com/subzone/Agentctl/internal/userconfig"
)

// HealthCheck is one row in the desktop health panel (m doctor for the GUI).
type HealthCheck struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"` // ok, warn, error
	Detail string `json:"detail"`
	FixTab string `json:"fixTab,omitempty"` // rail tab to open: settings, knowledge, mcp, tools
}

// HealthReport aggregates setup checks for the Command Center.
type HealthReport struct {
	OK     bool          `json:"ok"`
	Checks []HealthCheck `json:"checks"`
}

// GetHealth runs lightweight doctor-style checks for the desktop UI.
func (a *App) GetHealth() HealthReport {
	var checks []HealthCheck
	ok := true

	add := func(id, label, status, detail, fixTab string) {
		checks = append(checks, HealthCheck{ID: id, Label: label, Status: status, Detail: detail, FixTab: fixTab})
		if status == "error" {
			ok = false
		}
	}

	cfg, err := userconfig.Load()
	if err != nil {
		add("config", "Configuration", "error", "Not configured — open Settings to add API keys", "settings")
	} else {
		add("config", "Configuration", "ok", string(cfg.Provider)+"/"+cfg.Model, "")
	}

	// Free MoE keys — at least one unlocks the default agent.
	freeKeys := []struct {
		provider userconfig.Provider
		env      string
		label    string
	}{
		{userconfig.ProviderGroq, "GROQ_API_KEY", "Groq"},
		{userconfig.ProviderCerebras, "CEREBRAS_API_KEY", "Cerebras"},
		{userconfig.ProviderMistral, "MISTRAL_API_KEY", "Mistral"},
	}
	freeFound := 0
	for _, fk := range freeKeys {
		if k, _ := userconfig.GetAPIKey(fk.provider); k != "" {
			freeFound++
		}
	}
	switch {
	case freeFound > 0:
		add("free_moe", "Free MoE providers", "ok", "API keys found for "+strconv.Itoa(freeFound)+" free tier(s)", "")
	case cfg != nil:
		if k, _ := userconfig.GetAPIKeyWithFallback(cfg.Provider); k != "" {
			add("api_key", "API key", "ok", "Key found for "+string(cfg.Provider), "")
		} else {
			add("api_key", "API key", "error", "No API key for "+string(cfg.Provider), "settings")
		}
	default:
		add("free_moe", "Free MoE providers", "warn", "No free-tier keys — add Groq/Cerebras/Mistral in Settings", "settings")
	}

	km := a.KMHealth()
	if km.Available {
		add("km", "knowledge-master", "ok", "Reachable at 127.0.0.1:9999", "")
	} else {
		detail := "Not running — start with: km start && km serve"
		if km.Error != "" {
			detail = km.Error
		}
		add("km", "knowledge-master", "warn", detail, "knowledge")
	}

	tools := a.ListTools()
	toolErrs := 0
	for _, t := range tools {
		if t.Error != "" {
			toolErrs++
		}
	}
	switch {
	case toolErrs > 0:
		add("tools", "Custom tools", "warn", strconv.Itoa(len(tools))+" defined, "+strconv.Itoa(toolErrs)+" with parse errors", "ext-tools")
	case len(tools) > 0:
		add("tools", "Custom tools", "ok", strconv.Itoa(len(tools))+" tool(s) in ~/.config/m/tools", "ext-tools")
	default:
		add("tools", "Custom tools", "ok", "None yet (optional)", "ext-tools")
	}

	skills := a.ListSkills()
	skillErrs := 0
	for _, s := range skills {
		if s.Error != "" {
			skillErrs++
		}
	}
	switch {
	case skillErrs > 0:
		add("skills", "Skills", "warn", strconv.Itoa(len(skills))+" defined, "+strconv.Itoa(skillErrs)+" with parse errors", "ext-skills")
	case len(skills) > 0:
		add("skills", "Skills", "ok", strconv.Itoa(len(skills))+" skill(s) active on New Chat", "ext-skills")
	default:
		add("skills", "Skills", "ok", "None yet (optional)", "ext-skills")
	}

	mcps := a.ListMCP()
	mcpErrs := 0
	for _, m := range mcps {
		if m.Error != "" {
			mcpErrs++
		}
	}
	switch {
	case mcpErrs > 0:
		add("mcp", "MCP servers", "warn", strconv.Itoa(len(mcps))+" defined, "+strconv.Itoa(mcpErrs)+" with parse errors", "ext-mcp")
	case len(mcps) > 0:
		add("mcp", "MCP servers", "ok", strconv.Itoa(len(mcps))+" server(s) — open on New Chat", "ext-mcp")
	default:
		add("mcp", "MCP servers", "ok", "None yet (optional)", "ext-mcp")
	}

	if _, err := exec.LookPath("git"); err != nil {
		add("git", "git", "warn", "Not found — git tool unavailable", "")
	} else {
		add("git", "git", "ok", "Found on PATH", "")
	}

	return HealthReport{OK: ok, Checks: checks}
}
