# AgentCTL

> The CLI binary is `m` for ergonomics — the product name is **AgentCTL**.

A small, single-binary CLI for running AI agents defined as Markdown files
against your choice of LLM. Aimed at developers and DevOps people who live in
the terminal and want to script agentic work without IDE lock-in or SDK
sprawl.

**Current version:** v0.0.33 | **Go version:** 1.26+ | **Binary size:** ~8.4 MB | **Docker image:** ~16 MB

**Status:** alpha. ~1 month of evenings of work. Works for the author's daily
use, but expect breaking changes until v0.1.0. Tagged releases (`v0.0.1` →
`v0.0.33`) ship as macOS `.pkg` and Linux `.deb`.

```text
$ m
» fix the failing test in api/handler.go
→ fs_read   api/handler.go
→ shell     go test ./api/...
→ fs_write  api/handler.go   (patch: nil check)
  Overwrite api/handler.go? [y/N]: y
→ shell     go test ./api/...
  PASS
→ git       commit -m "fix: nil check in handler"
```

Full docs site (EN + SR): **<https://subzone.github.io/Agentctl/>**

---

## Quick Start (5 minutes)

```bash
# 1. Install (macOS — pick one)
brew tap subzone/tap && brew install subzone/tap/m
# or: curl -sL https://github.com/subzone/Agentctl/releases/latest/download/m_0.0.33_macos.pkg -o m.pkg && sudo installer -pkg m.pkg -target /

# 2. Run the setup wizard
m
# Pick Ollama (free, local) or paste an API key for Anthropic/OpenAI/Gemini/Alibaba

# 3. Your first chat (with Steva Đubre fixing himself!)
» help me fix the failing test in internal/engine/engine_test.go
→ fs_read   internal/engine/engine_test.go
→ shell     go test ./internal/engine/...
→ fs_write  internal/engine/engine_test.go (patch: add nil check)
  Overwrite? [y/N]: y
→ shell     go test ./internal/engine/...
  PASS
→ git       commit -m "fix: nil check in engine test"

# 4. Slash commands
» /help          # show available commands
» /reset         # clear history
» /undo          # revert last fs_write
» /model ollama/qwen3-coder  # switch model mid-session
» /exit          # leave

# 5. Run a specific agent
m run examples/agents/devops.md "review the Dockerfile"
m chat examples/agents/coder.md

# 6. Create your own agent
m new my-agent
# edit my-agent.md, then: m chat my-agent

# 7. Check your setup
m doctor

# 8. Shell completions
m completion zsh > "${fpath[1]}/_m"
```

---

## Why this exists

- **Agents are files, not config** — define an agent as a Markdown file with
  YAML frontmatter, version it in git alongside your code, share it like any
  other source file.
- **No LLM SDK dependencies** — every provider client is plain `net/http` +
  `encoding/json`. The build won't break when a vendor SDK changes.
- **CLI-first, IDE-agnostic** — pipes, scripts, cron, CI all work because
  it's a normal binary that reads stdin and writes stdout.
- **Plays well with existing tooling** — `kubectl`, `terraform`, `helm`,
  `git`, `make` are reachable through the shell tool. Not a replacement for
  Cursor or Claude Code; a complementary tool for terminal-driven dev/DevOps
  work.

---

## Install

| Platform | How |
|----------|-----|
| macOS (Homebrew) | `brew tap subzone/tap && brew install subzone/tap/m` |
| macOS (pkg) | Download `.pkg` from [latest release][releases] → double-click. Installs to `/usr/local/bin/m`. |
| Windows | Download `.zip` from [latest release][releases] → extract `m.exe` to a folder on your PATH. |
| Linux (Debian/Ubuntu) | `sudo dpkg -i m_*_linux_amd64.deb` |
| Linux (other) | Tarball: `tar -xzf m_*_linux_amd64.tar.gz && sudo mv m /usr/local/bin/` |
| From source | `go install github.com/subzone/Agentctl/cmd/m@latest` (requires Go 1.26+) |

