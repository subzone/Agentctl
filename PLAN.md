# Agent — Build Plan

A single-binary, MD-driven CLI agent for infrastructure, code, and automation
work. Built in Go for cross-platform reach and small Docker footprint. Designed
CLI-first, with ports/adapters layering so a k8s operator + helm chart can be
added later without rewriting the engine.

## Vision (recap)

- **One binary, multiple modes**: `agent run` / `agent chat` / (later)
  `agent worker` / `agent operator`.
- **MD files are the source of truth**: agents, skills, tools, MCP servers all
  defined in markdown with YAML frontmatter.
- **Multi-LLM**: Anthropic, OpenAI, Ollama at minimum; per-agent model choice
  in frontmatter.
- **Hub-and-spoke**: agents can delegate to subagents (separate context, own
  model).
- **MCP-first extensibility**: Jira, Confluence, Bitbucket, GitHub, Azure
  DevOps, etc. all plug in as MCP servers — no custom integrations.
- **Small footprint**: distroless image, ~30 MB, multi-arch (amd64 + arm64).
- **K8s-ready (later)**: ports layer keeps engine free of `client-go`; operator
  + CRDs + helm chart slot in without engine changes.

## Architecture (target)

```
cmd/agent/                        # cobra entry, subcommand wiring
internal/
  engine/                         # agent loop, tool dispatch, subagent
  config/                         # MD frontmatter parser + schema
  llm/                            # Provider interface
    anthropic/  openai/  ollama/
  tools/                          # builtin: shell, fs, http, git, delegate
  mcp/                            # MCP client (stdio first)
  ports/                          # ConfigSource, Secrets, TaskQueue, StateStore
  adapters/                       # fs/, sqlite/, env/
  log/                            # slog JSON
examples/
  agents/  skills/  mcp/
```

**Hard rule**: nothing in `internal/engine/` or `internal/tools/` imports
`client-go`, `controller-runtime`, or HTTP server libs. That's how the k8s
door stays open.

## Frontmatter schema (v1)

```yaml
---
name: hello-agent                 # required, unique
type: agent                       # agent | skill | tool | mcp_server
description: "..."                # optional, human-readable
version: 1                        # schema version

# Agent-only
model: anthropic/claude-sonnet-4-6
tools: [shell, fs.read]           # builtin or MCP tools (allowlist)
mcp: [github, jira]               # MCP servers reachable
skills: [code-review]             # skill MDs to compose into prompt
subagents: [planner]              # delegate-able agents
powers: [filesystem-read, network] # coarse permission grants
temperature: 0.7
max_tokens: 4096

# MCP-server-only
transport: stdio                  # stdio | sse | http
command: ["mcp-jira"]
env:
  JIRA_URL: ...
  JIRA_TOKEN: ${secret:jira-token}
tool_prefix: jira

# Tool-only
runtime: builtin                  # builtin | mcp | shell
---
# Body = system prompt / instructions / docs
```

## Milestones

Each milestone is small and demoable. We mark them done as we ship.

- [x] **M1 — Bootstrap** ✅ *done*: repo, go.mod, cobra skeleton, MD frontmatter
      parser, schema validation, `agent validate` works against `examples/`.
      Binary: 4.3 MB static (`CGO_ENABLED=0`). Tests + vet clean.
      - [x] Repo / go.mod / .gitignore
      - [x] `internal/config` schema (Agent / Skill / Tool / MCPServer)
      - [x] Frontmatter parser with strict-field decoding
      - [x] Validator: per-type rules + cross-ref + duplicate detection
      - [x] Unit tests (parser + validator)
      - [x] cobra root + `validate` + `run`/`chat` stubs
      - [x] Example MDs: hello, coder, planner, code-review skill, github MCP
      - [x] End-to-end: `agent validate examples/` → 5 docs, 0 issues
      - [x] Negative path verified (bad name, missing model, dangling refs caught)
