package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/subzone/Agentctl/internal/engine"
	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/tools"
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
const tuiHistoryCap = 256 * 1024

type tuiModel struct {
	sess     *engine.Session
	ctx      context.Context
	streamCh chan streamMsg

	viewport viewport.Model
	input    textinput.Model
	spinner  spinner.Model

	history    *strings.Builder
	stats      sysStats
	usage      llm.Usage
	lastIn     int
	provider   string
	model      string
	thinking   bool
	thinkTick  int
	theme      *Theme
	styles     Styles
	undo       *tools.UndoStack
	stepCancel context.CancelFunc
	confirmCh  chan bool // channel for tool confirmation in TUI
	confirming bool      // true when waiting for y/n

	width  int
	height int
	name           string
	agentPhrases   []string // per-agent thinking phrases (highest priority)
	trustMode      *bool    // shared with tool confirm closure

	// UI stability improvements
	lastUpdateTime time.Time // timestamp of last viewport update to throttle
}

func newTUIModel(ctx context.Context, sess *engine.Session, ch chan streamMsg, name, provider, model string, undo *tools.UndoStack, confirmCh chan bool, agentPhrases []string, trust *bool) tuiModel {
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
		sess:           sess,
		ctx:            ctx,
		streamCh:       ch,
		viewport:       vp,
		input:          in,
		spinner:        sp,
		history:        &strings.Builder{},
		stats:          blankStats(),
		provider:       provider,
		model:          model,
		name:           name,
		theme:          t,
		styles:         s,
		undo:           undo,
		confirmCh:      confirmCh,
		agentPhrases:   agentPhrases,
		trustMode:      trust,
		lastUpdateTime: time.Now(),
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
		wrapped := wordWrap(m.history.String(), m.viewport.Width)
		m.viewport.SetContent(wrapped)
		m.viewport.GotoBottom()
		return m, nil

	case tea.KeyMsg:
		// Always allow viewport updates for navigation during streaming
		// but block input updates
		if msg.Type == tea.KeyCtrlC {
			if m.thinking && m.stepCancel != nil {
				m.stepCancel()
				m.thinking = false
				m.appendHistory("\n(cancelled)\n")
				// Keep listening to drain the done message from the goroutine.
				return m, listenStreamCmd(m.streamCh)
			}
			return m, tea.Quit
		}
		if msg.Type == tea.KeyEsc && m.thinking && m.stepCancel != nil {
			m.stepCancel()
			m.thinking = false
			m.appendHistory("\n(cancelled)\n")
			return m, listenStreamCmd(m.streamCh)
		}

		// Handle tool confirmation y/n
		if m.confirming {
			switch msg.String() {
			case "y", "Y":
				m.confirming = false
				m.confirmCh <- true
				m.appendHistory("y\n")
				return m, listenStreamCmd(m.streamCh)
			case "n", "N":
				m.confirming = false
				m.confirmCh <- false
				m.appendHistory("n (declined)\n")
				return m, listenStreamCmd(m.streamCh)
			}
			return m, nil
		}
		// Handle viewport navigation keys - ALWAYS allow scrolling with keyboard
		// This fixes the issue where user can only scroll when agent is thinking
		switch msg.Type {
		case tea.KeyUp, tea.KeyDown, tea.KeyHome, tea.KeyEnd, tea.KeyPgUp, tea.KeyPgDown:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

		// Block input while thinking (except navigation which we handled above)
		if m.thinking {
			// Block all other input while a turn is in flight, including
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
				m.appendHistory("commands: /exit /quit /reset /compact /undo /trust /debug /model <provider/model> /models /theme [name] /themes /save /sessions /resume <id> /config /help\n")
				return m, nil
			case "/trust":
				if m.trustMode != nil {
					*m.trustMode = true
				}
				m.appendHistory("trust mode ON — all tools auto-approved\n")
				return m, nil
			case "/trust off":
				if m.trustMode != nil {
					*m.trustMode = false
				}
				m.appendHistory("trust mode OFF — destructive tools require confirmation\n")
				return m, nil
			case "/debug":
				m.appendHistory("debug mode not available in TUI — use REPL mode (pipe input) for raw LLM trace\n")
				return m, nil
			case "/config":
				m.appendHistory("run `m config` from the shell to manage providers and models\n")
				return m, nil
			}
			if line == "/models" || strings.HasPrefix(line, "/models ") {
				buf := &strings.Builder{}
				handleModelsCommand(line, m.sess, buf)
				result := buf.String()
				m.appendHistory(result)
				// Update provider/model display if switched
				if idx := strings.Index(result, "switched to "); idx >= 0 {
					full := strings.TrimSpace(result[idx+len("switched to "):])
					if p, mod, ok := strings.Cut(full, "/"); ok {
						m.provider = p
						m.model = mod
					}
				}
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
			if line == "/undo" {
				if m.undo == nil {
					m.appendHistory("undo not available\n")
				} else if msg, err := m.undo.Pop(); err != nil {
					m.appendHistory(fmt.Sprintf("undo: %v\n", err))
				} else {
					m.appendHistory(msg + "\n")
				}
				return m, nil
			}
			if line == "/themes" || strings.HasPrefix(line, "/themes ") {
				themeDescriptions := []string{
					"  default  - clean blue accents, no diff backgrounds (best for light terminals)",
					"  minimal  - no colors at all, pure terminal defaults",
					"  matrix   - green on black, hacker style",
					"  nord     - arctic bluish-gray, very readable",
					"  dracula  - dark with high contrast, popular",
					"  gruvbox  - warm retro colors, cozy feeling",
					"  tokyonight - clean dark, Japanese aesthetic",
					"  catppuccin - soothing pastel colors",
					"  solarized - precision terminal colors, classic",
				}
				current := "current: " + m.theme.Name
				m.appendHistory("available themes:\n" + strings.Join(themeDescriptions, "\n") + "\n" + current + "\n")
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
			if line == "/save" {
				buf := &strings.Builder{}
				handleSessionSave(m.sess, buf)
				m.appendHistory(buf.String())
				return m, nil
			}
			if line == "/sessions" {
				buf := &strings.Builder{}
				handleSessionList(buf)
				m.appendHistory(buf.String())
				return m, nil
			}
			if strings.HasPrefix(line, "/resume ") {
				buf := &strings.Builder{}
				handleSessionResume(line, m.sess, buf)
				m.appendHistory(buf.String())
				return m, nil
			}
			if strings.HasPrefix(line, "/") {
				m.appendHistory(fmt.Sprintf("unknown command %q (try /help)\n", line))
				return m, nil
			}
			m.appendHistory(m.styles.User.Render("» "+line) + "\n")
			m.thinking = true
			m.thinkTick = 0
			stepCtx, stepCancel := context.WithCancel(m.ctx)
			m.stepCancel = stepCancel
			return m, tea.Batch(
				runStepCmd(stepCtx, m.sess, m.streamCh, line),
				listenStreamCmd(m.streamCh),
			)
		}

	case streamMsg:
		if msg.done {
			// Always update usage counters, even after cancel.
			m.usage = msg.usage
			m.lastIn = msg.lastIn
			if m.thinking {
				m.thinking = false
				if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
					m.appendHistoryImportant(m.styles.Error.Render(fmt.Sprintf("error: %v", msg.err)) + "\n")
				} else {
					// Force viewport update to flush any throttled content.
					m.appendHistoryImportant("\n")
				}
			}
			// If not thinking (already cancelled), just drain silently.
			// Autosave after each completed step.
			snap := exportSession(m.sess, m.provider, m.model)
			go func() { autoSaveSnapshot(snap) }()
			return m, nil
		}
		// After cancel, ignore stale chunks.
		if !m.thinking {
			return m, listenStreamCmd(m.streamCh)
		}
		// Detect tool confirmation prompts.
		chunk := msg.chunk
		if strings.HasSuffix(strings.TrimSpace(chunk), "[y/n]") {
			m.confirming = true
			m.appendHistoryImportant(m.styles.Confirm.Render(strings.TrimRight(chunk, "\n")) + "\n")
			return m, nil // Don't listen for more — wait for y/n keypress
		}
		if strings.HasPrefix(chunk, "→ ") || strings.HasPrefix(chunk, "← ") {
			// Add newline before tool lines if previous content didn't end with one
			histStr := m.history.String()
			if len(histStr) > 0 && histStr[len(histStr)-1] != '\n' {
				chunk = "\n" + chunk
			}
			chunk = m.styles.Tool.Render(strings.TrimRight(chunk, "\n")) + "\n"
		}
		// Style diff lines from fs_write previews.
		chunk = styleDiffLines(chunk, m.styles)
		chunk = fixMarkdownSpacing(chunk)
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
		if m.thinking {
			m.thinkTick++
		}
		return m, cmd
	}

	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Always allow viewport updates (for navigation)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	// Only update input if we're not thinking (agent is streaming)
	// This prevents input from "pulling" during streaming
	if !m.thinking {
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// appendHistory adds text to the chat transcript and updates the viewport.
// During streaming (m.thinking == true), always auto-scrolls to bottom.
// When idle, preserves the user's scroll position.
func (m *tuiModel) appendHistory(s string) {
	m.appendHistoryForce(s, false)
}

// appendHistoryImportant adds text and always updates the viewport,
// bypassing the streaming throttle. Use for confirm prompts and other
// state-change messages that the user must see immediately.
func (m *tuiModel) appendHistoryImportant(s string) {
	m.appendHistoryForce(s, true)
}

func (m *tuiModel) appendHistoryForce(s string, force bool) {
	m.history.WriteString(s)

	// Truncate if history is too large.
	if m.history.Len() > tuiHistoryCap {
		content := m.history.String()
		half := len(content) / 2
		if idx := strings.Index(content[half:], "\n"); idx >= 0 {
			content = content[half+idx+1:]
		} else {
			content = content[half:]
		}
		m.history.Reset()
		m.history.WriteString(content)
	}

	// Throttle viewport updates to ~60fps during streaming, but never
	// throttle important messages (confirm prompts, errors, etc.).
	if !force && m.thinking && time.Since(m.lastUpdateTime) < 16*time.Millisecond {
		return
	}
	m.lastUpdateTime = time.Now()

	wrapped := wordWrap(m.history.String(), m.viewport.Width)
	m.viewport.SetContent(wrapped)
	m.viewport.GotoBottom()
}

// wordWrap breaks lines longer than width at word boundaries. Lines
// already shorter than width pass through unchanged. Preserves existing
// newlines.
// fixMarkdownSpacing inserts newlines before numbered list items and
// markdown headers when they're glued to previous text, and renders
// basic markdown formatting as terminal styles.
func fixMarkdownSpacing(s string) string {
	if len(s) < 2 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 64)
	prev := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		// Insert newline before a digit that starts a list item
		if prev != 0 && prev != '\n' && prev != ' ' && c >= '1' && c <= '9' {
			if i+1 < len(s) && s[i+1] == '.' {
				b.WriteByte('\n')
			}
		}
		// Render **bold** as terminal bold
		if c == '*' && i+1 < len(s) && s[i+1] == '*' {
			// Find closing **
			end := strings.Index(s[i+2:], "**")
			if end >= 0 {
				b.WriteString("\033[1m") // bold on
				b.WriteString(s[i+2 : i+2+end])
				b.WriteString("\033[22m") // bold off
				i += 2 + end + 1 // skip past closing **
				if i < len(s) {
					prev = s[i]
				}
				continue
			}
		}
		// Render `code` as dim
		if c == '`' && (i+2 >= len(s) || s[i+1] != '`' || s[i+2] != '`') {
			end := strings.IndexByte(s[i+1:], '`')
			if end >= 0 {
				b.WriteString("\033[2m") // dim on
				b.WriteString(s[i+1 : i+1+end])
				b.WriteString("\033[22m") // dim off
				i += 1 + end
				prev = '`'
				continue
			}
		}
		// Convert ## headers to bold text with newline before
		if c == '#' && (prev == '\n' || prev == 0) {
			j := i
			for j < len(s) && s[j] == '#' {
				j++
			}
			if j < len(s) && s[j] == ' ' {
				j++
			}
			// Find end of header line
			nl := strings.IndexByte(s[j:], '\n')
			var headerText string
			if nl >= 0 {
				headerText = s[j : j+nl]
				i = j + nl - 1
			} else {
				headerText = s[j:]
				i = len(s) - 1
			}
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString("\033[1m") // bold on
			b.WriteString(headerText)
			b.WriteString("\033[22m") // bold off
			prev = '\n'
			continue
		}
		// Insert newline before ## headers glued to text
		if prev != 0 && prev != '\n' && c == '#' {
			if i+1 < len(s) && s[i+1] == '#' {
				b.WriteByte('\n')
			}
		}
		b.WriteByte(c)
		prev = c
	}
	return b.String()
}

