---
title: Changelog
layout: default
nav_order: 12
---

# Changelog

Released versions of `m`, newest first. The same content is shown by
the CLI's built-in `m changelog` command and on first run after each
upgrade.

The source of truth is [`cmd/m/releases.go`][src] in the repo; this
page is kept in sync.

[src]: https://github.com/subzone/m/blob/main/cmd/m/releases.go

---

## v0.0.12 — 2026-05-01

- **Theme system** — built-in themes (matrix, default, minimal) + custom
  themes via `~/.config/m/theme.yaml`.
- **Matrix theme** is the new default: green monochrome.
- **`/theme`** command: list themes or switch live.
- **Responsive layout** — header collapses on small terminals.
- **Tool activity visible in TUI** — `→ fs_list` / `← 245 bytes`.
- **Visual message styling** — user (bold), tools (faint), errors (red).
- **`/config` and `/theme`** work in both TUI and REPL.
- **Gemini/Alibaba/LiteLLM fix** — `WithCompat()` disables
  OpenAI-specific `stream_options` that broke tool calling.

## v0.0.11 — 2026-05-01

- **Google Gemini provider** — gemini-2.5-pro/flash via OpenAI-compat.
- **Alibaba Cloud provider** — qwen-plus/turbo/max via DashScope.
- **`m config` command** — interactive provider/model manager with
  model discovery (scans provider APIs).
- **Wizard updated** to 6 providers.
- **Full provider/model label** below the token box in TUI.

## v0.0.10 — 2026-05-01

- **Structured output post-processing** — engine buffers JSON, extracts
  `answer` field for display, validates, rewrites history for hub.
- **Anthropic response-tool extraction** — synthetic tool call
  intercepted, not executed.
- **Parallel tool execution** — multiple tool calls run concurrently.
- **Ports/adapters layer** — ConfigSource, Secrets, StateStore
  interfaces + MemoryStore adapter.

## v0.0.9 — 2026-05-01

- **Engine-enforced structured output** — `response_schema` in agent
  frontmatter constrains model to valid JSON via provider-native
  mechanisms.
- **Hub-and-spoke agents** — orchestrator + spoke-coder/reviewer/planner
  with citations and confidence levels.
- **429 retry with backoff** for Anthropic and OpenAI.
- **Anthropic model picker** in wizard.
- **Ollama output cap** — `num_predict` defaults to 8192.
- **Assertive system prompt** forces tool use.
- **cwd injected** into system prompt.

## v0.0.8 — 2026-05-01

- **`/model <provider/model>`** — switch LLM mid-session. History is
  preserved; the next turn uses the new provider. Works in both TUI
  and line REPL.
- **`/compact`** — manually truncate history to the last 4 exchanges.
  Frees context window when it gets crowded.
- **Context window indicator** — `ctx: N%` shown next to the input
  line. Based on `input_tokens` from the last call and known context
  window sizes (200K for Claude, 128K for GPT-4o, 1M for GPT-4.1).
- **Auto-continue on `max_tokens`** — when the model's output is
  truncated, the engine automatically sends a "continue" message so
  the response picks up where it left off.
- **No artificial output cap** — `defaultMaxTokens` is now 0 (provider
  decides). Anthropic falls back to 16384; OpenAI and Ollama use their
  server-side defaults.
- **Custom default agent** — set `default_agent: /path/to/agent.md` in
  `config.yaml` to replace the built-in default for bare `m`.
- **Orchestrator agent** — new `examples/agents/orchestrator.md` routes
  tasks to coder, reviewer, writer, devops, planner, or summarize.
- **`fs_write` tool** — create or patch files with user confirmation.
- **`fs_list` tool** — list directories, recursive, skips `.git`.
- **New example agents** — reviewer, writer, devops, local.
- **Updated docs** — agents page, changelog, quickstart, configuration.

## v0.0.7 — 2026-05-01

- **`fs_write` tool** — create files or patch existing ones. Every
  write is gated by a user confirmation prompt (`y/N`). Supports
  `mode=create` (full file) and `mode=patch` (find-and-replace).
