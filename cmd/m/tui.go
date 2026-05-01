package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/subzone/m/internal/engine"
	"github.com/subzone/m/internal/llm"
)

// streamMsg unifies model-text chunks and the final completion event.
// One channel per session; bubbletea's single-threaded Update reads
// them in order. runStepCmd pushes the terminal {done: true} after
// sess.Step returns so the UI knows to re-enable input.
type streamMsg struct {
	chunk  string
	done   bool
	err    error
	usage  llm.Usage // cumulative snapshot after Step completes
	lastIn int       // input_tokens from last call
}

// streamWriter is the io.Writer wired into engine.Config.Out. Each
// engine Write becomes one streamMsg{chunk}; the buffered channel
// gives the UI breathing room while engine streams faster than the
// renderer flushes a frame.
type streamWriter struct {
	ch chan<- streamMsg
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if len(p) > 0 {
		w.ch <- streamMsg{chunk: string(p)}
	}
	return len(p), nil
}

func listenStreamCmd(ch <-chan streamMsg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func runStepCmd(ctx context.Context, sess *engine.Session, ch chan<- streamMsg, line string) tea.Cmd {
	return func() tea.Msg {
		err := sess.Step(ctx, line)
		ch <- streamMsg{done: true, err: err, usage: sess.Usage(), lastIn: sess.LastInputTokens()}
		return nil
	}
}

// tuiHistoryCap bounds the in-memory chat transcript. Way past anything
// a single session is realistically going to produce; the truncation
// halves the buffer when crossed so we don't pay a sliding-window cost
// on every keystroke.
const tuiHistoryCap = 64 * 1024

type tuiModel struct {
	sess     *engine.Session
	ctx      context.Context
	streamCh chan streamMsg

	viewport viewport.Model
	input    textinput.Model
	spinner  spinner.Model

	history  *strings.Builder
	stats    sysStats
	usage    llm.Usage
	lastIn   int
	provider string
	model    string
	thinking bool
	theme    *Theme
	styles   Styles

	width  int
	height int
	name   string
}

func newTUIModel(ctx context.Context, sess *engine.Session, ch chan streamMsg, name, provider, model string) tuiModel {
	in := textinput.New()
	in.Prompt = "» "
	in.Placeholder = "type a message, /exit to quit"
	in.Focus()
	in.CharLimit = 4000

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	vp := viewport.New(80, 10)

	t := Load()
	s := t.Resolve()

	vp.SetContent(s.Dim.Render(fmt.Sprintf("chat with %s — /exit to quit, /reset to clear, /help for more", name)) + "\n\n")

	return tuiModel{
		sess:     sess,
		ctx:      ctx,
		streamCh: ch,
		viewport: vp,
		input:    in,
		spinner:  sp,
		history:  &strings.Builder{},
		stats:    blankStats(),
		provider: provider,
		model:    model,
		name:     name,
		theme:    t,
		styles:   s,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
		statsSampleCmd(),
		statsTickCmd(),
	)
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		if m.thinking {
			// Block all input while a turn is in flight, including
			// printable characters — otherwise the user can queue text
			// that gets misinterpreted when the next prompt is ready.
			return m, nil
		}
		if msg.Type == tea.KeyEnter {
			line := strings.TrimSpace(m.input.Value())
			m.input.SetValue("")
			if line == "" {
				return m, nil
			}
			switch line {
			case "/exit", "/quit":
				return m, tea.Quit
			case "/reset":
				m.sess.Reset()
				m.appendHistory("(history cleared)\n")
				return m, nil
			case "/help":
				m.appendHistory("commands: /exit, /quit, /reset, /compact, /model <provider/model>, /help\n")
				return m, nil
			}
			if strings.HasPrefix(line, "/model ") {
				newModel := strings.TrimSpace(strings.TrimPrefix(line, "/model "))
				p, model, err := llm.Resolve(newModel)
				if err != nil {
					m.appendHistory(fmt.Sprintf("error: %v\n", err))
					return m, nil
				}
				m.sess.SetModel(p, model)
				m.provider, _, _ = strings.Cut(newModel, "/")
				m.model = model
				m.appendHistory(fmt.Sprintf("switched to %s\n", newModel))
				return m, nil
			}
			if line == "/compact" {
				m.sess.Truncate(4)
				m.appendHistory("(compacted to last 4 exchanges)\n")
				return m, nil
			}
			if strings.HasPrefix(line, "/theme") {
				arg := strings.TrimSpace(strings.TrimPrefix(line, "/theme"))
				if arg == "" {
					names := []string{}
					for n := range Builtin {
						mark := "  "
						if n == m.theme.Name {
							mark = "* "
						}
						names = append(names, mark+n)
					}
					m.appendHistory("themes: " + strings.Join(names, ", ") + " (* = active)\n")
					return m, nil
				}
				t := ByName(arg)
				if t == nil {
					m.appendHistory(fmt.Sprintf("unknown theme %q (try /theme for list)\n", arg))
					return m, nil
				}
				m.theme = t
				m.styles = t.Resolve()
				_ = Save(t)
				m.appendHistory(fmt.Sprintf("switched to %s theme\n", arg))
				return m, nil
			}
			if strings.HasPrefix(line, "/") {
				m.appendHistory(fmt.Sprintf("unknown command %q (try /help)\n", line))
				return m, nil
			}
			m.appendHistory(m.styles.User.Render("» "+line) + "\n")
			m.thinking = true
			return m, tea.Batch(
				runStepCmd(m.ctx, m.sess, m.streamCh, line),
				listenStreamCmd(m.streamCh),
			)
		}

	case streamMsg:
		if msg.done {
			m.thinking = false
			m.usage = msg.usage
			m.lastIn = msg.lastIn
			if msg.err != nil {
				m.appendHistory(m.styles.Error.Render(fmt.Sprintf("error: %v", msg.err)) + "\n")
			} else {
				m.appendHistory("\n")
			}
			return m, nil
		}
		// Style tool activity indicators from the engine's Status writer.
		chunk := msg.chunk
		if strings.HasPrefix(chunk, "→ ") || strings.HasPrefix(chunk, "← ") {
			chunk = m.styles.Tool.Render(strings.TrimRight(chunk, "\n")) + "\n"
		}
		m.appendHistory(chunk)
		return m, listenStreamCmd(m.streamCh)

	case statsTickMsg:
		return m, tea.Batch(statsSampleCmd(), statsTickCmd())

	case statsMsg:
		m.stats = sysStats(msg)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// appendHistory mutates the receiver via pointer; the caller's value
// shares the same strings.Builder, viewport, etc. since those are
// internally pointer-y. Truncation halves the buffer when the cap is
// crossed; an approximate cut is fine because the user only sees the
// tail.
func (m *tuiModel) appendHistory(s string) {
	m.history.WriteString(s)
	if m.history.Len() > tuiHistoryCap {
		full := m.history.String()
		m.history.Reset()
		m.history.WriteString(full[len(full)/2:])
	}
	m.viewport.SetContent(m.history.String())
	m.viewport.GotoBottom()
}

// layout recomputes child component sizes whenever the terminal
// resizes. Header height is fixed at the banner's row count plus a
// border; input is one row plus padding.
func (m *tuiModel) layout() {
	const inputHeight = 1
	// Responsive header: hide elements on small terminals.
	headerHeight := 9 // banner(6) + model label(1) + cmds bar(1) + gap(1)
	if m.width < 80 {
		headerHeight = 3 // just cmds bar + minimal padding
	} else if m.height < 20 {
		headerHeight = 3
	}
	bodyHeight := m.height - headerHeight - inputHeight - 2
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	m.viewport.Width = m.width - 2
	m.viewport.Height = bodyHeight
	m.input.Width = m.width - 4
}

// View renders the static-header / scrolling-body / input layout. The
// header places the M banner on the left and the system-stats table
// on the right; lipgloss handles the alignment so the box edges stay
// flush regardless of terminal width.
func (m tuiModel) View() string {
	bannerBox := lipgloss.NewStyle().Padding(0, 2).Render(
		m.styles.Banner.Render(strings.TrimLeft(banner, "\n")),
	)
	tokenBox := renderTokenBox(m.usage, m.provider, m.model, m.styles.Dim)
	statsBox := renderStatsTable(m.stats)

	boxes := lipgloss.Width(bannerBox) + lipgloss.Width(tokenBox) + lipgloss.Width(statsBox)
	gapTotal := m.width - boxes
	if gapTotal < 0 {
		gapTotal = 0
	}
	gapLeft := gapTotal / 2
	gapRight := gapTotal - gapLeft
	spacerL := lipgloss.NewStyle().Width(gapLeft).Render("")
	spacerR := lipgloss.NewStyle().Width(gapRight).Render("")

	header := lipgloss.JoinHorizontal(lipgloss.Top, bannerBox, spacerL, tokenBox, spacerR, statsBox)

	cmdsBar := m.styles.Dim.Padding(0, 2).Render("/exit  /reset  /compact  /model  /config  /theme  /help")

	cwdLabel := ""
	if cwd, err := os.Getwd(); err == nil {
		cwdLabel = m.styles.Dim.Render("cwd: " + cwd)
	}

	body := m.viewport.View()
	if m.thinking {
		body += "\n" + m.styles.Dim.Render(m.spinner.View()+" thinking…")
	}

	inputLine := m.input.View()
	if pct := contextPercent(m.lastIn, m.model); pct >= 0 {
		ctxLabel := m.styles.Dim.Render(fmt.Sprintf("ctx: %d%%", pct))
		pad := m.width - lipgloss.Width(inputLine) - lipgloss.Width(ctxLabel) - 4
		if pad < 1 {
			pad = 1
		}
		inputLine = inputLine + strings.Repeat(" ", pad) + ctxLabel
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		cmdsBar+"  "+cwdLabel,
		bodyStyle.Render(body),
		inputStyle.Render(inputLine),
	)
}

var (
	bodyStyle  = lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.NormalBorder(), true, false)
	inputStyle = lipgloss.NewStyle().Padding(0, 1)

	statsKey = lipgloss.NewStyle().Width(6)
	statsVal = lipgloss.NewStyle().Align(lipgloss.Right).Width(8)
	statsBox = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1)
)

