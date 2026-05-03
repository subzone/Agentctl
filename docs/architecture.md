---
title: Architecture
layout: default
nav_order: 10
---

# Architecture

`m` is a single-binary CLI built in Go. The architecture follows a
hexagonal (ports-and-adapters) pattern: the engine core has no knowledge
of specific providers, tools, or transports.

## High-level diagram

```
┌─────────────────────────────────────────────────────────────┐
│                        cmd/m (CLI)                          │
│  main · run · chat · validate · init · config · releases    │
│  tui · tui_stats · theme · spawn                            │
├─────────────────────────────────────────────────────────────┤
│                     internal/engine                          │
│  Session · Step · tool dispatch · structured output          │
│  auto-continue · parallel execution                         │
├──────────┬──────────┬──────────┬────────────────────────────┤
│  llm/    │  tools/  │  mcp/    │  config/                   │
│ Provider │  Tool    │  Client  │  Parser                    │
│ Registry │ Registry │  Manager │  Validator                 │
├──────────┴──────────┴──────────┴────────────────────────────┤
│                    internal/ports                            │
│  ConfigSource · Secrets · StateStore (interfaces)           │
├─────────────────────────────────────────────────────────────┤
│                   internal/adapters                          │
│  MemoryStore (in-memory) · [future: SQLite, Vault, k8s]    │
├─────────────────────────────────────────────────────────────┤
│                  internal/userconfig                         │
│  config.yaml · state.yaml · OS keychain                     │
└─────────────────────────────────────────────────────────────┘
```

## Provider architecture

Providers self-register via `init()` + `llm.Register()`. The engine
never imports a provider directly — it calls `llm.Resolve("provider/model")`
which returns a `Provider` interface.

```
llm.Resolve("anthropic/claude-sonnet-4-6")
    │
    ├── anthropic/  → Custom SSE (Messages API)
    ├── openai/     → OpenAI SSE (Chat Completions)
    ├── ollama/     → NDJSON streaming (/api/chat)
    ├── gemini/     → OpenAI-compat wrapper (WithCompat)
    ├── alibaba/    → OpenAI-compat wrapper (WithCompat)
    └── litellm/    → OpenAI-compat wrapper (WithCompat)
```

All LLM clients are stdlib-only (`net/http`, `bufio`, `encoding/json`).
No SDK dependencies.

### OpenAI-compat providers

Gemini, Alibaba, and LiteLLM are thin wrappers over the OpenAI adapter
with `WithCompat()` enabled. This flag disables OpenAI-specific features
(`stream_options`, `json_schema` response format) that compat endpoints
don't support.

## Engine loop

```
User message
    │
    ▼
┌─────────────┐
│ Session.Step │──→ Provider.Stream() ──→ Events channel
└─────┬───────┘
      │
      ▼
  ┌─────────────────────────────────────────┐
  │ Event loop:                              │
  │   EventText      → stream to Out         │
  │   EventToolCall  → collect tool calls    │
  │   EventUsage     → accumulate tokens     │
  │   EventDone      → check stop reason     │
  │   EventError     → return error          │
  └─────────┬───────────────────────────────┘
            │
            ▼
  stop_reason?
  ├── "end_turn"   → done, return
  ├── "tool_use"   → execute tools (parallel if multiple) → loop
  └── "max_tokens" → auto-inject "continue" → loop
```

### Structured output mode

When `ResponseSchema` is set in the agent frontmatter:

```
Provider receives schema
    │
    ├── OpenAI:    response_format.json_schema (native)
    ├── Anthropic: synthetic structured_response tool + tool_choice
    ├── Ollama:    format field with JSON schema
    └── Compat:    schema in system prompt only (no enforcement)
    │
    ▼
Engine buffers full response (no streaming to user)
    │
    ▼
Extract "answer" field → display to user
Store full JSON in history → hub sees structured data
```

## Hub-and-spoke pattern

```
User ──→ Hub agent
           │
           ├──→ delegate("spoke-coder", task)
           │       └── spoke runs with own model, tools, schema
           │       └── returns {answer, sources[], confidence, caveats[]}
           │
           ├──→ delegate("spoke-reviewer", task)
           │       └── same structured JSON response
           │
           └── Hub synthesizes with citations
               └── [spoke-coder: main.go:10-25] ...
```

Spokes run in parallel when the model requests multiple delegations
in the same turn.

## Tool system

```
tools.Registry
    │
    ├── shell        → /bin/sh -c (timeout, output cap)
    ├── fs_read      → read UTF-8 file (size cap)
    ├── fs_write     → create/patch with user confirmation
    ├── fs_list      → directory listing (recursive, skip .git)
    ├── delegate     → spawn subagent (depth-limited)
    └── [MCP tools]  → namespaced via ToolAdapter
```

