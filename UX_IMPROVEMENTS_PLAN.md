# Agent CLI (`m`) - UX Improvements Plan

Status tracker for UX and quality-of-life improvements.

## ✅ Implemented

### 1. `/trust` Command for Session Auto-Approval
Auto-approve all tools for the session. Dangerous commands (`rm -rf`,
`kubectl delete`, `terraform destroy`, etc.) still require double confirmation
even in trust mode.
- `/trust` — enable auto-approve
- `/trust off` — back to normal confirmation
- Works in both REPL and TUI

### 2. `/debug` Verbose/LLM Trace Mode
Stream raw LLM request/response JSON to stderr for debugging hallucinated
tool calls or prompt issues.
- `/debug` — enable trace
- `/debug off` — disable
- REPL only (TUI owns stderr)

### 3. Agent Discoverability (`m list`)
Scan directories for `.md` files with agent frontmatter, display as a table.
- `m list` — scans `./` and `./examples/agents/`
- `m list ~/my-agents/` — custom directory
- Skips `.git`, `node_modules`, `vendor`

### 4. Interactive Error Correction for Tool Calls
When a tool fails, the user can press Enter to retry or type a hint that
gets appended to the error sent back to the LLM.
- Prevents infinite retry loops
- User hints steer the agent away from repeated mistakes

### 5. Anthropic Prompt Caching
System prompt and tool schemas sent with `cache_control: ephemeral`,
reducing input tokens and TTFT on subsequent turns.

### 6. Dangerous Command Protection
34 patterns (`rm -rf`, `kubectl delete`, `terraform destroy`, `git push --force`,
etc.) always require double confirmation — even in `/trust` mode.
- First prompt: "Are you REALLY sure? [y/n]"
- Second prompt: "This is irreversible. Confirm again? [y/n]"

### 7. Fallback Models
When primary model returns 429, auto-try fallback models from the agent's
`fallback:` list. Session switches to the first one that works.

### 8. Session Persistence
AES-256-GCM encrypted autosave after every step. `/save`, `/sessions`,
`/resume` commands. Key stored in OS keychain.

### 9. Token-Based Context Compaction
Per-model context windows. Compacts at 80% budget with meaningful summaries
of dropped messages instead of blind truncation.

---

## ❌ Not Yet Implemented

### MCP HTTP/SSE Transport
Stdio only. Many real-world MCP servers use HTTP/SSE — they don't work yet.
This is the biggest gap for MCP adoption.

### Codebase RAG / Context Retrieval
Agents see only what they explicitly read with `fs_read` / `fs_list` /
`web_fetch`. No embedding store, no similarity search. Workaround: use
an MCP vector-store server, or `web_fetch` for docs.

### `m search` Command
`m list` exists but `m search` for fuzzy name/description matching does not.

### Team Features
No shared agent registry, no audit log, no RBAC, no sandboxed execution.
Single-developer use only.

---

*Last updated: v0.0.23*
