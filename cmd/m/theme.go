// Theme support for the TUI. Built-in themes ship with the binary;
// users can override with ~/.config/m/theme.yaml.
package main

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

// Theme defines all colors used by the TUI.
type Theme struct {
	Name string `yaml:"name"`

	// Text colors
	Banner    string `yaml:"banner"`
	User      string `yaml:"user"`
	Assistant string `yaml:"assistant"`
	Tool      string `yaml:"tool"`
	Error     string `yaml:"error"`
	Dim       string `yaml:"dim"`
	Prompt    string `yaml:"prompt"`

	// Box borders
	Border string `yaml:"border"`

	// Background (empty = terminal default)
	Background string `yaml:"background"`

	// Diff colors (for file change previews)
	DiffAdd    string `yaml:"diff_add,omitempty"`    // added lines background
	DiffRemove string `yaml:"diff_remove,omitempty"` // removed lines background
	DiffHeader string `yaml:"diff_header,omitempty"` // diff header (--- +++ lines)
	ConfirmBg  string `yaml:"confirm_bg,omitempty"`  // confirmation prompt background

	// Thinking phrases shown while the agent works.
	// Empty = use built-in defaults.
	ThinkingPhrases []string `yaml:"thinking_phrases,omitempty"`
}

// Styles holds the resolved lipgloss styles for a theme.
type Styles struct {
	Banner    lipgloss.Style
	User      lipgloss.Style
	Assistant lipgloss.Style
	Tool      lipgloss.Style
	Error     lipgloss.Style
	Dim       lipgloss.Style
	Prompt    lipgloss.Style
	Border    lipgloss.Border
	DiffAdd    lipgloss.Style
	DiffRemove lipgloss.Style
	DiffHeader lipgloss.Style
	Confirm    lipgloss.Style
}

// Resolve converts a Theme into usable lipgloss Styles.
func (t *Theme) Resolve() Styles {
	defaultAdd := "#002200"
	defaultRem := "#220000"
	defaultHdr := "#333333"
	defaultCfm := "#1a1a2e"
	if t.DiffAdd != "" { defaultAdd = t.DiffAdd }
	if t.DiffRemove != "" { defaultRem = t.DiffRemove }
	if t.DiffHeader != "" { defaultHdr = t.DiffHeader }
	if t.ConfirmBg != "" { defaultCfm = t.ConfirmBg }

	s := Styles{
		Banner:    lipgloss.NewStyle().Foreground(color(t.Banner)),
		User:      lipgloss.NewStyle().Bold(true).Foreground(color(t.User)),
		Assistant: lipgloss.NewStyle().Foreground(color(t.Assistant)),
		Tool:      lipgloss.NewStyle().Faint(true).Foreground(color(t.Tool)),
		Error:     lipgloss.NewStyle().Foreground(color(t.Error)),
		Dim:       lipgloss.NewStyle().Faint(true).Foreground(color(t.Dim)),
		Prompt:    lipgloss.NewStyle().Foreground(color(t.Prompt)),
		DiffAdd:    lipgloss.NewStyle().Foreground(lipgloss.Color("#00cc00")).Background(lipgloss.Color(defaultAdd)),
		DiffRemove: lipgloss.NewStyle().Foreground(lipgloss.Color("#cc0000")).Background(lipgloss.Color(defaultRem)),
		DiffHeader: lipgloss.NewStyle().Faint(true).Background(lipgloss.Color(defaultHdr)),
		Confirm:    lipgloss.NewStyle().Bold(true).Background(lipgloss.Color(defaultCfm)).Padding(0, 1),
		Border:    lipgloss.NormalBorder(),
	}
	return s
}

func color(hex string) lipgloss.TerminalColor {
	if hex == "" {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(hex)
}

// Built-in themes.
var (
	Matrix = Theme{
		Name:       "matrix",
		Banner:     "#005500",
		User:       "#00ff00",
		Assistant:  "#00cc00",
		Tool:       "#008800",
		Error:      "#ff3333",
		Dim:        "#005500",
		Prompt:     "#00ff00",
		Border:     "#006600",
		DiffAdd:    "#002200",
		DiffRemove: "#220000",
		DiffHeader: "#001100",
		ConfirmBg:  "#001a00",
	}

	Default = Theme{
		Name:      "default",
		Banner:    "",
		User:      "#5f87ff",
		Assistant: "",
		Tool:      "#d7af00",
		Error:     "#ff5f5f",
		Dim:       "",
		Prompt:    "",
		Border:    "",
	}

	Minimal = Theme{
		Name:      "minimal",
		Banner:    "",
		User:      "",
		Assistant: "",
		Tool:      "",
		Error:     "",
		Dim:       "",
		Prompt:    "",
		Border:    "",
	}

	Builtin = map[string]*Theme{
		"matrix":  &Matrix,
		"default": &Default,
		"minimal": &Minimal,
	}
)

// Load reads the user's theme file. Returns the matrix theme if no file
// exists or if parsing fails.
func Load() *Theme {
	p := filePath()
	if p == "" {
		return &Matrix
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return &Matrix
	}
	var t Theme
	if err := yaml.Unmarshal(raw, &t); err != nil {
		return &Matrix
	}
	if t.Name == "" {
		t.Name = "custom"
	}
	return &t
}

// Save writes a theme to the user's config directory.
func Save(t *Theme) error {
	p := filePath()
	if p == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	out, err := yaml.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(p, out, 0o644)
}

// ByName returns a built-in theme by name, or nil if not found.
func ByName(name string) *Theme {
	return Builtin[name]
}

func filePath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m", "theme.yaml")
}
