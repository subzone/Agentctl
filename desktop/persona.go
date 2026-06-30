package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Persona is a per-agent behaviour overlay. It is stored in user config keyed
// by agent name and composed into the system prompt at session start, so users
// can tweak how any agent behaves — including bundled ones like "m" — without
// editing the agent's MD file.
type Persona struct {
	Instructions string   `json:"instructions"`         // free-form custom guidance
	Tone         string   `json:"tone"`                 // "", concise, friendly, formal, playful, direct
	Verbosity    string   `json:"verbosity"`            // "", terse, balanced, detailed
	Temperature  *float64 `json:"temperature,omitempty"` // nil = use the agent's default
}

var toneText = map[string]string{
	"concise":  "Be concise and get to the point; avoid filler and preamble.",
	"friendly": "Be warm, friendly, and encouraging in tone.",
	"formal":   "Maintain a formal, professional tone.",
	"playful":  "Be playful and use light humour where it fits.",
	"direct":   "Be blunt and direct; skip hedging and caveats.",
}

var verbosityText = map[string]string{
	"terse":    "Keep answers short — prefer the minimal useful response.",
	"balanced": "Provide a balanced amount of detail.",
	"detailed": "Be thorough: explain your reasoning and include relevant context.",
}

func personasDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m", "personas")
}

// personaFileName maps an agent name to a safe JSON filename.
func personaFileName(agent string) string {
	var b strings.Builder
	for _, r := range agent {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	name := b.String()
	if name == "" {
		name = "_"
	}
	return name + ".json"
}

// loadPersona reads the stored persona for an agent, returning a zero Persona
// when none is set or it can't be read.
func loadPersona(agent string) Persona {
	dir := personasDir()
	if dir == "" {
		return Persona{}
	}
	data, err := os.ReadFile(filepath.Join(dir, personaFileName(agent)))
	if err != nil {
		return Persona{}
	}
	var p Persona
	if err := json.Unmarshal(data, &p); err != nil {
		return Persona{}
	}
	return p
}

// IsEmpty reports whether the persona has no effect on a session.
func (p Persona) IsEmpty() bool {
	return strings.TrimSpace(p.Instructions) == "" && p.Tone == "" && p.Verbosity == "" && p.Temperature == nil
}

// compose renders the persona as a system-prompt block, or "" if it has no
// textual guidance. Temperature is applied separately on the engine config.
func (p Persona) compose() string {
	var lines []string
	if t, ok := toneText[p.Tone]; ok {
		lines = append(lines, "- "+t)
	}
	if v, ok := verbosityText[p.Verbosity]; ok {
		lines = append(lines, "- "+v)
	}
	instr := strings.TrimSpace(p.Instructions)
	if len(lines) == 0 && instr == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Personality & Behaviour\n")
	b.WriteString("Adopt the following persona for this conversation:")
	for _, l := range lines {
		b.WriteString("\n")
		b.WriteString(l)
	}
	if instr != "" {
		b.WriteString("\n\n")
		b.WriteString(instr)
	}
	return b.String()
}

// GetPersona returns the stored persona for an agent (zero value if unset).
func (a *App) GetPersona(agent string) Persona {
	return loadPersona(agent)
}

// SavePersona persists the persona for an agent. An empty persona deletes the
// stored file so the agent reverts to its default behaviour.
func (a *App) SavePersona(agent string, p Persona) error {
	if strings.TrimSpace(agent) == "" {
		return errors.New("agent is required")
	}
	dir := personasDir()
	if dir == "" {
		return errors.New("no user config directory available")
	}
	dst := filepath.Join(dir, personaFileName(agent))
	if p.IsEmpty() {
		err := os.Remove(dst)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