- [x] **M2 — First LLM call** ✅ *done*: `agent run hello.md "hi"` →
      Anthropic Messages API streaming round-trip, no tools, deltas printed
      to stdout as they arrive. Stdlib-only (`net/http`, `encoding/json`,
      `bufio`); no SDK dep.
      - [x] `internal/llm` provider interface + factory registry
            (`provider/model` resolution)
      - [x] `internal/llm/anthropic` Messages API client with SSE parser
            (`content_block_delta`, `message_delta`, `message_stop`, `error`)
      - [x] `agent run <agent.md> [task]` loads + validates the doc, builds
            request from frontmatter (model, temperature, max_tokens, body
            as system prompt), streams reply to stdout
      - [x] Tests: registry resolution, SSE parser happy + error paths,
            end-to-end Stream against `httptest.Server`
      - [x] Static binary still builds (`CGO_ENABLED=0`); `go vet` + tests
            clean
- [x] **M3 — Tool loop** ✅ *done*: `agent run` now drives a multi-turn
      loop. `llm.Message` upgraded to typed content blocks (text /
      tool_use / tool_result); Anthropic adapter sends/parses tool calls;
      `internal/tools` provides the Tool interface, registry, and builtin
      `shell` + `fs.read`; `internal/engine` runs the loop until the model
      stops naturally (or hits MaxTurns).
      - [x] `llm.ContentBlock` + `llm.ToolSchema`; `EventToolCall` event
      - [x] Anthropic SSE parser handles `content_block_*` (text +
            tool_use), `input_json_delta` accumulation, stop_reason
            propagation
      - [x] Anthropic request payload sends content-block messages and
            `tools` array
      - [x] `internal/tools`: Tool interface, Registry (Get/All/Filter/Run),
            `Builtins()`; `ShellTool` (timeout, output cap, exit-code in
            output), `FSReadTool` (size cap, truncation marker)
      - [x] `internal/engine`: `Run` drives the turn loop, dispatches
            tools, threads results back into the conversation, surfaces
            tool indicators on Status writer
      - [x] `cmd/agent/run` filters builtins by agent allowlist, warns on
            missing tools, wires SIGINT/SIGTERM
      - [x] Tests: tools (shell/fs.read happy + error + truncation),
            engine multi-turn (script provider) including tool-error
            propagation and MaxTurns cap, Anthropic SSE tool_use parsing
            + payload shape
- [x] **M4 — Multi-provider** ✅ *done*: `model:` of the form
      `openai/...` or `ollama/...` now routes to the matching adapter.
      Both adapters reuse the engine's content-block + tool-call vocabulary
      and normalize stop reasons so the loop in `internal/engine` is
      provider-agnostic.
      - [x] `internal/llm/openai`: Chat Completions client, system prompt
            hoisted into a leading `system` message, content blocks
            flattened (assistant text+tool_calls into one msg; user
            tool_results into separate `role:"tool"` msgs); SSE parser
            accumulates streamed `tool_calls` by index across delta chunks;
            stop reasons normalized
            (`stop`→`end_turn`, `tool_calls`→`tool_use`, `length`→`max_tokens`)
      - [x] `internal/llm/ollama`: `/api/chat` client over NDJSON streaming
            (no SSE), `OLLAMA_HOST` with scheme normalization, options
            (temperature, num_predict) when set, synthetic tool-call IDs
            since Ollama doesn't issue them; tool-call presence forces
            `tool_use` stop reason regardless of `done_reason`
      - [x] Tests: parser happy/error/tool-call cases, `httptest`
            end-to-end with payload assertions including round-trip of
            tool_use → tool_result, normalize tables
      - [x] `cmd/agent/main` blank-imports both packages; smoke run
            confirms `openai/...` and `ollama/...` reach the right
            provider