func renderStatsTable(s sysStats) string {
	rows := []string{
		statsKey.Render("CPU") + statsVal.Render(s.CPU),
		statsKey.Render("RAM") + statsVal.Render(s.RAM),
		statsKey.Render("GPU") + statsVal.Render(s.GPU),
		statsKey.Render("Disk") + statsVal.Render(s.Disk),
	}
	return statsBox.Render(strings.Join(rows, "\n"))
}

// renderTokenBox shows cumulative token counts and estimated cost for the
// session. Cost is a rough estimate based on published per-1M-token rates;
// models not in the table show tokens only.
func renderTokenBox(u llm.Usage, provider, model string, dim lipgloss.Style) string {
	total := u.InputTokens + u.OutputTokens
	rows := []string{
		statsKey.Render("In") + statsVal.Render(formatTokens(u.InputTokens)),
		statsKey.Render("Out") + statsVal.Render(formatTokens(u.OutputTokens)),
		statsKey.Render("Total") + statsVal.Render(formatTokens(total)),
	}
	cost := estimateCost(u, model)
	rows = append(rows, statsKey.Render("Cost") + statsVal.Render(formatCost(cost)))
	box := tokenBoxStyle.Render(strings.Join(rows, "\n"))
	label := provider + "/" + model
	labelStyled := dim.Align(lipgloss.Center).Width(lipgloss.Width(box)).Render(label)
	return lipgloss.JoinVertical(lipgloss.Center, box, labelStyled)
}

