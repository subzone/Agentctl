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
	ConfirmFg  string `yaml:"confirm_fg,omitempty"`  // confirmation prompt foreground (text)
	ConfirmBg  string `yaml:"confirm_bg,omitempty"`  // confirmation prompt background

	// Thinking phrases shown while the agent works.
	// Empty = use built-in defaults.
	ThinkingPhrases []string `yaml:"thinking_phrases,omitempty"`
}

// Styles holds the resolved lipgloss styles for a theme.
type Styles struct {
	Banner     lipgloss.Style
	User       lipgloss.Style
	Assistant  lipgloss.Style
	Tool       lipgloss.Style
	Error      lipgloss.Style
	Dim        lipgloss.Style
	Prompt     lipgloss.Style
	Border     lipgloss.Border
	DiffAdd    lipgloss.Style
	DiffRemove lipgloss.Style
	DiffHeader lipgloss.Style
	Confirm    lipgloss.Style
}

// Resolve converts a Theme into usable lipgloss Styles.
func (t *Theme) Resolve() Styles {
	s := Styles{
		Banner:    lipgloss.NewStyle().Foreground(color(t.Banner)),
		User:      lipgloss.NewStyle().Bold(true).Foreground(color(t.User)),
		Assistant: lipgloss.NewStyle().Foreground(color(t.Assistant)),
		Tool:      lipgloss.NewStyle().Faint(true).Foreground(color(t.Tool)),
		Error:     lipgloss.NewStyle().Foreground(color(t.Error)),
		Dim:       lipgloss.NewStyle().Faint(true).Foreground(color(t.Dim)),
		Prompt:    lipgloss.NewStyle().Foreground(color(t.Prompt)),
		Border:    lipgloss.NormalBorder(),
	}

	// Diff styles: use background if specified, otherwise just foreground colors
	if t.DiffAdd != "" {
		s.DiffAdd = lipgloss.NewStyle().Foreground(lipgloss.Color("#00cc00")).Background(lipgloss.Color(t.DiffAdd))
	} else {
		// No background - just green foreground with bold for visibility
		s.DiffAdd = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#22c55e"))
	}

	if t.DiffRemove != "" {
		s.DiffRemove = lipgloss.NewStyle().Foreground(lipgloss.Color("#cc0000")).Background(lipgloss.Color(t.DiffRemove))
	} else {
		// No background - just red foreground with bold for visibility
		s.DiffRemove = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ef4444"))
	}

	if t.DiffHeader != "" {
		s.DiffHeader = lipgloss.NewStyle().Faint(true).Background(lipgloss.Color(t.DiffHeader))
	} else {
		// No background - just faint style
		s.DiffHeader = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("#6b7280"))
	}

	// Confirmation prompt: ALWAYS use high contrast (foreground + background)
	// This is critical for visibility - user MUST see y/n prompts clearly
	if t.ConfirmFg != "" && t.ConfirmBg != "" {
		// Both specified - use them
		s.Confirm = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(t.ConfirmFg)).Background(lipgloss.Color(t.ConfirmBg)).Padding(0, 1)
	} else if t.ConfirmBg != "" {
		// Only background - use bright white/yellow text for contrast
		s.Confirm = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color(t.ConfirmBg)).Padding(0, 1)
	} else if t.ConfirmFg != "" {
		// Only foreground - use it with underline for visibility
		s.Confirm = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(lipgloss.Color(t.ConfirmFg)).Padding(0, 1)
	} else {
		// Neither - use bright yellow on default background with underline
		s.Confirm = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(lipgloss.Color("#fbbf24")).Padding(0, 1)
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
	// Matrix - green on black, hacker style (original)
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
		ConfirmFg:  "#00ff00",
		ConfirmBg:  "#003300",
	}

	// Default - clean blue accents, no backgrounds
	Default = Theme{
		Name:       "default",
		Banner:     "#5f87ff",
		User:       "#5f87ff",
		Assistant:  "#d0d0d0",
		Tool:       "#d7af00",
		Error:      "#ff5f5f",
		Dim:        "#6c6c6c",
		Prompt:     "#5f87ff",
		Border:     "#4e4e4e",
		DiffAdd:    "",
		DiffRemove: "",
		DiffHeader: "#4e4e4e",
		ConfirmFg:  "#ffffff",
		ConfirmBg:  "#1e3a5f",
	}

	// Minimal - no colors at all
	Minimal = Theme{
		Name:       "minimal",
		Banner:     "",
		User:       "",
		Assistant:  "",
		Tool:       "",
		Error:      "",
		Dim:        "",
		Prompt:     "",
		Border:     "",
		DiffAdd:    "",
		DiffRemove: "",
		DiffHeader: "",
		ConfirmFg:  "",
		ConfirmBg:  "",
	}

	// Nord - Arctic, bluish-gray color palette (very readable)
	Nord = Theme{
		Name:       "nord",
		Banner:     "#88c0d0",
		User:       "#88c0d0",
		Assistant:  "#d8dee9",
		Tool:       "#81a1c1",
		Error:      "#bf616a",
		Dim:        "#4c566a",
		Prompt:     "#88c0d0",
		Border:     "#3b4252",
		DiffAdd:    "#2e443a",
		DiffRemove: "#423842",
		DiffHeader: "#3b4252",
		ConfirmFg:  "#eceff4",
		ConfirmBg:  "#5e81ac",
	}

	// Dracula - dark theme with high contrast
	Dracula = Theme{
		Name:       "dracula",
		Banner:     "#bd93f9",
		User:       "#8be9fd",
		Assistant:  "#f8f8f2",
		Tool:       "#ffb86c",
		Error:      "#ff5555",
		Dim:        "#6272a4",
		Prompt:     "#50fa7b",
		Border:     "#44475a",
		DiffAdd:    "#1e3a2f",
		DiffRemove: "#3a1e2f",
		DiffHeader: "#44475a",
		ConfirmFg:  "#f8f8f2",
		ConfirmBg:  "#6272a4",
	}

	// Gruvbox - retro groove colors, warm and cozy
	Gruvbox = Theme{
		Name:       "gruvbox",
		Banner:     "#d79921",
		User:       "#83a598",
		Assistant:  "#ebdbb2",
		Tool:       "#fe8019",
		Error:      "#fb4934",
		Dim:        "#928374",
		Prompt:     "#b8bb26",
		Border:     "#3c3836",
		DiffAdd:    "#323529",
		DiffRemove: "#3c2a2a",
		DiffHeader: "#3c3836",
		ConfirmFg:  "#ebdbb2",
		ConfirmBg:  "#665c54",
	}

	// TokyoNight - clean dark theme inspired by Tokyo Night
	TokyoNight = Theme{
		Name:       "tokyonight",
		Banner:     "#7aa2f7",
		User:       "#7dcfff",
		Assistant:  "#c0caf5",
		Tool:       "#e0af68",
		Error:      "#f7768e",
		Dim:        "#565f89",
		Prompt:     "#7dcfff",
		Border:     "#292e42",
		DiffAdd:    "#1e3a3a",
		DiffRemove: "#3a1e2e",
		DiffHeader: "#292e42",
		ConfirmFg:  "#c0caf5",
		ConfirmBg:  "#565f89",
	}

	// Catppuccin - soothing pastel theme
	Catppuccin = Theme{
		Name:       "catppuccin",
		Banner:     "#cba6f7",
		User:       "#89dceb",
		Assistant:  "#cdd6f4",
		Tool:       "#fab387",
		Error:      "#f38ba8",
		Dim:        "#6c7086",
		Prompt:     "#a6e3a1",
		Border:     "#313244",
		DiffAdd:    "#243032",
		DiffRemove: "#302430",
		DiffHeader: "#313244",
		ConfirmFg:  "#cdd6f4",
		ConfirmBg:  "#45475a",
	}

	// Solarized - precision colors for terminals
	Solarized = Theme{
		Name:       "solarized",
		Banner:     "#859900",
		User:       "#268bd2",
		Assistant:  "#839496",
		Tool:       "#b58900",
		Error:      "#dc322f",
		Dim:        "#586e75",
		Prompt:     "#2aa198",
		Border:     "#073642",
		DiffAdd:    "#073630",
		DiffRemove: "#360707",
		DiffHeader: "#073642",
		ConfirmFg:  "#fdf6e3",
		ConfirmBg:  "#657b83",
	}

	Builtin = map[string]*Theme{
		"matrix":     &Matrix,
		"default":    &Default,
		"minimal":    &Minimal,
		"nord":       &Nord,
		"dracula":    &Dracula,
		"gruvbox":    &Gruvbox,
		"tokyonight": &TokyoNight,
		"catppuccin": &Catppuccin,
		"solarized":  &Solarized,
	}
)

// Load reads the user's theme file. Returns the matrix theme if no file
// exists or if parsing fails. If the saved name matches a builtin theme,
// the builtin is returned so updated colors take effect on upgrade.
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
	// If the saved theme matches a builtin name, return the builtin
	// so color updates from new versions take effect.
	if b, ok := Builtin[t.Name]; ok {
		return b
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