- **`fs_list` tool** — list directory contents, optionally recursive.
  Skips `.git`, `node_modules`, `__pycache__`. Capped at 500 entries.
- **Default agent upgraded** — now has `shell`, `fs_read`, `fs_write`,
  `fs_list`. You can ask it to read, explore, and edit files out of
  the box.
- **New example agents** — `reviewer` (read-only code review),
  `writer` (docs/README), `devops` (CI/infra), `local` (Ollama, no
  API key).
- **Updated agents** — `coder`, `qwen-coder`, `summarize`, `planner`
  all include the new tools.
- **Fixed** stale `fs.read` → `fs_read` in code-review skill.

## v0.0.6 — 2026-05-01

- **Token count + cost in TUI header** — new box between the M banner
  and system stats showing Model, In, Out, Total tokens, and estimated
  Cost.
- **Model name visible** in the token box (provider/model, truncated).
- **Cost always shown** — `$0.0000` for local/unknown models, real
  estimate for paid APIs (Claude, GPT-4o/4.1, o3/o4).
- **Persistent commands bar** — `/exit  /reset  /help` always visible
  below the header.
- **Usage events** — all three providers (Anthropic, OpenAI, Ollama)
  now emit `EventUsage` with input/output token counts.
- **golangci-lint fixes** — bodyclose, unused fields, capitalized
  error strings, empty branches.
- **Release pipeline** — macOS job runs after Linux to avoid
  goreleaser race; golangci-lint installed via `go install` (pre-built
  binaries are Go 1.24, can't lint Go 1.26 code).

## v0.0.5 — 2026-05-01

- **Makefile** with `build`, `test`, `lint`, `cover`, `docker`,
  `validate` targets.
- **golangci-lint config** (`.golangci.yml`) for static analysis.
- **CI gate** — release workflow now runs vet + lint + tests before
  publishing artifacts.
- **New tests** for `userconfig` (save/load/permissions/state),
  `litellm` provider registration, and version comparison logic.
- **Fixed** stale `.dockerignore` and `.gitignore` referencing old
  `agent` binary name.

## v0.0.4 — 2026-05-01

- **Crash fix.** v0.0.3 panicked on the second user message with
  `strings: illegal use of non-zero Builder copied by value`. The TUI
  model's chat-history `strings.Builder` was stored by value, so
  bubbletea's value-receiver `Update` copied it on every call — and a
  non-zero Builder panics on the next WriteString. Now stored as a
  `*Builder` so all copies share one backing buffer. Regression test
  added.

## v0.0.3 — 2026-05-01

- **Chat TUI** — bare `m` now opens a full-screen TUI when launched in
  a real terminal. Layout: persistent header with the M banner on the
  left and a live system-stats table on the right (CPU, RAM, GPU,
  Disk), a scrolling chat viewport, and a pinned input at the bottom.
- **Auto-fallback to line REPL** — when stdin/stdout/stderr aren't
  all TTYs (piped input, scripts, CI), `m` skips the TUI and uses the
  original line-oriented REPL.
- **`thinking…` spinner** — animated indicator while waiting for the
  model's first streamed token.
- **GPU = `n/a`** for now. Apple Silicon has no clean public API.
- **Stats refresh** at 1 Hz via `gopsutil`.

## v0.0.2 — 2026-05-01

- **Ollama daemon detection** — polls `localhost:11434` for up to 15 s
  with backoff, falls back to launching `ollama serve` as a background
  child.
- **Visible brew failures** — `brew services start ollama` errors are
  no longer silently swallowed.
- **Valid Qwen tags** — replaced invalid size tags with options that
  exist on Ollama's library.

## v0.0.1 — 2026-05-01

Initial public release.

- **Default chat** — type `m` to talk to an embedded agent.
- **Setup wizard** — four backends: Ollama+Qwen3-Coder, Anthropic,
  OpenAI, LiteLLM.
- **Keychain-backed key storage** — macOS Keychain or Linux libsecret.
- **Tag-driven release pipeline** — macOS `.pkg` and Linux `.deb`.
- **`m init`**, **`M_MODEL=...`**, **`m changelog`**.
