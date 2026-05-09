package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/subzone/Agentctl/internal/userconfig"
)

// release captures the user-visible highlights for one tagged version.
// Entries are kept newest-first so display logic can walk the slice in
// order and stop once it hits a version the user has already seen.
type release struct {
	Version    string
	Date       string
	Highlights []string
}

// releases is the source of truth for what gets shown in the changelog
// and "what's new" banner. When you cut a new tag, prepend an entry.
var releases = []release{
	{
		Version: "0.0.35",
		Date:    "2026-05-06",
		Highlights: []string{
			"m pipe: stdin/stdout mode for Unix pipelines (cat log | m pipe 'explain').",
			"@file context: @main.go in prompts auto-inlines file content. No tool call needed.",
			"m cost: token usage and estimated cost for recent sessions.",
			"m diff: show all uncommitted changes the agent made (git diff + stat).",
			"m mcp setup: automated MCP server install + credential config + connectivity check.",
			"m session list/export/delete: full session management from CLI.",
			"developer-hub agent: Jira ticket → branch → code → PR → Confluence docs.",
			"Web design agents: steve-webdev/steva-webdev hub + code/design/review spokes.",
			"Steva/Steve rebuilt as hub-and-spoke: code spoke + infra spoke with structured output.",
			"Hub agents resolve spokes from bundled/registry (no clone needed).",
			"42 bundled agents (was 32).",
		},
	},
	{
		Version: "0.0.34",
		Date:    "2026-05-05",
		Highlights: []string{
			"Default agent personality: The Pragmatic Architect — direct, FinOps-aware, automation-first.",
			"m run --yes: auto-approve tools for CI/headless (dangerous commands still blocked).",
			"m run --dry-run: validate agent and show config without calling the LLM.",
			"m search: fuzzy agent discovery by name, model, or path.",
			"m upgrade: self-update via brew, go install, or manual download.",
			"fs_write_multi: atomic multi-file writes with rollback on failure.",
			"Trace spans: per-turn LLM timing and per-tool duration (debug mode).",
			"Log file rotation: ~/.config/m/logs/ with 5-file cap.",
		},
	},
	{
		Version: "0.0.33",
		Date:    "2026-05-05",
		Highlights: []string{
			"Bundled agents: all 32 example agents embedded in the binary via go:embed.",
			"No clone needed: m chat devops / m run reviewer work immediately after install.",
			"On-demand extraction: agents extracted to ~/.config/m/agents/ on first use.",
			"m list shows (bundled) agents even before extraction.",
			"User edits preserved: extracted agents are never overwritten.",
			"Windows CI fix: bash shell for gosec/govulncheck, JSON path escaping in tests, .exe suffix.",
		},
	},
	{
		Version: "0.0.32",
		Date:    "2026-05-05",
		Highlights: []string{
			"Structured logging (slog): JSON output, debug/warn levels, /debug toggle.",
			"Graceful shutdown: session auto-saved on Ctrl+C / SIGTERM.",
			"Integration tests: 10 black-box CLI tests (m new, m list, m doctor, etc.).",
			"Coverage tests: dangerous commands, slash commands, markdown rendering, version compare.",
			"cmd/m coverage: 16% \u2192 25%.",
			"TUI fix: ANSI-aware word wrap prevents garbled escape codes.",
		},
	},
	{
		Version: "0.0.31",
		Date:    "2026-05-04",
		Highlights: []string{
			"PII guardrails: redact emails, phones, SSNs, credit cards, API keys before sending to LLM.",
			"Agent config: pii_guard: redact/warn/off. Toggle with /pii in session.",
			"Mermaid diagram skill: teaches agents to produce architecture diagrams.",
			"SAST (gosec) + SCA (govulncheck) in CI pipeline.",
			"Secret scanning + push protection enabled.",
		},
	},
	{
		Version: "0.0.30",
		Date:    "2026-05-04",
		Highlights: []string{
			"Update notifier: checks GitHub once/day, shows notice if newer version available.",
			"Non-blocking, cached, silent on errors. Never slows down startup.",
		},
	},
	{
		Version: "0.0.29",
		Date:    "2026-05-04",
		Highlights: []string{
			"REPL color: bold blue prompt, dim tips/shortcuts.",
			"Help examples: m chat, m run show real usage examples.",
			"Session summary: /sessions shows first user message for each session.",
			"Tool output: shows line count or short output instead of raw byte count.",
		},
	},
	{
		Version: "0.0.28",
		Date:    "2026-05-04",
		Highlights: []string{
			"Command shortcuts: /x /r /c /u /m /t /s /h for all slash commands.",
			"TUI command bar: underlined shortcut letters for discoverability.",
			"Welcome message: shortcuts listed on every session start.",
		},
	},
	{
		Version: "0.0.27",
		Date:    "2026-05-04",
		Highlights: []string{
			"First-run wizard: visual completion summary with next steps.",
			"TUI + REPL onboarding: tips line showing key commands on every session start.",
		},
	},
	{
		Version: "0.0.26",
		Date:    "2026-05-04",
		Highlights: []string{
			"m new: scaffold a new agent .md file with boilerplate.",
			"m doctor: health check (config, API key, model reachability, tools).",
			"m completion: shell completions for bash/zsh/fish/powershell.",
			"Named sessions: /save fixing-auth-bug creates a named snapshot.",
			"Better error messages: API key errors now tell you how to fix them.",
		},
	},
	{
		Version: "0.0.25",
		Date:    "2026-05-04",
		Highlights: []string{
			"MCP HTTP/SSE transport: all three transports now supported (stdio, http, sse).",
			"Agent registry: m install + m run/chat by name (no path needed).",
			"Shell timeout: 30s \u2192 120s (terraform/docker friendly).",
			"Undo stack capped at 20 entries.",
			"Example MCP servers: datadog (HTTP), slack (SSE).",
		},
	},
	{
		Version: "0.0.24",
		Date:    "2026-05-04",
		Highlights: []string{
			"code_search tool: grep + in-memory symbol index (functions, types, classes, imports).",
			"Languages: Go, Python, JS, TS, Java, Ruby, Rust, Terraform, Shell.",
			"Session history: autosave rotates, keeps last 10 backups.",
			"/trust command: auto-approve tools (dangerous commands still double-confirm).",
			"Dangerous command protection: 34 patterns always require double y/n.",
		},
	},
	{
		Version: "0.0.23",
		Date:    "2026-05-04",
		Highlights: []string{
			"Fallback models: auto-switch on 429 rate limit. Added to all 30 example agents.",
			"Per-agent thinking phrases: customize spinner text per agent (e.g. Serbian for Steva \u0110ubre).",
			"Markdown rendering in TUI: **bold** renders as terminal bold, `code` as dim, ## headers as bold with spacing.",
			"Tool confirmation cleanup: shows key=value instead of raw JSON.",
			"Tool output formatting: newlines before/after tool activity lines.",
			"Removed (thinking) text marker \u2014 TUI spinner handles thinking status.",
			"Continue prompt: 'Agent worked on this for a while' instead of technical turn count.",
			"TUI: version + copyright below M banner, tagline below stats.",
			"Docs: fallback and thinking_phrases documented in README and agents page.",
		},
	},
	{
		Version: "0.0.22",
		Date:    "2026-05-04",
		Highlights: []string{
			"Error intervention: agent auto-retries on tool failures with context.",
			"Anthropic prompt caching support.",
			"TUI: version + copyright displayed below M banner.",
			"All docs and GitHub Pages synced to v0.0.22.",
		},
	},
	{
		Version: "0.0.21",
		Date:    "2026-07-04",
		Highlights: []string{
			"High-contrast y/n confirmation prompts — ConfirmFg theme field, all 9 themes updated.",
			"TUI: bypass throttle for important messages (tool output, confirmations).",
			"Lint fixes: gocritic if-else chain rewritten to switch, go fmt formatting.",
			"CI: validate all examples/ so cross-references resolve.",
			"First self-made release by Steva Đubre!",
		},
	},
	{
		Version: "0.0.19",
		Date:    "2026-07-04",
		Highlights: []string{
			"Homebrew install: brew tap subzone/tap && brew install subzone/tap/m",
			"Auto-update Homebrew formula on every release via CI.",
			"SECURITY.md, CODE_OF_CONDUCT.md, CODEOWNERS, issue/PR templates.",
			"CI on every push/PR (not just tags). Dependabot for Go modules.",
			"Branch protection, tag protection, repo topics for discoverability.",
		},
	},
	{
		Version: "0.0.18",
		Date:    "2026-07-04",
		Highlights: []string{
			"Session persistence: autosave after every step, /save /sessions /resume. AES-256-GCM encrypted.",
			"web_fetch tool: fetch URLs and extract readable text. Stdlib-only, no API key needed.",
			"/models command: numbered model picker. Probes token plan endpoints when /v1/models unavailable.",
			"9 built-in themes: matrix, default, minimal, nord, dracula, gruvbox, tokyonight, catppuccin, solarized.",
			"m list: agent discovery command. Scans directories for .md files with agent frontmatter.",
			"Token-based context compaction: per-model context windows, 80% budget, meaningful summaries.",
			"Reasoning model support: MiniMax-M2.5, DeepSeek-R1 thinking indicator.",
			"Alibaba token plan support: custom base URL, GLM-5, MiniMax-M2.5 via DashScope.",
			"TUI guardrails: auto-approve read-only tools, y/n prompt for shell/fs_write/git.",
			"TUI scrolling fix: eliminated jitter during streaming.",
			"DashScope model filtering: 154 → 45 models (skip non-chat, deduplicate dated snapshots).",
		},
	},
	{
		Version: "0.0.17",
		Date:    "2026-05-03",
		Highlights: []string{
			"Professional product landing page at subzone.github.io/m — pure HTML/CSS, Font Awesome icons, no Jekyll.",
			"7-page documentation site: home, install (platform tabs), agents gallery, architecture (comparison table vs Cursor/Copilot/Aider), MCP servers, changelog timeline, support/troubleshooting.",
			"ASCII art M banner in site nav and footer.",
			"DevOps agents: k8s-debug (Kubernetes triage), terraform-plan (plan review), helm-deploy (chart management).",
			"Jira/Confluence integration: ticket-worker (ticket → branch → code → test → update), ticket-reviewer (review against acceptance criteria).",
			"MCP server definitions for Jira and Confluence (mcp-server-atlassian).",
			"Steva Đubre and Steve Trash personality agents (Serbian/English, multiple model variants).",
		},
	},
	{
		Version: "0.0.16",
		Date:    "2026-05-01",
		Highlights: []string{
			"All hosted provider wizards scan for live models: Anthropic, Gemini, Alibaba.",
			"Wizard flow: paste key → scan API → pick from numbered list → falls back to hardcoded menu if scan fails.",
			"Spec-driven development agent: requirement → design → tasks → code → verify workflow.",
			"Filesystem StateStore adapter: sessions persist to ~/.config/m/sessions/ as JSON.",
			"/spec slash command for discoverability.",
		},
	},
	{
		Version: "0.0.15",
		Date:    "2026-05-01",
		Highlights: []string{
			"Spec-driven development agent: requirement → design → tasks → code → verify workflow.",
			"Spec agent creates .m/spec.md, waits for design approval, executes tasks one by one, tests after each, tracks progress.",
			"Default agent gains spec-driven mode: say 'use spec workflow' for complex multi-step tasks.",
			"/spec slash command directs users to the spec agent.",
			"Filesystem StateStore adapter: sessions persist to ~/.config/m/sessions/ as JSON.",
		},
	},
	{
		Version: "0.0.14",
		Date:    "2026-05-01",
		Highlights: []string{
			"Diff preview: fs_write shows a unified diff before every write so you see exactly what changes.",
			"Undo stack: /undo reverts the last file write. Works in both TUI and REPL.",
			"Git builtin tool: status, diff, log, add, commit, branch, checkout, stash.",
			"Test runner tool: test_run executes tests and returns pass/fail with output for edit-test-fix loops.",
			"Project context auto-detection: scans for go.mod, package.json, Cargo.toml etc. and injects language/build/test into system prompt.",
			"TUI fs_write fix: auto-approves in TUI mode (bubbletea owns stdin), shows action in viewport.",
			"Default agent updated with git + test_run tools and development workflow instructions.",
		},
	},
	{
		Version: "0.0.12",
		Date:    "2026-05-01",
		Highlights: []string{
			"Theme system: built-in themes (matrix, default, minimal) + custom themes via ~/.config/m/theme.yaml.",
			"Matrix theme (default): green monochrome — green user text, dark green tools, red errors.",
			"`/theme` command: list themes or switch live (e.g. `/theme matrix`, `/theme default`).",
			"Responsive layout: header collapses on small terminals (<80 cols or <20 rows).",
			"Tool activity visible in TUI: → fs_list / ← 245 bytes shown in yellow while agent works.",
			"Visual message styling: user (bold colored), tool activity (faint), errors (red).",
			"Fix: v0.0.11 release asset conflict resolved (stale GitHub release deleted before retag).",
		},
	},
	{
		Version: "0.0.11",
		Date:    "2026-05-01",
		Highlights: []string{
			"Google Gemini provider — gemini-2.5-pro/flash/2.0-flash via OpenAI-compatible endpoint, 1M context.",
			"Alibaba Cloud (DashScope) provider — qwen-plus/turbo/max via OpenAI-compatible endpoint.",
			"`m config` command — interactive provider/model manager: scan available models, set default, add providers.",
			"Model discovery — queries Anthropic, OpenAI, Gemini, Alibaba, and Ollama APIs for available models.",
			"`/config` slash command — directs to `m config` from within chat.",
			"Wizard updated with 6 provider options: Ollama, Anthropic, OpenAI, Gemini, Alibaba, LiteLLM.",
			"Model pickers for Anthropic (sonnet/haiku/opus), Gemini (flash/pro), Alibaba (plus/turbo/max).",
			"Full provider/model label shown below the token box in TUI (no more truncation).",
			"Pricing and context windows for Gemini and Alibaba models.",
		},
	},
	{
		Version: "0.0.10",
		Date:    "2026-05-01",
		Highlights: []string{
			"Structured output post-processing: engine buffers JSON responses, extracts the `answer` field for display, validates JSON, rewrites history for hub consumption.",
			"Anthropic response-tool extraction: the synthetic `structured_response` tool call is intercepted and not executed as a real tool.",
			"Parallel tool execution: multiple tool calls in the same turn run concurrently, results collected in order.",
			"Ports/adapters layer: `internal/ports` defines ConfigSource, Secrets, and StateStore interfaces; `internal/adapters` provides an in-memory StateStore.",
			"Test coverage push: structured output tests, parallel execution test, validate/changelog/cwd cmd tests. Engine at 90.5%, adapters at 100%.",
		},
	},
	{
		Version: "0.0.9",
		Date:    "2026-05-01",
		Highlights: []string{
			"Engine-enforced structured output: `response_schema` in agent frontmatter constrains model output to valid JSON via provider-native mechanisms (OpenAI json_schema, Anthropic response-tool, Ollama format).",
			"Hub-and-spoke agents: hub orchestrator delegates to spoke-coder, spoke-reviewer, spoke-planner — spokes return structured JSON with citations, confidence, and caveats.",
			"429 retry with exponential backoff for Anthropic and OpenAI — respects Retry-After header, clear error after 3 retries.",
			"Anthropic model picker in wizard: choose sonnet (default), haiku, opus, or custom.",
			"Ollama output cap: num_predict defaults to 8192 when agent doesn't specify max_tokens (was using tiny model default).",
			"Assertive default agent system prompt forces tool use instead of asking the user.",
			"Current working directory injected into system prompt for all sessions.",
			"Structured-output skill for spoke agents.",
		},
	},
	{
		Version: "0.0.8",
		Date:    "2026-05-01",
		Highlights: []string{
			"`/model <provider/model>` — switch LLM mid-session, history preserved.",
			"`/compact` — truncate history to last 4 exchanges to free context window.",
			"Context window indicator (`ctx: N%`) shown next to the input line.",
			"Auto-continue on `max_tokens` — no more silent truncation of long responses.",
			"No artificial output cap — providers use their own limits (Anthropic 16K, OpenAI/Ollama server default).",
			"Custom default agent: set `default_agent: /path/to/agent.md` in config.yaml.",
			"Orchestrator agent: routes tasks to coder, reviewer, writer, devops, planner, or summarize.",
			"`fs_write` tool: create or patch files with user confirmation before every write.",
			"`fs_list` tool: list directories, recursive, skips .git/node_modules.",
			"New example agents: reviewer, writer, devops, local (Ollama).",
			"Updated docs: agents, changelog, quickstart, configuration.",
		},
	},
	{
		Version: "0.0.7",
		Date:    "2026-05-01",
		Highlights: []string{
			"`fs_write` tool: create files or patch existing ones with user confirmation before every write.",
			"`fs_list` tool: list directory contents, optionally recursive, skips .git/node_modules.",
			"Default agent now has full filesystem access: shell, fs_read, fs_write, fs_list.",
			"New example agents: reviewer (read-only code review), writer (docs/README), devops (CI/infra), local (Ollama, no API key).",
			"Updated coder, qwen-coder, summarize, planner agents with new tools.",
			"Fixed stale fs.read → fs_read in code-review skill.",
		},
	},
	{
		Version: "0.0.6",
		Date:    "2026-05-01",
		Highlights: []string{
			"Token count and estimated cost displayed in the TUI header between the M banner and system stats.",
			"Model name shown in the token box (provider/model, truncated to fit).",
			"Cost always visible ($0.0000 for local models).",
			"Persistent commands bar (/exit /reset /help) below the header.",
			"All three providers (Anthropic, OpenAI, Ollama) now emit usage events with input/output token counts.",
			"Cost estimation for Claude, GPT-4o/4.1, o3/o4 models.",
			"golangci-lint fixes: bodyclose, unused fields, capitalized error strings, empty branches.",
			"Release pipeline: macOS job runs after Linux to avoid goreleaser race.",
		},
	},
	{
		Version: "0.0.5",
		Date:    "2026-05-01",
		Highlights: []string{
			"Makefile with build, test, lint, cover, docker, and validate targets.",
			"golangci-lint config (.golangci.yml) for static analysis.",
			"Release workflow now gates on vet + lint + tests before publishing artifacts.",
			"Tests for userconfig (save/load/permissions/state), litellm provider registration, and version comparison logic.",
			"Fix stale .dockerignore and .gitignore referencing old `agent` binary name.",
		},
	},
	{
		Version: "0.0.4",
		Date:    "2026-05-01",
		Highlights: []string{
			"Fix v0.0.3 crash: TUI panicked on the second message with `strings: illegal use of non-zero Builder copied by value`. Bubbletea's `Update` is value-receiver, so the model is copied each call; the chat-history `strings.Builder` is now stored as a `*Builder` so the copies share one backing buffer. Regression test added.",
		},
	},
	{
		Version: "0.0.3",
		Date:    "2026-05-01",
		Highlights: []string{
			"New chat TUI: persistent header with the M banner on the left and a live system-stats table (CPU, RAM, GPU, Disk) on the right, scrolling chat viewport in the middle, input pinned at the bottom. Auto-falls-back to the line-oriented REPL when stdin/stdout/stderr aren't all terminals (so scripts and tests stay clean).",
			"Animated `thinking…` indicator while waiting for the model's first streamed token; disappears the moment text starts flowing.",
			"GPU stat shows `n/a` for now — Apple Silicon has no clean public API; Linux NVIDIA via `nvidia-smi` is planned.",
			"Stats refresh once per second via `gopsutil`.",
		},
	},
	{
		Version: "0.0.2",
		Date:    "2026-05-01",
		Highlights: []string{
			"Ollama daemon detection: polls localhost:11434 for up to 15 s and falls back to starting `ollama serve` as a background child when `brew services` doesn't bring it up.",
			"`brew services start ollama` failures now print their stderr instead of being silently swallowed.",
			"Qwen tag picker no longer hardcodes invalid `:Nb` sizes. Defaults to the always-valid `qwen3-coder` tag, with a `qwen2.5-coder:7b` fallback and a custom-tag option.",
		},
	},
	{
		Version: "0.0.1",
		Date:    "2026-05-01",
		Highlights: []string{
			"Default chat: type `m` to talk to an embedded agent — no arguments required.",
			"First-run setup wizard with four backends: Ollama+Qwen3-Coder, Anthropic, OpenAI, LiteLLM.",
			"Auto-installs Ollama (brew/curl) and libsecret (apt/dnf/pacman/apk) on demand, with explicit confirmation.",
			"API keys stored in the OS keychain (macOS Keychain, Linux libsecret) — never in plain config.",
			"GitHub Actions release pipeline: macOS .pkg installer and Linux .deb on every `vX.Y.Z` tag.",
			"`m init` re-runs the wizard; `M_MODEL=provider/model m` overrides the configured model for one session.",
			"`m changelog` prints the full version history.",
		},
	},
}

func newChangelogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "changelog",
		Short: "Print the m release history",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			printReleases(cmd.OutOrStdout(), releases, "")
			return nil
		},
	}
}

// showReleaseNotesIfNeeded compares the current build's Version against
// the last version whose notes we showed the user, prints anything new,
// and persists the bump. Failures here are non-fatal — release notes are
// nice-to-have and we never want them blocking the user from chatting.
//
// Skips entirely when Version is "dev" (an unreleased build), so source
// builds don't pollute the state file.
func showReleaseNotesIfNeeded(out io.Writer, current string) {
	if current == "" || current == "dev" {
		return
	}
	st, err := userconfig.LoadState()
	if err != nil && !userconfig.IsNotExist(err) {
		// Treat read failures as "first install" to avoid swallowing notes;
		// SaveState below will overwrite the bad file on the way out.
		st = nil
	}

	var unseen []release
	header := ""
	if st == nil || st.LastSeenVersion == "" {
		// First install on this machine: show everything.
		unseen = releases
		header = fmt.Sprintf("Welcome to m %s. Highlights:\n", current)
	} else {
		for _, r := range releases {
			if versionLess(st.LastSeenVersion, r.Version) {
				unseen = append(unseen, r)
			}
		}
		if len(unseen) > 0 {
			header = fmt.Sprintf("What's new in m since %s:\n", st.LastSeenVersion)
		}
	}

	if len(unseen) == 0 {
		// Even with nothing to show, advance the marker so a future
		// release notices the bump correctly.
		_ = userconfig.SaveState(&userconfig.State{LastSeenVersion: current})
		return
	}

	printReleases(out, unseen, header)
	_ = userconfig.SaveState(&userconfig.State{LastSeenVersion: current})
}

