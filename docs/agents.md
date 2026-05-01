---
title: Custom agents
---

[← Docs home](./)

# Custom agents

The bare `m` command chats with a built-in agent — useful for ad-hoc
questions. For real work, write your own agents as Markdown files
with YAML frontmatter and run them explicitly.

## Anatomy of an agent file

```markdown
---
name: hello
type: agent
description: Smallest possible agent — answers a single message.
version: 1
model: anthropic/claude-sonnet-4-6
temperature: 0.7
max_tokens: 1024
---
You are a friendly assistant. Reply concisely and directly.
```

The YAML frontmatter (between the `---` fences) defines the agent's
metadata; the body after the frontmatter is the system prompt.

### Required fields

| Field | Type | Notes |
|---|---|---|
| `name` | string | Agent identifier shown in chat header. |
| `type` | string | Must be `agent`. |
| `model` | string | `provider/model` form, e.g. `anthropic/claude-sonnet-4-6`. |

### Common optional fields

| Field | Type | Notes |
|---|---|---|
| `description` | string | One-line summary. |
| `version` | int | Bump when meaningfully changed. |
| `temperature` | float | Sampling temperature. Provider defaults if omitted. |
| `max_tokens` | int | Output cap. Defaults to ~4 K. |
| `tools` | list | Built-in tool allowlist. Empty/missing → builtins only. |
| `mcp` | list | Names of MCP servers (declared in companion `.md` files). |
| `skills` | list | Reusable instruction blocks composed into the prompt. |
| `subagents` | list | Names of agents this one can delegate to. |
| `powers` | list | Capability tags (advisory, used by tools). |

## Built-in tools

The `tools:` allowlist exposes these to the model:

| Tool | What it does |
|---|---|
| `shell` | Run shell commands. |
| `fs_read` | Read files. |
| `fs_write` | Create or overwrite files. |
| `delegate` | Invoke a sub-agent (auto-added when `subagents:` is set). |

Empty allowlist → only safe built-ins (`fs_read` is currently the only
builtin in the safe set; check `m run --help` for current behavior).

## Running an agent

One-shot:

```bash
m run my-agent.md "task description"
```

Interactive REPL:

```bash
m chat my-agent.md
```

Validate without running:

```bash
m validate my-agent.md
```

## Examples

The repo ships with a handful in [`examples/agents/`][examples]:

- `hello.md` — minimal chat agent
- `coder.md` — software-engineer agent with shell + fs access and a
  `planner` sub-agent
- `qwen-coder.md` — local coder backed by Ollama / Qwen
- `summarize.md` — single-purpose summarizer
- `planner.md` — sub-agent that drafts implementation plans

Read them as templates for your own.

[examples]: https://github.com/subzone/m/tree/main/examples/agents

## Project layout

For non-trivial setups, drop agents and companion docs (skills, MCP
server defs, sub-agents) under a single project root:

```
my-project/
  agents/
    coder.md
    planner.md
  skills/
    code-review.md
  mcp/
    github.md
  tools/
    custom-linter.md
```

When you `m run agents/coder.md "..."`, `m` walks the project root and
loads every parseable MD file, resolving cross-references in the agent
file (e.g. `subagents: [planner]` finds `agents/planner.md`).

## Replacing the default chat agent

The default chat agent is **embedded** in the binary, so you can't
edit it directly without rebuilding. Two workarounds:

1. **Override per-session:** `M_MODEL=anthropic/claude-opus-4-7 m`.
2. **Use a custom agent for chat:** `m chat my-agent.md` instead of
   bare `m`.

A future release may add a "use this agent file as the bare-`m`
default" option in the wizard. Until then, `m chat` is the explicit
path.

## Next steps

- **[Configuration](configuration.html)** — env vars and file layout.
- **[Troubleshooting](troubleshooting.html)** — agent validation
  failures, tool errors.