- [x] **M5 — MCP client (stdio)** ✅ *done*: `agent run` now spawns the
      MCP servers an agent references in `mcp:`, runs the JSON-RPC
      handshake, lists tools, and exposes them to the engine under a
      `<prefix>__<name>` namespace. Stdio transport only; HTTP/SSE
      logged-and-skipped per the milestone.
      - [x] `internal/mcp` JSON-RPC client over newline-delimited JSON,
            atomic id assignment, concurrent calls multiplexed by id,
            ctx-cancel and connection-close paths
      - [x] `Initialize` (with `notifications/initialized`), `ListTools`,
            `CallTool` (text content concatenated; `isError` surfaced)
      - [x] `ToolAdapter` implements `tools.Tool`, sanitizes names to fit
            Anthropic/OpenAI's `^[a-zA-Z0-9_-]{1,64}$` regex, namespaces
            with `__`
      - [x] `Manager.Open` enforces `allowed_agents`, skips non-stdio
            transports with a warning, tears down partial state on error,
            `Close` shuts every spawned server down
      - [x] `cmd/agent/run` resolves the agent's project root, loads
            companion docs, opens the manager, merges builtins + MCP
            tools, applies the `tools:` allowlist
      - [x] Renamed builtin `fs.read` → `fs_read` (dots are illegal in
            provider tool-name regexes); coder.md updated
      - [x] Tests: handshake + ListTools + CallTool over `io.Pipe` fakes,
            concurrent-id correlation with reverse-order replies, ctx
            cancel during call, `isError` translation, manager
            allowed_agents enforcement, transport skip; `TestMain`-style
            re-exec spawns the test binary as a real subprocess MCP
            server fixture
      - [x] End-to-end smoke: `agent run examples/agents/coder.md` spawns
            `mcp-server-github`, handshakes, enumerates 26 tools
- [x] **M6 — Subagent delegation** ✅ *done*: `delegate(name, task)` is a
      builtin tool; agents listing `subagents:` get it auto-included.
      Subagents run with their own model, system prompt, and tool
      allowlist; their tokens stream live to the user prefixed with
      `[<name>] ` and are also captured to feed back to the parent as a
      tool_result. Recursion is capped at depth 2 (hub → sub → sub-sub),
      enforced both by spawner check and by omitting the delegate tool
      from agents already at the depth boundary.
      - [x] `internal/tools/delegate.go`: `DelegateTool` with `SpawnFunc`
            indirection, enum schema listing the agent's `subagents:`
            names, defense-in-depth name check
      - [x] `cmd/agent/spawn.go`: `spawner` (depth-tracking),
            `buildAgentRuntime` (shared by hub and subagents — composes
            builtins + per-agent MCP + delegate, applies allowlist),
            `prefixWriter` for tagged streaming output
      - [x] `cmd/agent/run.go` refactor: hub goes through the same
            `buildAgentRuntime`; companion docs loaded once for both MCP
            and subagent resolution
      - [x] Per-subagent MCP: each subagent's `mcp:` opens its own
            servers, scoped to its `allowed_agents`, torn down on return
      - [x] Tests: delegate tool (success / unknown name / missing fields
            / spawn error / enum schema), prefixWriter (single line,
            multi-line, sub-line streaming, trailing partial),
            `findAgentDoc`, `projectRoot` (sibling-subdir + flat),
            spawner end-to-end with scripted provider, depth-limit
            refusal, full hub → engine → delegate → subagent → result
            round-trip with two scripted providers
      - [x] `examples/agents/planner.md` updated `fs.read` → `fs_read`
