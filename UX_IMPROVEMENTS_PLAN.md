# Agent CLI (`m`) - UX Improvements Plan

This document outlines a backlog of User Experience (UX) and developer quality-of-life improvements for the `m` CLI agent. These tasks are scoped to be actionable and can be picked up by any developer or AI code assistant.

## 1. Implement `/trust` Command for Session Auto-Approval

**Context**: Destructive tools (like `fs_write` or `shell` commands) currently require manual `y/n` confirmation. In long-running autonomous coding sessions, this causes fatigue and blocks the agent from operating completely autonomously.
**Goal**: Allow users to auto-approve tools for the duration of a session.

**Implementation Steps**:
1. Add a new boolean field `autoApprove` to `engine.Session` or the `spawner`/`chatLoop` state.
2. In `cmd/m/chat.go`, intercept the `/trust` slash command.
3. When `/trust` is active, the `ToolConfirm` callback should bypass the `y/n` prompt and instantly return `true`, while still printing `→ tool_name [auto-approved]` to the status writer so the user knows what is happening.
4. (Optional) Add a `--trust` flag to `m chat` and `m run` to enable this from the command line on startup.

## 2. Add Verbose/LLM Trace Mode (`/debug`)

**Context**: When users write their own custom `agent.md` and the LLM hallucinates tool calls or misinterprets the prompt, it is hard to debug because the raw LLM JSON payloads are hidden.
**Goal**: Provide a way to stream the raw request/response JSON sent to the LLM providers.

**Implementation Steps**:
1. Add a `/debug` toggle in the chat REPL.
2. Add an `io.Writer` (e.g., `TraceWriter`) to the `engine.Config`.
3. In `internal/engine/engine.go`, before calling `Provider.Stream`, if `TraceWriter` is non-nil, marshal the `llm.Request` to JSON and write it.
4. Similarly, log the raw `llm.Event` streams coming back from the provider.
5. In `cmd/m/chat.go`, wire the `/debug` command to toggle attaching `os.Stderr` (or a file) to the `TraceWriter`.

## 3. Agent Discoverability (`m list` / `m search`)

**Context**: Currently, users must point the CLI to a specific markdown file path (e.g., `m chat examples/agents/coder.md`). It is hard to discover what agents exist in a project or globally.
**Goal**: Provide a native command to list all available agents in the current project or a global registry.

**Implementation Steps**:
1. Create a new Cobra command `newListCmd()` in `cmd/m/list.go`.
2. The command should walk standard directories (`./`, `./agents`, `./examples/agents`, and `~/.agent/`) looking for `.md` files.
3. Parse the frontmatter of found `.md` files using `config.ParseFile`.
4. If `type: agent`, display the `name`, `description`, and file path in a neatly formatted table to the console.

## 4. Interactive Error Correction for Tool Calls

**Context**: When an LLM generates invalid JSON for a tool call (or misses a required argument), the engine catches the error, sends it back as a `tool_result` with `IsError: true`, and the LLM blindly retries. This wastes tokens and often traps the LLM in a loop.
**Goal**: Allow the user to inject a hint when a tool call fails.

**Implementation Steps**:
1. Modify `runToolBlock` in `internal/engine/engine.go` to detect when a tool execution returns an error.
2. Before returning the error to the LLM automatically, trigger a special callback (e.g., `ErrorIntervention`) if configured.
3. In `cmd/m/chat.go`, implement this callback: print the error to the user and ask: `[Tool failed. Press Enter to let agent retry, or type a hint:]`.
4. If the user types a hint, append it to the error payload sent back to the LLM (e.g., `ERROR: file not found. User hint: check the /src directory instead.`).

## 5. Optimize Context Window for Large MCP Schemas

**Context**: If a user connects an MCP server with dozens of tools, pushing the entire JSON schema array to the LLM on every turn eats up input tokens and slows down the Time-to-First-Token (TTFT).
**Goal**: Reduce the overhead of tool schemas.

**Implementation Steps**:
1. Investigate provider-specific caching features (e.g., Anthropic's prompt caching for tools).
2. If the provider supports caching, update `internal/llm/anthropic` to inject cache control headers onto the `tools` array block.
3. Alternatively, implement "lazy loading" of tools in the engine, where the LLM is first given a summary of available tools, and must use a `get_tool_schema` builtin tool to fetch the exact arguments before calling the real tool.

---
*Created as an actionable backlog based on code assessment.*