func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func formatCost(dollars float64) string {
	if dollars < 0.01 {
		return fmt.Sprintf("$%.4f", dollars)
	}
	return fmt.Sprintf("$%.2f", dollars)
}

// pricing is per-million-token rates (input, output). Rough estimates from
// published pricing pages; updated as of mid-2025.
type pricing struct{ input, output float64 }

var modelPricing = map[string]pricing{
	"claude-sonnet-4-6":        {3.0, 15.0},
	"claude-sonnet-4-20250514": {3.0, 15.0},
	"claude-haiku-3-5":         {0.80, 4.0},
	"claude-opus-4":            {15.0, 75.0},
	"gpt-4o":                   {2.50, 10.0},
	"gpt-4o-mini":              {0.15, 0.60},
	"gpt-4.1":                  {2.0, 8.0},
	"gpt-4.1-mini":             {0.40, 1.60},
	"gpt-4.1-nano":             {0.10, 0.40},
	"o3":                       {2.0, 8.0},
	"o3-mini":                  {1.10, 4.40},
	"o4-mini":                  {1.10, 4.40},
	"gemini-2.5-pro":           {1.25, 10.0},
	"gemini-2.5-flash":         {0.15, 0.60},
	"gemini-2.0-flash":         {0.10, 0.40},
	"qwen-plus":               {0.80, 2.0},
	"qwen-max":                {2.0, 6.0},
	"qwen-turbo":              {0.30, 0.60},
}

var modelContextWindow = map[string]int{
	"claude-sonnet-4-6":        200_000,
	"claude-sonnet-4-20250514": 200_000,
	"claude-haiku-3-5":         200_000,
	"claude-opus-4":            200_000,
	"gpt-4o":                   128_000,
	"gpt-4o-mini":              128_000,
	"gpt-4.1":                  1_000_000,
	"gpt-4.1-mini":             1_000_000,
	"gpt-4.1-nano":             1_000_000,
	"o3":                       200_000,
	"o3-mini":                  200_000,
	"o4-mini":                  200_000,
	"gemini-2.5-pro":           1_000_000,
	"gemini-2.5-flash":         1_000_000,
	"gemini-2.0-flash":         1_000_000,
	"qwen-plus":               131_072,
	"qwen-max":                32_768,
	"qwen-turbo":              131_072,
}

func contextPercent(lastInputTokens int, model string) int {
	window, ok := modelContextWindow[model]
	if !ok || window == 0 || lastInputTokens == 0 {
		return -1 // unknown
	}
	return (lastInputTokens * 100) / window
}

func estimateCost(u llm.Usage, model string) float64 {
	p, ok := modelPricing[model]
	if !ok {
		return 0
	}
	return (float64(u.InputTokens)*p.input + float64(u.OutputTokens)*p.output) / 1_000_000
}

var tokenBoxStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1)

// runTUI runs the bubbletea program. The caller is responsible for
// having wired sess.Out to a streamWriter pointing at streamCh — that
// happens in runChatWithDoc so the engine starts streaming the moment
// Step is called.
func runTUI(ctx context.Context, sess *engine.Session, streamCh chan streamMsg, name, provider, model string) error {
	m := newTUIModel(ctx, sess, streamCh, name, provider, model)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