- [x] **M7 — Chat REPL** ✅ *done*: `agent chat <agent.md>` is an
      interactive REPL with streaming, persistent multi-turn history, and
      exchange-aware truncation. Uses the same agent runtime (builtins +
      MCP + delegate) as `agent run`, so chat sessions inherit tool use
      and subagent delegation.
      - [x] `engine.Session` extracted from `Run`: `Step` advances one
            user turn (with the full inner tool loop); `Reset` clears
            history; `Truncate(N)` keeps the last N user-initiated
            exchanges, never splitting an assistant tool_use from its
            tool_result; `Messages()` returns a defensive copy
      - [x] `engine.Run` becomes a thin `NewSession + Step` wrapper —
            existing one-shot callers unchanged
      - [x] `cmd/agent/chat.go`: REPL with `» ` prompt, slash commands
            (`/exit`, `/quit`, `/reset`, `/help`, unknown-`/x` notice),
            input goroutine + select-on-ctx so SIGINT cancels both an
            in-flight Step and a blocked prompt read, EOF-clean exit,
            history truncation at 8 exchanges per turn
      - [x] `cmd/agent/stubs.go` removed (chat was the last stub)
      - [x] Tests: `Session` two-step history threading, `Messages()`
            defensive copy, `Reset`, `Truncate` (no-op when below limit,
            keeps last N including a tool_use/tool_result pair),
            `Run`-still-works wrapper; `chatLoop` greet + reply + exit,
            multi-turn history, `/reset`, `/help` + unknown-slash
            notice, blank-line skip, EOF-exit, ctx-cancel-exit, history
            truncation across many turns
      - [x] Smoke: `printf "/help\n/exit\n" | agent chat hello.md`
            renders the prompt, runs the slash command, exits cleanly
            without ever calling the LLM
- [x] **M8 — Dockerfile** ✅ *partial — packaging done, Jira→PR demo
      deferred*: distroless multi-stage Dockerfile builds linux/amd64 +
      linux/arm64 from a single source tree; image is ~16 MB on disk
      (~3.5 MB unique content), runs as nonroot, ships the example agent
      docs at `/examples`. A summarize agent (shell + fs_read) replaces
      the Jira→PR demo as the M8 hello-world; the latter needs a custom
      git builtin (go-git) and live Jira/GitHub creds, which is its own
      milestone of work.
      - [x] `Dockerfile`: cross-compile via `$TARGETOS/$TARGETARCH`,
            module + go-build cache mounts, `-trimpath` + `-X
            main.Version=…` for reproducible/labeled builds, distroless
            `static-debian12:nonroot` runtime
      - [x] `.dockerignore` keeps the build context to source + examples
      - [x] `examples/agents/summarize.md`: agent that explores a
            directory with shell + fs_read and produces a 5–10 bullet
            project summary — runnable end-to-end with just an
            `ANTHROPIC_API_KEY`
      - [x] Verified: single-arch build → 15.8 MB image with `agent run`,
            `agent validate /examples`, and `agent --help` all working
            inside the container; multi-arch buildx run produces an OCI
            manifest list containing both `linux/amd64` and `linux/arm64`
            image manifests
      - [ ] *Deferred*: Jira→PR demo. Needs (a) a `git` builtin or
            tools-via-MCP for clone/branch/commit/push and (b) a Jira MCP
            server with credentials; both are larger than M8's packaging
            scope.

After M8 the CLI is production-shaped. *Then* revisit operator / helm /
worker subcommand with the engine already battle-tested.

## Decided

- **Language**: Go 1.26+. Static build, `CGO_ENABLED=0`.
- **CLI framework**: `spf13/cobra`.
- **YAML**: `gopkg.in/yaml.v3`.
- **Logging**: stdlib `log/slog` (JSON handler).
- **MCP SDK**: `mark3labs/mcp-go` (revisit at M5; abstract behind `internal/mcp`).
- **Module path**: `github.com/milenkom81/m` (rename later if needed).
- **Binary name**: `agent`.
- **Config root**: `~/.agent/` (user) + `./.agent/` (project, overrides) +
  `/etc/agent/` (container/k8s).

## Deferred (do not build yet)

- Operator, CRDs, helm chart
- `agent worker` subcommand and task queue
- HTTP/SSE MCP transports (port exists, only stdio implemented)
- Native plugins (Go plugin, WASM) — MCP covers this
- Web UI, telemetry backends
