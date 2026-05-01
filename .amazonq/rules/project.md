# m — Project Rules for Amazon Q

## Project Overview

`m` is an MD-driven agent CLI written in Go 1.26. Agents are plain Markdown
files with YAML frontmatter; the CLI runs them against 6 LLM backends.
Binary name is `m`, module path is `github.com/subzone/m`.

## Architecture

```
cmd/m/                  # cobra CLI entry point
  main.go               # root command, default chat, applyConfig
  run.go                # `m run` one-shot execution
  chat.go               # `m chat` REPL + TUI branching, slash commands
  spawn.go              # subagent spawner, buildAgentRuntime
  tui.go                # bubbletea TUI model + rendering
  tui_stats.go          # system stats (CPU/RAM/Disk via gopsutil)
  theme.go              # theme system (matrix/default/minimal + custom YAML)
  init.go               # first-run wizard (6 providers)
  config.go             # `m config` interactive manager + model discovery
  context.go            # project context auto-detection
  validate.go           # `m validate` frontmatter linter
  releases.go           # changelog data + release notes display

internal/
  engine/               # agent loop, tool dispatch, session, structured output
    engine.go           # Session, Step, Run, parallel executeTools, auto-continue
    structured.go       # structured output post-processing (buffer, extract, validate)
  config/               # MD frontmatter parser + schema + validator
    schema.go           # AgentSpec (with response_schema), SkillSpec, ToolSpec, MCPServerSpec
    parser.go           # YAML frontmatter parser (strict mode)
    validator.go        # per-type + cross-ref validation
  llm/                  # Provider interface + registry
    provider.go         # Provider, Event (Text/ToolCall/Usage/Done/Error), Request, Usage
    registry.go         # Register + Resolve
    anthropic/          # Messages API, SSE, 429 retry, response-tool for structured output
    openai/             # Chat Completions, SSE, 429 retry, json_schema response_format
    ollama/             # /api/chat, NDJSON, num_predict default 8192
    litellm/            # OpenAI-compat wrapper (WithCompat)
    gemini/             # OpenAI-compat wrapper for Google AI Studio (WithCompat)
    alibaba/            # OpenAI-compat wrapper for DashScope (WithCompat)
  tools/                # Tool interface, Registry, builtins
    tool.go             # Tool interface, Registry, Builtins(confirm, undo), Merge
    shell.go            # ShellTool (timeout, output cap)
    fsread.go           # FSReadTool (size cap, truncation)
    fswrite.go          # FSWriteTool (diff preview, user confirm, undo stack)
    fslist.go           # FSListTool (recursive, skip .git/node_modules)
    git.go              # GitTool (status, diff, log, add, commit, branch, checkout, stash)
    testrun.go          # TestRunTool (run tests, return pass/fail + output)
    delegate.go         # DelegateTool + SpawnFunc
  mcp/                  # MCP client (stdio JSON-RPC), manager, tool adapter
  ports/                # ConfigSource, Secrets, StateStore interfaces
  adapters/             # MemoryStore (in-memory StateStore)
  userconfig/           # ~/.config/m/config.yaml, OS keychain, state
examples/agents/        # 17 example agent MD files
```

### Hard Rules
- Nothing in `internal/engine/` or `internal/tools/` imports `client-go`,
  `controller-runtime`, or HTTP server libs.
- LLM clients are stdlib-only (`net/http`, `bufio`, `encoding/json`).
- Providers self-register via `init()` + `llm.Register()`.
- API keys live in the OS keychain, never in plaintext config files.
- OpenAI-compat providers (gemini, alibaba, litellm) use `WithCompat()`
  which disables `stream_options` and `json_schema` response_format.

## 6 Providers

| Provider | Adapter | Auth Env Var | Notes |
|---|---|---|---|
| anthropic | Custom SSE | ANTHROPIC_API_KEY | 429 retry, response-tool for structured output |
| openai | OpenAI SSE | OPENAI_API_KEY | 429 retry, native json_schema |
| ollama | NDJSON | (none) | num_predict=8192 default, format field for structured |
| gemini | OpenAI-compat | GEMINI_API_KEY | WithCompat, Google AI Studio endpoint |
| alibaba | OpenAI-compat | DASHSCOPE_API_KEY | WithCompat, DashScope endpoint |
| litellm | OpenAI-compat | LITELLM_API_KEY | WithCompat, custom base URL |

## 8 Builtin Tools

| Tool | Description |
|---|---|
| shell | Run shell commands via /bin/sh -c |
| fs_read | Read UTF-8 files (size cap) |
| fs_write | Create/patch files with diff preview + user confirmation + undo |
| fs_list | List directories (recursive, skip .git) |
| git | Git operations (status, diff, log, add, commit, branch, checkout, stash) |
| test_run | Run tests, return pass/fail + output for edit-test-fix loops |
| delegate | Spawn subagent (depth-limited, parallel execution) |

## Key Types

- `llm.Provider` — `Stream(ctx, Request) (<-chan Event, error)`
- `llm.Event` — EventText, EventToolCall, EventUsage, EventDone, EventError
- `llm.Request` — includes ResponseSchema for structured output
- `engine.Session` — multi-turn driver, usage tracking, SetModel, LastInputTokens
- `tools.Tool` — Name, Description, InputSchema, Run
- `tools.UndoStack` — Push/Pop for file write rollback
- `tools.ConfirmFunc` — gates fs_write (stdin prompt in REPL, auto-approve in TUI)
- `config.AgentSpec` — includes response_schema (any → JSON via ResponseSchemaJSON())

## Build & Test

```bash
make build          # CGO_ENABLED=0 static binary
make test           # go test ./...
make race           # go test -race ./...
make lint           # golangci-lint run (v2.x, go install)
make cover          # coverage report
make validate       # validate example agent docs
make docker         # docker build
```

## Slash Commands (TUI + REPL)

/exit /quit /reset /compact /undo /model <provider/model> /theme [name] /config /help

## TUI Features

- Header: M banner | token/cost box | system stats
- Full provider/model label below token box
- Commands bar + cwd display
- Context window % on input line (ctx: N%)
- Theme system: matrix (default), default, minimal + custom ~/.config/m/theme.yaml
- Tool activity visible (→ fs_list / ← 245 bytes) in yellow
- User messages bold colored, errors red
- Responsive layout (collapses on small terminals)
- Auto-approve fs_write in TUI (bubbletea owns stdin)

## CI/CD

`.github/workflows/release.yml` triggers on `v*` tags:
1. `gate`: go vet → golangci-lint (v2, via `go install`) → tests with -race
2. `linux`: goreleaser → .deb + tarballs
3. `macos` (after linux): universal binary → .pkg

## Release Process

1. Update `releases` slice in `cmd/m/releases.go`
2. Run `make lint && make race` locally
3. `git commit && git tag v0.0.X && git push origin main --tags`
4. If retag needed: delete GitHub release first, then retag

## Current State (v0.0.14)

- 11,936 lines production + 4,299 lines tests
- 15 packages, 17 example agents, 7.8 MB binary
- Engine 90.5%, adapters 100%, providers 79-100%, tools 59%
- 25 commits, 14 tags

### What's Next
- Spec-driven workflow (requirement → design → tasks → code → verify)
- Filesystem StateStore adapter (conversation persistence)
- Stabilization pass (test all 6 providers against live APIs)
- HTTP/SSE MCP transport
- Multi-file atomic edits