`fs_write` gates every write through a `ConfirmFunc` callback. The CLI
layer provides the implementation (stdin prompt for REPL, auto-approve
for non-interactive).

## MCP integration

```
Agent frontmatter: mcp: [github]
    │
    ▼
mcp.Manager.Open()
    │
    ├── Spawn subprocess (stdio transport)
    ├── JSON-RPC handshake (initialize + notifications/initialized)
    ├── tools/list → ToolAdapter per tool
    └── tools/call → forwarded by ToolAdapter.Run()
```

Tools are namespaced: `github__create_issue` to avoid collisions.

The repo ships MCP server definitions for GitHub, Jira, and Confluence
in [`examples/mcp/`][mcp-examples]. Agents reference them by name:
`mcp: [jira, confluence]`. See [Jira & Confluence](jira-confluence.html)
for the full workflow.

[mcp-examples]: https://github.com/subzone/m/tree/main/examples/mcp

## File layout

```
cmd/m/
  main.go           CLI entry, default chat, applyConfig
  run.go            `m run` one-shot execution
  chat.go           `m chat` REPL + runChatWithDoc
  spawn.go          subagent spawner, buildAgentRuntime
  tui.go            bubbletea TUI model + rendering
  tui_stats.go      system stats (CPU/RAM/Disk via gopsutil)
  theme.go          theme system (matrix/default/minimal + custom)
  init.go           first-run wizard (6 providers)
  config.go         `m config` interactive manager + model discovery
  validate.go       `m validate` frontmatter linter
  releases.go       changelog data + release notes display

internal/
  engine/
    engine.go       Session, Step, Run, executeTools
    structured.go   structured output post-processing
  config/
    schema.go       AgentSpec, SkillSpec, ToolSpec, MCPServerSpec
    parser.go       YAML frontmatter parser
    validator.go    per-type + cross-ref validation
  llm/
    provider.go     Provider interface, Event, Request, Usage
    registry.go     Register + Resolve
    anthropic/      Anthropic Messages API + SSE + 429 retry
    openai/         OpenAI Chat Completions + SSE + 429 retry
    ollama/         Ollama /api/chat + NDJSON
    gemini/         Google AI Studio (OpenAI-compat)
    alibaba/        DashScope (OpenAI-compat)
    litellm/        LiteLLM proxy (OpenAI-compat)
  tools/
    tool.go         Tool interface, Registry, Builtins, Merge
    shell.go        ShellTool
    fsread.go       FSReadTool
    fswrite.go      FSWriteTool + ConfirmFunc
    fslist.go       FSListTool
    delegate.go     DelegateTool + SpawnFunc
  mcp/
    protocol.go     JSON-RPC types
    client.go       stdio JSON-RPC client
    manager.go      multi-server lifecycle
    tool.go         ToolAdapter (MCP → tools.Tool)
  ports/
    ports.go        ConfigSource, Secrets, StateStore interfaces
  adapters/
    memory.go       MemoryStore (in-memory StateStore)
  userconfig/
    config.go       config.yaml load/save
    state.go        state.yaml (release notes tracking)
    keychain.go     OS keychain abstraction
    keychain_darwin.go  macOS security CLI
    keychain_linux.go   Linux secret-tool
```

## Design principles

1. **Engine is provider-agnostic.** No provider-specific code in
   `internal/engine/` or `internal/tools/`.
2. **Stdlib-only LLM clients.** No SDK deps. Easy to debug and fork.
3. **Providers self-register.** `init()` + `llm.Register()`. Adding a
   provider means one new package + a blank import in `main.go`.
4. **Keys in the OS keychain.** Never in plaintext config files.
5. **Ports/adapters for extensibility.** The engine depends on
   interfaces, not concrete implementations.
6. **MD files are the source of truth.** Agents, skills, tools, MCP
   servers — all defined in Markdown with YAML frontmatter.
7. **Shell over native tools for DevOps.** Kubernetes, Terraform, Helm,
   and other infrastructure CLIs are accessed via the `shell` tool —
   not compiled into the binary. This avoids version coupling (e.g.
   client-go, terraform-exec) and keeps the binary small. For richer
   structured output, use MCP servers. See
   [DevOps patterns](devops-patterns.html).

## Next steps

- **HTTP/SSE MCP transport** — many MCP servers use HTTP, not stdio
- **SQLite StateStore adapter** — conversation persistence
- **Structured logging (slog)** — JSON logs for debugging
- **k8s operator** — CRDs + helm chart, using the ports layer