First run launches a setup wizard:

```bash
m
# Pick a provider (Ollama / Anthropic / OpenAI / Gemini / Alibaba / LiteLLM)
# Paste an API key (or skip for Ollama)
# Done — drops you into a chat with the default agent
```

Verify your setup:

```bash
m doctor
# Checks config, API key, model reachability, tools (git, grep, rg)
```

### Shell completions

```bash
# bash
m completion bash > /etc/bash_completion.d/m
# zsh
m completion zsh > "${fpath[1]}/_m"
# fish
m completion fish > ~/.config/fish/completions/m.fish
```

API keys are stored in the OS keychain (macOS Keychain / Linux libsecret).
Never in config files, never in plaintext.

### Auto-update notifications

AgentCTL checks GitHub for new releases once per day. If a newer version
exists, you'll see a dim notice on startup:

```
↑ update available: v0.0.29 → v0.0.32 (brew upgrade subzone/tap/m)
```

This is non-blocking, cached, and silent on errors. No data is sent — it
only reads the public releases API.

### API key fallback

If you don't want to use the keychain (or `secret-tool` isn't installed on Linux),
you can set API keys via environment variables instead:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
export OPENAI_API_KEY=sk-...
export GEMINI_API_KEY=...
export DASHSCOPE_API_KEY=...  # Alibaba
export LITELLM_API_KEY=...
```

The CLI checks keychain first, then falls back to the environment variable.
This works for both the main `m` command and for model discovery (`m config scan`).

---

## Defining an agent

A complete agent is one Markdown file:

```markdown
---
name: devops
type: agent
model: anthropic/claude-sonnet-4-6
fallback:
  - anthropic/claude-haiku-4-5-20251001
  - openai/gpt-4.1
tools:
  - shell
  - fs_read
  - fs_write
  - git
  - test_run
  - web_fetch
  - code_search
temperature: 0.3
pii_guard: redact
thinking_phrases:
  - "analyzing"
  - "reading code"
  - "checking config"
---
You are a DevOps engineer.
Explore the project with fs_list before editing.
Make targeted changes with fs_write.
Always consider security.
```

**Fallback models:** when the primary model returns 429 (rate limit), the
agent automatically tries the next model in the `fallback` list. The session
switches to the first one that works.

**Thinking phrases:** customize the spinner text shown while the agent works.
Overrides theme defaults. Useful for non-English agents.

Run it:

```bash
m chat examples/agents/devops.md
m run examples/agents/devops.md "audit the Dockerfile"
```

The repo ships **32 example agents** in [`examples/agents/`](examples/agents/),
including `coder`, `reviewer`, `planner`, `k8s-debug`, `terraform-plan`,
`helm-deploy`, `ticket-worker`, plus persona variants (`steva-djubre.md`,
`steve-trash.md`).

---

## Bundled agents

All 32 example agents are **embedded in the binary**. No need to clone the
repo — they're available immediately after install:

```bash
# List all available agents (bundled + user-created)
m list