// styleDiffLines applies background colors to diff lines (+ and - prefixed).
// Also styles --- and +++ headers.
func styleDiffLines(chunk string, s Styles) string {
	if !strings.Contains(chunk, "\n") {
		return styleSingleDiffLine(chunk, s)
	}
	var out strings.Builder
	for _, line := range strings.Split(chunk, "\n") {
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(styleSingleDiffLine(line, s))
	}
	return out.String()
}

func styleSingleDiffLine(line string, s Styles) string {
	if strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") {
		return s.DiffHeader.Render(line)
	}
	if strings.HasPrefix(line, "+ ") {
		return s.DiffAdd.Render(line)
	}
	if strings.HasPrefix(line, "- ") {
		return s.DiffRemove.Render(line)
	}
	return line
}

func wordWrap(s string, width int) string {
	if width <= 0 || len(s) == 0 {
		return s
	}
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if len(line) <= width {
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}
		// Break at word boundaries.
		rem := line
		for len(rem) > width {
			// Find last space within width.
			cut := strings.LastIndex(rem[:width], " ")
			if cut <= 0 {
				// No space found — hard break.
				cut = width
			}
			b.WriteString(rem[:cut])
			b.WriteByte('\n')
			rem = strings.TrimLeft(rem[cut:], " ")
		}
		b.WriteString(rem)
		b.WriteByte('\n')
	}
	// Remove the trailing extra newline we added.
	out := b.String()
	if len(out) > 0 && len(s) > 0 && s[len(s)-1] != '\n' {
		out = out[:len(out)-1]
	}
	return out
}

