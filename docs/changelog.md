---
title: Changelog
---

[← Docs home](./)

# Changelog

Released versions of `m`, newest first. The same content is shown by
the CLI's built-in `m changelog` command and on first run after each
upgrade.

The source of truth is [`cmd/m/releases.go`][src] in the repo; this
page is kept in sync.

[src]: https://github.com/subzone/m/blob/main/cmd/m/releases.go

---

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
  original line-oriented REPL. No surprises in non-interactive contexts.
- **`thinking…` spinner** — animated indicator while waiting for the
  model's first streamed token; disappears the moment text starts
  flowing.
- **GPU = `n/a`** for now. Apple Silicon has no clean public API and
  shelling out to `powermetrics` needs sudo. Linux NVIDIA via
  `nvidia-smi` is on the roadmap.
- **Stats refresh** at 1 Hz via `gopsutil`. CPU is overall system
  usage; RAM is overall used percent; Disk is `/` used percent. No
  per-process metrics in this release.

## v0.0.2 — 2026-05-01

- **Ollama daemon detection** — `m init` now polls `localhost:11434`
  for up to 15 s with exponential backoff and falls back to starting
  `ollama serve` as a background child when `brew services` doesn't
  bring it up. Previously the wizard gave up after 2 seconds.
- **Visible brew failures** — `brew services start ollama` errors
  are no longer silently swallowed; stderr is printed.
- **Valid Qwen tags** — replaced the made-up `qwen3-coder:7b/14b/30b`
  size menu with options that actually exist on Ollama's library.
  Default is `qwen3-coder` (latest tag), with a `qwen2.5-coder:7b`
  fallback and a custom-tag option that links to the Ollama library
  page.

## v0.0.1 — 2026-05-01

Initial public release.

- **Default chat** — type `m` to talk to an embedded agent. No
  arguments required.
- **Setup wizard** — first-run picker with four backends:
  Ollama+Qwen3-Coder, Anthropic, OpenAI, LiteLLM. Auto-installs
  Ollama (`brew install ollama` / `curl https://ollama.com/install.sh
  | sh`) and `libsecret` (`apt`/`dnf`/`pacman`/`apk`) on demand with
  explicit confirmation.
- **Keychain-backed key storage** — API keys stored in macOS
  Keychain or Linux libsecret. Never in plain config.
- **Tag-driven release pipeline** — GitHub Actions builds a macOS
  `.pkg` and Linux `.deb` (amd64 + arm64) on every `vX.Y.Z` push.
- **`m init`** re-runs the wizard;
  **`M_MODEL=provider/model m`** overrides for one session;
  **`m changelog`** prints history.