func printReleases(out io.Writer, rs []release, header string) {
	if header != "" {
		fmt.Fprintln(out, header)
	}
	for i, r := range rs {
		if i > 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprintf(out, "v%s — %s\n", r.Version, r.Date)
		for _, h := range r.Highlights {
			fmt.Fprintf(out, "  • %s\n", h)
		}
	}
	fmt.Fprintln(out)
}

// versionLess reports a < b for SemVer-shaped strings. Pre-release tags
// (-rc1, +meta) are stripped before comparison; we don't model them
// because the project tags releases as plain MAJOR.MINOR.PATCH.
func versionLess(a, b string) bool {
	pa, errA := parseVersion(a)
	pb, errB := parseVersion(b)
	if errA != nil || errB != nil {
		// Fall back to string compare so a malformed entry doesn't crash
		// the wizard — worst case the user sees notes once more than
		// they should.
		return a < b
	}
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func parseVersion(v string) ([3]int, error) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 3 {
		return [3]int{}, errors.New("version must be MAJOR.MINOR.PATCH")
	}
	var out [3]int
	for i, p := range parts {
		// Trim pre-release / build metadata: "1.0.0-rc1" → "1.0.0".
		if cut := strings.IndexAny(p, "-+"); cut >= 0 {
			p = p[:cut]
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, fmt.Errorf("part %d %q: %w", i, p, err)
		}
		out[i] = n
	}
	return out, nil
}