// layout recomputes child component sizes whenever the terminal
// resizes. Header height is fixed at the banner's row count plus a
// border; input is one row plus padding.
func (m *tuiModel) layout() {
	const inputHeight = 1
	// Responsive header: hide elements on small terminals.
	headerHeight := 11 // banner(6) + version(1) + copyright(1) + model label(1) + cmds bar(1) + gap(1)
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
	bannerText := m.styles.Banner.Render(strings.TrimLeft(banner, "\n"))
	versionLine := m.styles.Dim.Render("AgentCTL " + Version)
	copyLine := m.styles.Dim.Render("© 2026 @subzone")
	bannerBox := lipgloss.NewStyle().Padding(0, 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, bannerText, versionLine, copyLine),
	)
	tokenBox := renderTokenBox(m.usage, m.lastIn, m.provider, m.model, m.styles.Dim)
	statsRendered := renderStatsTable(m.stats)
	tagline := m.styles.Dim.Render("made with ❤ for DevOps")
	statsBox := lipgloss.JoinVertical(lipgloss.Center, statsRendered, tagline)

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

	cmdsBar := m.styles.Dim.Padding(0, 2).Render("/exit /reset /compact /undo /model /models /config /theme /help")

	cwdLabel := ""
	if cwd, err := os.Getwd(); err == nil {
		cwdLabel = m.styles.Dim.Render("cwd: " + cwd)
	}

	body := m.viewport.View()
	if m.thinking {
		body += "\n" + m.styles.Dim.Render(m.spinner.View()+" "+thinkingPhrase(m.thinkTick, m.theme, m.agentPhrases))
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
func renderTokenBox(u llm.Usage, lastIn int, provider, model string, dim lipgloss.Style) string {
	total := u.InputTokens + u.OutputTokens
	rows := []string{
		statsKey.Render("In") + statsVal.Render(formatTokens(u.InputTokens)),
		statsKey.Render("Out") + statsVal.Render(formatTokens(u.OutputTokens)),
		statsKey.Render("Total") + statsVal.Render(formatTokens(total)),
	}
	cost := estimateCost(u, model)
	rows = append(rows, statsKey.Render("Cost")+statsVal.Render(formatCost(cost)))
	if pct := contextPercent(lastIn, model); pct >= 0 {
		rows = append(rows, statsKey.Render("Ctx")+statsVal.Render(fmt.Sprintf("%d%%", pct)))
	}
	box := tokenBoxStyle.Render(strings.Join(rows, "\n"))
	label := provider + "/" + model
	labelStyled := dim.Render(label)
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
	"claude-sonnet-4-6":         {3.0, 15.0},
	"claude-sonnet-4-20250514":  {3.0, 15.0},
	"claude-haiku-3-5":          {0.80, 4.0},
	"claude-haiku-4-5-20251001": {1.0, 5.0},
	"claude-opus-4":             {15.0, 75.0},
	"gpt-4o":                    {2.50, 10.0},
	"gpt-4o-mini":               {0.15, 0.60},
	"gpt-4.1":                   {2.0, 8.0},
	"gpt-4.1-mini":              {0.40, 1.60},
	"gpt-4.1-nano":              {0.10, 0.40},
	"o3":                        {2.0, 8.0},
	"o3-mini":                   {1.10, 4.40},
	"o4-mini":                   {1.10, 4.40},
	"gemini-2.5-pro":            {1.25, 10.0},
	"gemini-2.5-flash":          {0.15, 0.60},
	"gemini-2.0-flash":          {0.10, 0.40},
	"qwen-plus":                 {0.80, 2.0},
	"qwen-max":                  {2.0, 6.0},
	"qwen-turbo":                {0.30, 0.60},
	"deepseek-v3":               {0.27, 1.10},
	"deepseek-v3.2":             {0.27, 1.10},
	"deepseek-chat":             {0.14, 0.28},
	"deepseek-r1":               {0.55, 2.19},
	"qwen3.6-plus":              {0.80, 2.0},
	"qwen3.6-flash":             {0.15, 0.60},
	"qwen3-coder-plus":          {0.80, 2.0},
	"qwen3-coder-flash":         {0.15, 0.60},
	"qwen3-max":                 {2.0, 6.0},
	"qwen3.5-plus":              {0.80, 2.0},
	"qwen3.5-flash":             {0.15, 0.60},
	"glm-5":                     {0.50, 0.50},
	"glm-4-plus":                {0.35, 0.35},
	"glm-4-flash":               {0.01, 0.01},
	"MiniMax-M2.5":              {1.0, 4.0},
	"MiniMax-Text-01":           {0.50, 2.0},
}

var modelContextWindow = map[string]int{
	"claude-sonnet-4-6":         200_000,
	"claude-sonnet-4-20250514":  200_000,
	"claude-haiku-3-5":          200_000,
	"claude-haiku-4-5-20251001": 200_000,
	"claude-opus-4":             200_000,
	"gpt-4o":                    128_000,
	"gpt-4o-mini":               128_000,
	"gpt-4.1":                   1_000_000,
	"gpt-4.1-mini":              1_000_000,
	"gpt-4.1-nano":              1_000_000,
	"o3":                        200_000,
	"o3-mini":                   200_000,
	"o4-mini":                   200_000,
	"gemini-2.5-pro":            1_000_000,
	"gemini-2.5-flash":          1_000_000,
	"gemini-2.0-flash":          1_000_000,
	"qwen-plus":                 131_072,
	"qwen-max":                  32_768,
	"qwen-turbo":                131_072,
	"deepseek-v3":               64_000,
	"deepseek-v3.2":             64_000,
	"deepseek-chat":             64_000,
	"deepseek-r1":               64_000,
	"qwen3.6-plus":              131_072,
	"qwen3.6-flash":             131_072,
	"qwen3-coder-plus":          131_072,
	"qwen3-coder-flash":         131_072,
	"qwen3-max":                 32_768,
	"qwen3.5-plus":              131_072,
	"qwen3.5-flash":             131_072,
	"glm-5":                     128_000,
	"glm-4-plus":                128_000,
	"glm-4-flash":               128_000,
	"MiniMax-M2.5":              1_000_000,
	"MiniMax-Text-01":           1_000_000,
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

var defaultThinkingPhrases = []string{
	"brewing ideas",
	"reading the codebase",
	"connecting the dots",
	"weighing options",
	"almost there",
	"crafting a response",
	"digging deeper",
	"putting it together",
	"one moment",
	"working on it",
}

func thinkingPhrase(tick int, theme *Theme, agentPhrases []string) string {
	phrases := defaultThinkingPhrases
	if theme != nil && len(theme.ThinkingPhrases) > 0 {
		phrases = theme.ThinkingPhrases
	}
	if len(agentPhrases) > 0 {
		phrases = agentPhrases
	}
	// Each phrase gets ~6 seconds (spinner ticks ~4x/sec, so 24 ticks).
	// Within each phrase cycle, show typing dots for the first 8 ticks.
	cycle := tick % 24
	idx := (tick / 24) % len(phrases)
	phrase := phrases[idx]
	if cycle < 8 {
		// Typing dots: . .. ...
		dots := (cycle / 2) % 4
		if dots == 0 {
			return phrase
		}
		return phrase + strings.Repeat(".", dots)
	}
	return phrase + "..."
}

// runTUI runs the bubbletea program. The caller is responsible for
// having wired sess.Out to a streamWriter pointing at streamCh — that
// happens in runChatWithDoc so the engine starts streaming the moment
// Step is called.
func runTUI(ctx context.Context, sess *engine.Session, streamCh chan streamMsg, name, provider, model string, undo *tools.UndoStack, confirmCh chan bool, agentPhrases []string, trust *bool) error {
	m := newTUIModel(ctx, sess, streamCh, name, provider, model, undo, confirmCh, agentPhrases, trust)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
