# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1] - 2026-05-14

### Added
- `m run --ci` mode with default 15m timeout and JSON event output support (`--output json`)
- `m run --timeout` and `m run --output text|json` flags
- Explicit `m run` exit codes for policy violations and timeout paths
- Inline policy engine (`policy.rules`) with hard deny enforcement before tool execution
- Audit sink interfaces and event model (`internal/ports/audit.go`)
- Audit backends: no-op, file JSONL (+ optional per-event HMAC), Splunk HEC
- Audit batching layer (`batch_size`, `flush_interval`) for async sink writes

### Changed
- Engine tool execution pipeline now emits audit events around tool calls/results
- Chat and run session lifecycles emit audit `session_start` / `session_end` events

## [0.1.0] - 2026-05-09

### Added
- Desktop app (Wails + Svelte): streaming chat, agent switcher, file upload, cost display
- 8 themes in desktop UI: default, matrix, nord, dracula, gruvbox, tokyonight, catppuccin, solarized
- Desktop settings panel: provider, model, API key, visual theme picker
- Native macOS file picker (entitlements for user-selected file access)
- Agent switcher: `/agent` command in TUI + REPL, active agent shown in header
- `/retry`: re-send last message without retyping
- Progress timer: shows elapsed time every 5s for long-running tools
- Multi-line input: paste code blocks between `"""` delimiters
- `.m/config.yaml`: per-project agent/model override (walks up to repo root)
- `m pipe`: stdin/stdout pipeline mode
- `@file` context: auto-inline file content in prompts
- `m cost`: token usage and estimated cost for recent sessions
- `m diff`: show all uncommitted changes the agent made
- `m mcp setup/status/list`: automated MCP server deployment
- `m session list/export/delete`: full session management
- `m upgrade`: self-update command
- `m search`: fuzzy agent discovery
- `m run --yes` / `--dry-run`: CI/headless and validation modes
- `fs_write_multi`: atomic multi-file writes with rollback
- 42 bundled agents: hub-and-spoke Steva/Steve, web design, developer-hub
- Hub agents resolve spokes from bundled/registry (no clone needed)
- Trace spans + log file rotation

### Changed
- Version milestone: v0.0.x → v0.1.0 (first stable release)
- Binary size: ~8.4 MB CLI, ~18 MB desktop

## [0.0.35] - 2026-05-06

### Added
- `m pipe`: stdin/stdout mode for Unix pipelines (`cat log | m pipe "explain"`)
- `@file` context: reference files in prompts with `@path` (auto-inlined, no tool call)
- `m cost`: show token usage and estimated cost for recent sessions
- `m diff`: show all uncommitted changes the agent made (git diff + stat)
- `m mcp setup`: automated MCP server install, credential config, connectivity check
- `m mcp status`: show installed/missing state of all MCP servers
- `m mcp list`: list available MCP server definitions
- `m session list/export/delete`: full session management from CLI
- `developer-hub` agent: Jira → branch → code → test → PR → Confluence
- `steve-webdev` / `steva-webdev`: web design hub agents with code/design/review spokes
- `spoke-webdev-code/design/review`: specialized web development spokes
- `spoke-steva-code/infra`, `spoke-steve-code/infra`: language-specific spokes
- Steva/Steve rebuilt as hub-and-spoke orchestrators
- Hub agents resolve spokes from bundled/registry (no clone needed)
- `MCPServerSpec.Install` field for automated installation (pip/npm/brew)
- `userconfig.SaveAPIKeyByName/GetAPIKeyByName` for MCP secrets
- MCP `HealthCheck()` method for connectivity verification
- Engine benchmarks: Step, tokenCompact, estimateTokens
- Logging tests: full coverage for slog + Span
- 15 new functional tests for all new features

### Changed
- 42 bundled agents (was 32)
- `loadCompanionDocs` also scans agent registry dir for spokes
- `findAgentDoc` falls back to `resolveAgentPath` for spoke resolution

## [0.0.34] - 2026-05-05

### Added
- Default agent personality: "The Pragmatic Architect" — direct, FinOps-aware, Balkan-style
- `m run --yes` / `-y`: auto-approve tools for CI/headless execution
- `m run --dry-run`: validate agent config without calling the LLM
- `m search <query>`: fuzzy agent discovery by name, model, or path
- `m upgrade`: self-update command (brew/go install/manual)
- `fs_write_multi` tool: atomic multi-file writes with rollback on failure
- Trace spans: per-turn LLM timing, per-tool duration (`logging.Span`)
- Log file rotation: `~/.config/m/logs/` with 5-file cap
- `code_search` and `web_fetch` added to default agent tools

