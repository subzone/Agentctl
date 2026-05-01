# m — Project Rules for Amazon Q

## Project Overview

`m` is an MD-driven agent CLI written in Go 1.26. Agents are plain Markdown
files with YAML frontmatter; the CLI runs them against Anthropic, OpenAI,
Ollama, or LiteLLM backends. Binary name is `m`, module path is
`github.com/subzone/m`.

## Architecture

```
cmd/m/                  # cobra CLI: main, run, chat, validate, init, config, releases, tui
internal/
  engine/               # agent loop, tool dispatch, session, structured output
  config/               # MD frontmatter parser + schema + validator
  llm/                  # Provider interface + registry
    anthropic/          # Messages API, SSE streaming, stdlib-only
    openai/             # Chat Completions, SSE streaming, stdlib-only
    ollama/             # /api/chat, NDJSON streaming, stdlib-only
    litellm/            # thin wrapper over openai with custom base URL
    gemini/             # thin wrapper over openai for Google AI Studio
    alibaba/            # thin wrapper over openai for DashScope
  tools/                # Tool interface, Registry, builtins: shell, fs_read, fs_write, fs_list, delegate
  mcp/                  # MCP client (stdio JSON-RPC), manager, tool adapter
  userconfig/           # ~/.config/m/config.yaml, OS keychain (macOS/Linux), state
  ports/                # ConfigSource, Secrets, StateStore interfaces
  adapters/             # MemoryStore (in-memory StateStore)
examples/agents/        # example .md agent files (17 docs)
```

### Hard Rules
- Nothing in `internal/engine/` or `internal/tools/` imports `client-go`,
  `controller-runtime`, or HTTP server libs.
- LLM clients are stdlib-only (`net/http`, `bufio`, `encoding/json`) — no SDK deps.
- Providers self-register via `init()` + `llm.Register()`. Engine never imports
  a provider directly.
- API keys live in the OS keychain, never in plaintext config files.

## Dependencies

Only essential deps — keep it minimal:
- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — YAML parsing
- `golang.org/x/term` — terminal detection + password input
- `github.com/charmbracelet/bubbletea` + `bubbles` + `lipgloss` — TUI
- `github.com/shirou/gopsutil/v4` — system stats (CPU/RAM/Disk)

## Build & Test

```bash
make build          # CGO_ENABLED=0 static binary
make test           # go test ./...
make race           # go test -race ./...
make lint           # golangci-lint run (requires v2.x)
make cover          # coverage report
make validate       # validate example agent docs
make docker         # docker build
```

Go version: 1.26.1. Binary output: `./m`.

## Code Style

- Go standard formatting (gofmt/goimports).
- No SDK deps for LLM providers — use stdlib `net/http` + `encoding/json`.
- Errors from `fmt.Fprint*` to io.Writer are intentionally unchecked (status/log output).
- `defer resp.Body.Close()` in goroutines needs `//nolint:bodyclose` comment.
- Test helpers go in `testing_helpers_test.go` files.
- Scripted/fake providers in tests replay `[]llm.Event` turns.

## Key Types

- `llm.Provider` — interface with `Stream(ctx, Request) (<-chan Event, error)`
- `llm.Event` — streamed event: EventText, EventToolCall, EventUsage, EventDone, EventError
- `llm.Usage` — InputTokens + OutputTokens per response
- `engine.Session` — multi-turn conversation driver, accumulates usage
- `tools.Tool` — interface: Name, Description, InputSchema, Run
- `tools.Registry` — tool lookup, filter by allowlist, merge
- `config.Document` — parsed MD file with typed Spec (AgentSpec, SkillSpec, etc.)

## Frontmatter Schema

Agent MD files have YAML frontmatter with: name, type, version, model
(provider/model), tools, mcp, skills, subagents, powers, temperature,
max_tokens. See `internal/config/schema.go` for the full spec.

## TUI Layout

Header: M banner (left) | token/cost box (center) | system stats (right)
Commands bar: `/exit  /reset  /help`
Body: scrolling chat viewport
Footer: text input

Token box shows: Model, In, Out, Total, Cost.
Cost estimated from `modelPricing` map in `tui.go`.

## CI/CD

`.github/workflows/release.yml` triggers on `v*` tags:
1. `gate` job: go vet → golangci-lint (v2, installed via `go install`) → tests with -race
2. `linux` job: goreleaser → .deb + tarballs
3. `macos` job (after linux): universal binary → .pkg

golangci-lint must be installed via `go install` because pre-built binaries
are compiled with Go 1.24 which can't analyze Go 1.26 code.

## Release Process

1. Update `releases` slice in `cmd/m/releases.go` (prepend new entry)
2. Run `make lint && make race` locally
3. `git commit && git tag v0.0.X && git push origin main --tags`

## Current State (v0.0.6)

Milestones M1–M8 complete. See PLAN.md for full details.
- Config/parser, validation, multi-provider LLM, tool loop, MCP stdio,
  subagent delegation, chat REPL, TUI with stats/tokens/cost, setup wizard,
  keychain storage, Dockerfile, release pipeline.

### Test Coverage
- engine: 90%, llm registry: 93%, providers: 81-86%, mcp: 78%,
  tools: 78%, config: 51%, userconfig: 50%, cmd/m: ~21%

### What's Next (from PLAN.md)
- `git` builtin tool (go-git)
- HTTP/SSE MCP transport
- `ports/adapters` layer (ConfigSource, Secrets, TaskQueue, StateStore)
- Structured logging (slog JSON)
- k8s operator + helm chart (later)