# Run a bundled agent by name — extracted on first use
m chat devops
m run reviewer "check the auth module"
m chat steva-djubre
```

On first run (or when you reference a bundled agent), the .md file is
extracted to `~/.config/m/agents/` (or `~/Library/Application Support/m/agents/`
on macOS). You can edit these freely — your changes are never overwritten.

To reset a bundled agent to its original version, just delete it and run
again:

```bash
rm ~/.config/m/agents/devops.md
m chat devops  # re-extracts the bundled version
```

---

## Built-in tools

| Tool        | Purpose                                  | User confirmation |
|-------------|------------------------------------------|-------------------|
| `shell`     | Run a shell command                      | yes (per call)    |
| `fs_read`   | Read a file                              | no                |
| `fs_write`  | Create or patch a file                   | yes (diff preview) |
| `fs_list`   | List a directory (recursive, skips `.git`/`node_modules`) | no |
| `git`       | Common git operations                    | yes for writes    |
| `test_run`  | Run the project's test command           | no                |
| `web_fetch` | Fetch a URL and extract readable text    | no                |
| `code_search`| Search codebase: text (grep) + symbol index | no             |
| `delegate`  | Call a sub-agent                         | no                |

`fs_write` writes are reversible via `/undo`.

---

## Providers

Selected per-agent via `model: provider/model-name`. Switch providers
mid-session with `/model provider/model`.

| Provider   | Transport   | Notes |
|------------|-------------|-------|
| `ollama`   | NDJSON      | Local, free. Default for the wizard. |
| `anthropic`| Custom SSE  | Claude family. Native tool use, response-tool for structured output. |
| `openai`   | OpenAI SSE  | GPT-4o / GPT-4.1. `json_schema` strict mode. |
| `gemini`   | OpenAI-compat | `gemini-2.5-pro` / `flash` via Google's OpenAI-compat endpoint. |
| `alibaba`  | OpenAI-compat | DashScope: `qwen-plus` / `turbo` / `max`. |
| `litellm`  | OpenAI-compat | Proxy passthrough — opens up ~100 more models. |

All clients are stdlib-only. Gemini, Alibaba and LiteLLM use a `WithCompat()`
flag that disables OpenAI-specific stream options.

---

## MCP integrations

Five MCP server definitions ship in [`examples/mcp/`](examples/mcp/):

- `github` — PR/issue/repo operations (stdio)
- `jira`   — search, read, create, update, transition issues (stdio)
- `confluence` — search, read, create, update pages (stdio)
- `datadog` — monitoring, alerts, dashboards (HTTP)
- `slack` — channels, messages, users (SSE)

Reference one from an agent:

```yaml
mcp: [jira, confluence]
```

Tools are namespaced (`jira__get_issue`, `confluence__update_page`) and merged
into the same registry as built-ins. Supported transports:

| Transport | How it works |
|-----------|-------------|
| `stdio`   | Spawns a subprocess, JSON-RPC over stdin/stdout |
| `http`    | POST JSON-RPC to a URL, get JSON-RPC response |
| `sse`     | POST JSON-RPC, receive response via Server-Sent Events |

---

## Slash commands (chat REPL)

| Command     | Effect |
|-------------|--------|
| `/help`     | Show available commands |
| `/exit`, `/quit` | Leave the session |
| `/reset`    | Clear chat history |
| `/compact`  | Truncate history to last 4 exchanges |
| `/undo`     | Revert the most recent `fs_write` |
| `/config`   | Open interactive provider/model manager |
| `/spec`     | Show the agent's resolved spec |
| `/model`    | Switch provider/model mid-session |
| `/models`   | List available models, pick by number |
| `/save [name]` | Save session snapshot — `/save` (timestamped) or `/save fixing-auth` (named) |
| `/sessions` | List saved sessions |
| `/resume`   | Resume a saved session by id or number |
| `/themes`   | List available themes with descriptions |
| `/theme`    | Switch TUI theme |

---

## Architecture

Hexagonal layout, ~8.8k LOC, 24 test files. No SDK dependencies for LLM
clients.

```
cmd/m/                CLI entry, TUI, REPL, slash commands
internal/engine/      Session loop, tool dispatch, structured output
internal/llm/         Provider registry + 6 stdlib-only clients
internal/tools/       Built-in tool implementations
internal/mcp/         JSON-RPC stdio client, tool adapter
internal/config/      Frontmatter parsing, agent/MCP/skill schemas
internal/ports/       ConfigSource, Secrets, StateStore interfaces
internal/adapters/    Keychain (macOS/libsecret), file-backed stores
examples/agents/      32 ready-to-use agents
examples/mcp/         5 MCP server definitions
docs/                 Static product site (EN + SR), GitHub Pages
```

The engine never sees provider-specific code — providers register themselves
via `init()` + `llm.Register()`, and the engine only consumes a
`Provider.Stream(ctx, req) → <-chan Event` interface.

For a deeper walk-through (engine loop, hub-and-spoke delegation, MCP flow,
structured output mechanics), see the
[architecture page](https://subzone.github.io/Agentctl/architecture.html) or
[`PLAN.md`](PLAN.md).

---

## What works today

- Single-binary install on macOS / Linux / Windows (amd64 + arm64)
- **32 bundled agents** — available immediately after install, no clone needed
- 6 LLM providers, switchable mid-session
- 9 built-in tools with user confirmation on writes + undo
- MCP stdio + HTTP + SSE transports with auto-discovery and namespacing
- Hub-and-spoke sub-agent delegation
- Provider-native structured output enforcement (`response_schema`)
- Full-screen TUI with token/cost/context indicators, falls back to line REPL in pipes
- 9 built-in themes (matrix, nord, dracula, gruvbox, tokyonight, catppuccin, solarized, default, minimal)
- Session persistence with AES-256-GCM encryption, autosave, and graceful shutdown (Ctrl+C saves)
- Token-based context compaction (per-model context window awareness)
- Agent discovery (`m list`), registry (`m install`), scaffold (`m new`)
- Fallback models (auto-switch on 429 rate limit)
- Dangerous command double-confirmation (34 patterns)
- PII guardrails (redact emails, phones, SSNs, credit cards, API keys before sending to LLM)
- Command shortcuts (/x /r /c /u /m /t /s /h)
- Auto-update notifications (checks GitHub once/day)
- Shell completions (bash/zsh/fish/powershell)
- Homebrew tap with auto-update on release
- Tagged release pipeline producing `.pkg` and `.deb`

## Known gaps

These are real, not roadmap-ware. They affect what AgentCTL can be used for today:

- **No codebase RAG / embedding store.** The `code_search` tool provides
  grep + symbol index search, but there's no semantic/embedding-based
  retrieval. For most codebases, `code_search` + `fs_read` is sufficient.
- **No `/trust` for autonomous sessions.** Every `fs_write` and `shell`
  prompts. Fine for interactive use, blocks long-running headless runs.
- **No team features.** No shared agent registry, no audit log, no RBAC,
  no sandboxed execution. Single-developer use only for now.
- **No IDE integration.** Intentional — this is a CLI tool. Not planned.

The internal UX backlog is in [`UX_IMPROVEMENTS_PLAN.md`](UX_IMPROVEMENTS_PLAN.md).

---

## Codebase context (RAG)

Not built in. Three reasonable paths if you need it:

1. **MCP route** — point AgentCTL at any vector-store MCP server (Qdrant,
   Chroma, etc.). The agent gets `vector__search` as a normal tool. No
   code changes needed; this is how it'll work for now.
2. **A `code_search` built-in tool** that wraps `ripgrep` + a small in-memory
   index over the working tree. Cheaper than embeddings, often enough for
   "find similar functions". Probably the next logical addition.
3. **First-class embedding store** in `internal/` with a pluggable backend.
   Bigger lift, only worth it if there's a commercial story behind it.

If RAG matters for your use case, option 1 unblocks you today.

---

## Naming

The product is called **AgentCTL**. The CLI binary remains `m` for
ergonomics — short to type, easy to alias, works in scripts. Think of it
like how "Kubernetes" is the product but `kubectl` is the binary.

---

## Building from source

```bash
git clone https://github.com/subzone/Agentctl.git
cd Agentctl
make build       # produces ./m
make test        # runs go test ./...
make lint        # golangci-lint
```

Requires Go 1.26+.

---

## Contributing

Early-stage project. Bugs, design feedback, and PRs all welcome. Before a
PR for a non-trivial change, open an issue so we can align on scope —
the architecture is small enough that one wrong abstraction hurts.

- Issues: <https://github.com/subzone/Agentctl/issues>
- Discussions: <https://github.com/subzone/Agentctl/discussions>

---

## License

MIT. See [LICENSE](LICENSE).

[releases]: https://github.com/subzone/Agentctl/releases/latest