### Changed
- Default agent temperature lowered to 0.4 for precision
- Default agent now has fallback models configured

## [0.0.33] - 2026-05-05

### Added
- Bundled agents: all 32 example agents embedded in the binary via `go:embed`
- `m chat devops` / `m run reviewer` work immediately after install (no clone needed)
- On-demand extraction: agents extracted to `~/.config/m/agents/` on first use
- `m list` shows `(bundled)` agents even before extraction
- User edits preserved: extracted agents are never overwritten
- `examples/embed.go`: thin package exposing `examples.Agents` embed.FS
- `cmd/m/bundled.go`: extraction logic (bulk on init, on-demand per agent)

### Fixed
- Windows CI: force `bash` shell for gosec/govulncheck steps (PowerShell comma parsing)
- Windows tests: JSON path escaping (`jsonPath` helper), skip unix-only tests
- Windows integration tests: `.exe` suffix for built binary
- Windows userconfig tests: use `APPDATA` env var instead of `XDG_CONFIG_HOME`

## [0.0.32] - 2026-05-05

### Added
- Windows support: shell tool uses `cmd.exe`, Windows Credential Manager, `findstr` fallback
- Windows .exe in release artifacts (goreleaser)
- CI tests on `windows-latest` in addition to `ubuntu-latest`
- Structured logging: `internal/logging` package using stdlib slog
- Graceful shutdown: session auto-saved on Ctrl+C / SIGTERM
- Integration tests: 10 black-box CLI tests
- Coverage tests: dangerous commands, slash commands, markdown rendering
- TUI fix: ANSI-aware word wrap prevents garbled escape codes

### Changed
- cmd/m test coverage: 16% → 25%
- Shell timeout: uses `cmd.exe /c` on Windows, `/bin/sh -c` on Unix
- code_search: falls back to `findstr` on Windows when grep/rg unavailable

## [0.0.31] - 2026-05-04

### Added
- PII guardrails: scan outgoing messages for sensitive data
  - Detects: emails, phones, SSNs, credit cards, IPs, AWS keys, API keys, JWTs, private keys, passwords
  - Modes: `redact` (replace with placeholders), `warn` (show findings), `off`
  - Agent config: `pii_guard: redact`
  - Session toggle: `/pii` / `/pii off`
- Mermaid diagram skill: `examples/skills/mermaid-diagrams.md`
  - Teaches agents to produce flowcharts, sequence diagrams, C4, ERD, etc.
- SAST: gosec added to CI pipeline and golangci-lint config
- SCA: govulncheck added to CI pipeline
- GitHub secret scanning + push protection enabled

## [0.0.30] - 2026-05-04

### Added
- Update notifier: checks GitHub releases API once per day
- Shows dim notice if newer version available: `↑ update available: v0.0.29 → v0.0.30 (brew upgrade subzone/tap/m)`
- Non-blocking (goroutine), cached (24h), silent on errors
- Skipped when Version is "dev" (local builds)

## [0.0.29] - 2026-05-04

### Added
- REPL color: bold blue prompt, dim welcome/tips/shortcuts
- Help examples: `m chat --help` and `m run --help` show real usage examples
- Session summary: `/sessions` shows first user message for each saved session
- Tool output: shows "42 lines, 6196 bytes" or short output instead of raw byte count

## [0.0.28] - 2026-05-04

### Added
- Command shortcuts: `/x` `/r` `/c` `/u` `/m` `/t` `/s` `/h` for all slash commands
- TUI command bar: underlined shortcut letters for discoverability
- Welcome message: shortcuts listed on every session start
- `/m 3` works as shortcut for `/models 3`
- `/t dracula` works as shortcut for `/theme dracula`
- `/s fixing-auth` works as shortcut for `/save fixing-auth`

## [0.0.27] - 2026-05-04

### Added
- First-run wizard: visual completion summary with next steps box
- TUI onboarding: tips line on session start (models, themes, trust, save)
- REPL onboarding: tips line on session start (models, trust, save, debug)

## [0.0.26] - 2026-05-04

