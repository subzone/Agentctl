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
| `shell` | Run shell commands via `/bin/sh -c`. |
| `fs_read` | Read a UTF-8 file from disk. |
| `fs_write` | Create or patch files. **User confirms every write.** |
| `fs_list` | List directory contents, optionally recursive. |
| `delegate` | Invoke a sub-agent (auto-added when `subagents:` is set). |

### File write confirmation

When the model calls `fs_write`, the user is always prompted:

```
Overwrite main.go (1200 → 1250 bytes)? [y/N]:
```

Type `y` to approve or `n` (or just Enter) to decline. If declined,
the model sees "user declined the write" and can adjust its approach.

`fs_write` supports two modes:
- **`create`** — write full file content (creates parent directories).
- **`patch`** — find-and-replace a specific substring in an existing file.

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

## Example agents

The repo ships with these in [`examples/agents/`][examples]:

| Agent | Model | Tools | Use case |
|---|---|---|---|
| `hello.md` | Claude Sonnet | none | Minimal test agent |
| `coder.md` | Claude Sonnet | all + MCP GitHub | Full coding assistant with planner subagent |
| `qwen-coder.md` | Ollama/qwen3-coder | all | Local coding, no API key |
| `reviewer.md` | Claude Sonnet | read-only | Code review, never edits |
| `writer.md` | Claude Sonnet | read + write | Docs, READMEs, prose |
| `devops.md` | Claude Sonnet | all | Dockerfiles, CI/CD, infra |
| `local.md` | Ollama/qwen3-coder | read-only | General local assistant, no cost |
| `summarize.md` | Claude Sonnet | read + list | Project summarizer |
| `planner.md` | Claude Haiku | read + list | Task planning, no execution |
| `k8s-debug.md` | Claude Sonnet | all + git | Kubernetes troubleshooting and triage |
| `terraform-plan.md` | Claude Sonnet | all + git + test | Terraform plan review and module authoring |
| `helm-deploy.md` | Claude Sonnet | all + git | Helm chart review, linting, and deployment |
| `ticket-worker.md` | Claude Sonnet | all + Jira/Confluence MCP | Ticket-driven development (read ticket → implement → update) |
| `ticket-reviewer.md` | Claude Sonnet | read + git + Jira/Confluence MCP | Review code against Jira ticket acceptance criteria |

Read them as templates for your own. For DevOps-specific patterns and
usage examples, see **[DevOps patterns](devops-patterns.html)**. For
Jira/Confluence workflows, see **[Jira & Confluence](jira-confluence.html)**.

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
```

When you `m run agents/coder.md "..."`, `m` walks the project root and
loads every parseable MD file, resolving cross-references in the agent
file (e.g. `subagents: [planner]` finds `agents/planner.md`).

## Choosing the default chat agent

The bare `m` command uses an embedded default agent. To customize:

- **Override per-session:** `M_MODEL=anthropic/claude-opus-4 m`.
- **Use a custom agent:** `m chat my-agent.md` instead of bare `m`.
- **Set a default agent file:** add `default_agent: /path/to/agent.md`
  to your `config.yaml`.

## Structured output (response_schema)

Agents can declare a `response_schema` in their frontmatter to constrain
the model's output to valid JSON:

```yaml
---
name: spoke-reviewer
type: agent
model: anthropic/claude-sonnet-4-6
response_schema:
  type: object
  properties:
    answer:
      type: string
    sources:
      type: array
      items:
        type: object
        properties:
          file: { type: string }
          summary: { type: string }
        required: [file, summary]
    confidence:
      type: string
      enum: [high, medium, low]
    caveats:
      type: array
      items: { type: string }
  required: [answer, sources, confidence, caveats]
---
```

The engine enforces this via provider-native mechanisms:
- **OpenAI:** `response_format.json_schema` with strict mode
- **Anthropic:** synthetic response-tool with forced `tool_choice`
- **Ollama:** `format` field with JSON schema

The user sees only the `answer` field; the full JSON is stored in
history for the hub agent to consume.

## Hub-and-spoke pattern

For complex tasks, use an orchestrator (hub) that delegates to
specialist agents (spokes):

```
examples/agents/
  hub.md              — routes tasks, synthesizes with citations
  spoke-coder.md      — writes code, returns structured JSON
  spoke-reviewer.md   — reviews code, returns structured JSON
  spoke-planner.md    — creates plans, returns structured JSON
```

Spokes return `{answer, sources[], confidence, caveats[]}`. The hub
cites which spoke provided each piece of information:
`[spoke-coder: main.go:10-25]`.

Multiple spoke delegations in the same turn run in parallel.

## Next steps

- **[DevOps patterns](devops-patterns.html)** — K8s, Terraform, Helm agents and MCP.
- **[Architecture](architecture.html)** — how it's all wired up.
- **[Configuration](configuration.html)** — env vars and file layout.
- **[Troubleshooting](troubleshooting.html)** — agent validation
  failures, tool errors.