### Added
- `m new <name>`: scaffold a new agent .md file with boilerplate frontmatter
- `m doctor`: health check — config, API key, model reachability, tools (git, rg, grep)
- `m completion bash/zsh/fish/powershell`: shell completion scripts
- Named sessions: `/save fixing-auth-bug` creates a named snapshot

### Changed
- Error messages: API key errors now include fix instructions (`run m config` or `export ...`)

## [0.0.25] - 2026-05-04

### Added
- MCP HTTP transport: POST JSON-RPC to URL, get JSON-RPC response
- MCP SSE transport: POST JSON-RPC, receive response via Server-Sent Events
- `Transport` interface: stdio, HTTP, SSE all implement the same contract
- Agent registry: `m install ./agent.md` copies to `~/.config/m/agents/`
- Name-based run: `m run coder "task"` and `m chat coder` resolve by name
- Example MCP servers: datadog (HTTP), slack (SSE)

### Changed
- Shell timeout: 30s → 120s (terraform, docker build friendly)
- Undo stack capped at 20 entries (was unbounded)
- MCP HTTP/SSE removed from known gaps

## [0.0.24] - 2026-05-04

### Added
- `code_search` tool: codebase search with two modes
  - `text`: grep/ripgrep for pattern matching across source files
  - `symbol`: in-memory index of functions, types, classes, imports
  - Languages: Go, Python, JavaScript, TypeScript, Java, Ruby, Rust, Terraform, Shell
  - Index built lazily on first symbol search, cached for session
- Session history rotation: autosave backs up before overwriting, keeps last 10
- `/trust` command in TUI: auto-approve tools (dangerous commands still double-confirm)
- Dangerous command protection: 34 patterns (`rm -rf`, `kubectl delete`, etc.) always require double confirmation even in trust mode
- `code_search` added to all 31 example agents

### Changed
- Tool count: 8 → 9 built-in tools
- Known gaps updated: codebase RAG partially addressed by code_search

## [0.0.23] - 2026-05-04

### Added
- Fallback models: auto-switch on 429 rate limit. Added to all 30 example agents
- Per-agent `thinking_phrases`: customize spinner text per agent
- Markdown rendering in TUI: `**bold**` as terminal bold, `` `code` `` as dim, `##` headers as bold
- Fallback and thinking_phrases documented in README and docs/agents.html

### Changed
- Tool confirmation shows `key=value` instead of raw JSON
- Tool output lines get newlines before/after so they don't run inline with model text
- Continue prompt: "Agent worked on this for a while" instead of technical turn count
- Removed `(thinking)` text marker — TUI spinner handles thinking status
- TUI: version + copyright below M banner, tagline below stats box

## [0.0.22] - 2026-05-04

### Added
- Interactive error correction for tool failures: when a tool call fails,
  the REPL prompts the user to press Enter (retry) or type a hint that gets
  appended to the error sent back to the LLM, steering it away from loops.
- Anthropic prompt caching: system prompt and tool schemas are sent with
  `cache_control: ephemeral`, reducing input tokens and TTFT on every
  subsequent turn in a session.

### Fixed
- Pre-existing compile error: `chatState` forward reference and stale
  `TraceWriter` field in engine Config literal.
- Chat tests updated to use `chatState` instead of raw `*engine.Session`.
- Removed unused `stdinToolConfirm` and `isSafeTool` functions (lint).
- TUI: version + copyright displayed below M banner.
- All docs and GitHub Pages synced to v0.0.22.

## [0.0.21] - 2026-07-04

### Added
- `ConfirmFg` theme field for high-contrast y/n prompts (critical fix!)
- All 9 themes now have proper foreground + background for confirmation prompts

### Fixed
- **CRITICAL**: Confirmation prompts (y/n) now have MAXIMUM CONTRAST
  - Before: only background, text depended on terminal default (could be invisible!)
  - After: foreground + background always set, human eye can actually READ it
  - This was written by Steva Đubre himself - first self-made release!

### Note
- This is the first release where Steva Đubre (the agent persona) fixed himself.
- The confirmation prompt colors were terrible - who wrote that, a monkey?
- Now you can actually SEE when the agent asks "Allow shell command? [y/n]"

## [0.0.19] - 2026-07-04

### Added
- Homebrew install: `brew tap subzone/tap && brew install subzone/tap/m`
- Auto-update Homebrew formula on every release via CI
- SECURITY.md, CODE_OF_CONDUCT.md, CODEOWNERS
- Issue templates (bug report, feature request) and PR template
- CI workflow on every push/PR (vet, lint, race tests, build, validate agents)
- Dependabot for Go modules and GitHub Actions
- Branch protection, tag protection, repo topics
- `.goreleaser.yml` explicit config

## [0.0.18] - 2026-07-04

### Added
- Session persistence with AES-256-GCM encryption and autosave
- `/save`, `/sessions`, `/resume` slash commands
- `web_fetch` tool — fetch URLs and extract readable text (stdlib-only)
- `/models` command — numbered model picker with live API scanning
- `/themes` command — 9 built-in themes (nord, dracula, gruvbox, tokyonight, catppuccin, solarized)
- `m list` — agent discovery command
- Token-based context compaction with per-model context windows
- Reasoning model support (MiniMax-M2.5, DeepSeek-R1 thinking indicator)
- Alibaba token plan support (custom base URL, GLM-5, MiniMax-M2.5)
- Screenshot gallery on docs landing page
- SECURITY.md, CODE_OF_CONDUCT.md, issue/PR templates
- CI workflow on every push/PR (not just tags)
- Dependabot for Go modules and GitHub Actions

### Changed
- TUI tool confirmation: auto-approve read-only tools, y/n for destructive
- DashScope model filtering: 154 → 45 models (skip non-chat, deduplicate)
- Context compaction now token-based instead of message-count-based

### Fixed
- TUI scrolling jitter during streaming
- `/themes` command caught by `/theme` prefix handler
- Path traversal in EncryptedFileStore session IDs
- Data race in autosave goroutine (snapshot before launch)

## [0.0.17] - 2026-05-03

### Added
- Product landing page at subzone.github.io/Agentctl
- 7-page documentation site (EN + SR)
- DevOps agents: k8s-debug, terraform-plan, helm-deploy
- Jira/Confluence MCP integration and ticket agents
- Steva Đubre & Steve Trash personality agents
- GLM model pricing and context windows

## [0.0.16] - 2026-05-01

### Added
- Live model scanning for all hosted providers
- Wizard: paste key → scan API → pick from numbered list
- API key fallback: keychain first, then environment variable
- `/models` slash command (initial version)
- Delegate tool model override

## [0.0.15] - 2026-04-30

### Added
- Alibaba Cloud (DashScope) provider
- LiteLLM proxy provider
- Gemini provider via OpenAI-compat endpoint

## [0.0.14] - 2026-04-28

### Added
- Full-screen TUI with bubbletea
- Token/cost/context indicators
- Theme system (matrix, default, minimal)
- System stats (CPU/RAM/GPU/Disk)

[0.0.31]: https://github.com/subzone/Agentctl/compare/v0.0.30...v0.0.31
[0.0.30]: https://github.com/subzone/Agentctl/compare/v0.0.29...v0.0.30
[0.0.29]: https://github.com/subzone/Agentctl/compare/v0.0.28...v0.0.29
[0.0.28]: https://github.com/subzone/Agentctl/compare/v0.0.27...v0.0.28
[0.0.27]: https://github.com/subzone/Agentctl/compare/v0.0.26...v0.0.27
[0.0.26]: https://github.com/subzone/Agentctl/compare/v0.0.25...v0.0.26
[0.0.25]: https://github.com/subzone/Agentctl/compare/v0.0.24...v0.0.25
[0.0.24]: https://github.com/subzone/Agentctl/compare/v0.0.23...v0.0.24
[0.0.23]: https://github.com/subzone/Agentctl/compare/v0.0.22...v0.0.23
[0.0.22]: https://github.com/subzone/Agentctl/compare/v0.0.21...v0.0.22
[0.0.21]: https://github.com/subzone/Agentctl/compare/v0.0.20...v0.0.21
[0.0.19]: https://github.com/subzone/Agentctl/compare/v0.0.18...v0.0.19
[0.0.18]: https://github.com/subzone/Agentctl/compare/v0.0.17...v0.0.18
[0.0.17]: https://github.com/subzone/Agentctl/compare/v0.0.16...v0.0.17
[0.0.16]: https://github.com/subzone/Agentctl/compare/v0.0.15...v0.0.16
[0.0.15]: https://github.com/subzone/Agentctl/compare/v0.0.14...v0.0.15
[0.0.14]: https://github.com/subzone/Agentctl/releases/tag/v0.0.14
