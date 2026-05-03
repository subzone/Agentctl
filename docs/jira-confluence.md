---
title: Jira & Confluence
layout: default
nav_order: 9
---

# Jira & Confluence integration

`m` integrates with Jira and Confluence through MCP servers — no custom
code in the binary. Agents read tickets, check linked Confluence pages,
implement the work, and update the ticket when done.

## Setup

### 1. Install the MCP server

The [mcp-server-atlassian](https://github.com/sooperset/mcp-atlassian)
package provides both Jira and Confluence MCP servers:

```bash
pip install mcp-server-atlassian
```

Verify it's on PATH:

```bash
mcp-server-atlassian --help
```

### 2. Set environment variables

```bash
export JIRA_URL=https://myteam.atlassian.net
export JIRA_EMAIL=you@company.com
export CONFLUENCE_URL=https://myteam.atlassian.net/wiki
export CONFLUENCE_EMAIL=you@company.com
```

### 3. Store API tokens in the keychain

Generate an API token at
[id.atlassian.com/manage-profile/security/api-tokens](https://id.atlassian.com/manage-profile/security/api-tokens).

Store it so `m` can pass it to the MCP server:

```bash
# macOS
security add-generic-password -a default -s m-cli-jira-token -w "<your-token>"
security add-generic-password -a default -s m-cli-confluence-token -w "<your-token>"

# Linux
secret-tool store --label="m jira token" service m-cli provider jira-token account default
secret-tool store --label="m confluence token" service m-cli provider confluence-token account default
```

### 4. MCP server definitions

The repo includes ready-to-use MCP server definitions in
[`examples/mcp/`][mcp-examples]:

- `jira.md` — Jira issue operations (search, read, create, update, transition)
- `confluence.md` — Confluence page operations (search, read, create, update)

These are referenced by agents via `mcp: [jira, confluence]` in their
frontmatter.

## Agents

### ticket-worker — Ticket-driven development

The full development workflow driven by a Jira ticket:

```bash
m chat examples/agents/ticket-worker.md
```

```
» work on PROJ-123
→ jira__get_issue {"key": "PROJ-123"}
← PROJ-123: Add rate limiting to /api/v2/users endpoint
  Acceptance criteria:
  1. Rate limit of 100 req/min per API key
  2. Return 429 with Retry-After header
  3. Confluence spec: https://myteam.atlassian.net/wiki/spaces/ENG/pages/12345
→ confluence__get_page {"page_id": "12345"}
← Rate Limiting Spec: token bucket algorithm, Redis backend, ...
→ fs_list {"path": ".", "recursive": true}
→ fs_read {"path": "internal/api/middleware.go"}
→ git {"action": "checkout", "args": ["-b", "PROJ-123-rate-limiting"]}
→ fs_write {"path": "internal/api/ratelimit.go", "mode": "create", ...}
→ fs_write {"path": "internal/api/middleware.go", "mode": "patch", ...}
→ test_run {"command": "go test ./internal/api/..."}
  PASS
→ git {"action": "add", "args": ["."]}
→ git {"action": "commit", "args": ["-m", "PROJ-123: Add rate limiting middleware"]}
→ jira__add_comment {"key": "PROJ-123", "body": "Implemented rate limiting..."}
→ jira__transition_issue {"key": "PROJ-123", "transition": "In Review"}
```

The agent:
1. Reads the ticket and linked Confluence pages for the full spec
2. Explores the codebase to understand where changes go
3. Creates a feature branch named after the ticket
4. Implements the changes, runs tests
5. Commits with the ticket key in the message
6. Updates the ticket with a summary and transitions it

### ticket-reviewer — Ticket-aware code review

Reviews code changes against what the ticket actually asked for:

```bash
m chat examples/agents/ticket-reviewer.md
```

```
» review PROJ-123 changes
→ jira__get_issue {"key": "PROJ-123"}
← Acceptance criteria: rate limit 100/min, 429 + Retry-After, Redis backend
→ git {"action": "diff", "args": ["main...HEAD"]}
→ fs_read {"path": "internal/api/ratelimit.go"}
→ fs_read {"path": "internal/api/ratelimit_test.go"}

Review against PROJ-123 acceptance criteria:
✅ Rate limit of 100 req/min per API key — ratelimit.go:34, configurable via env
✅ Returns 429 with Retry-After header — ratelimit.go:52
⚠️ Redis backend — implementation uses in-memory store, not Redis as spec requires
❌ No test for the 429 response body format

Code quality:
- ratelimit.go:18 — mutex not needed if using sync.Map (minor)
- ratelimit_test.go — missing test for concurrent access

→ jira__add_comment {"key": "PROJ-123", "body": "Code review findings: ..."}
```

The reviewer compares implementation against each acceptance criterion
and posts findings back to the ticket.

## Workflow patterns

### Pattern 1: Ticket → Branch → Code → Review

The most common flow. Two agents, one ticket:

```bash
# Developer works the ticket
m chat examples/agents/ticket-worker.md
» work on PROJ-123

# Reviewer checks the work against the ticket
m chat examples/agents/ticket-reviewer.md
» review PROJ-123 changes
```

### Pattern 2: Sprint backlog triage

Use the ticket-worker in chat mode to triage multiple tickets:

```
» what's in the current sprint?
→ jira__search {"jql": "sprint in openSprints() AND assignee = currentUser()"}
← 5 tickets: PROJ-120 (Bug), PROJ-121 (Story), PROJ-123 (Story), ...

» summarize PROJ-120
→ jira__get_issue {"key": "PROJ-120"}
← Bug: Login fails when email contains a plus sign
  Priority: High, Status: To Do

» start on PROJ-120
→ git checkout -b PROJ-120-login-plus-sign
...
```

### Pattern 3: Confluence-driven implementation

When the real spec lives in Confluence, not the ticket:

```
» work on PROJ-200
→ jira__get_issue {"key": "PROJ-200"}
← "See Confluence page for full spec" + link
→ confluence__get_page {"page_id": "67890"}
← Full API specification with request/response schemas, error codes, ...
```

The agent follows Confluence links automatically and uses the page
content as the source of truth for implementation.

### Pattern 4: Hub orchestration with ticket context

Build a hub that reads the ticket once and delegates to specialists:

```markdown
---
name: ticket-hub
type: agent
model: anthropic/claude-sonnet-4-6
tools:
  - fs_read
  - fs_list
mcp:
  - jira
  - confluence
subagents:
  - spoke-coder
  - spoke-reviewer
  - spoke-planner
---
You are a ticket-aware orchestrator.

1. Fetch the ticket with jira__get_issue.
2. Read any linked Confluence pages.
3. Delegate to the right spoke with the full ticket context:
   - spoke-planner for task breakdown
   - spoke-coder for implementation
   - spoke-reviewer for code review
4. Update the ticket with results from each spoke.
```

## Available MCP tools

Once the Jira/Confluence MCP servers are connected, agents have access
to these tools (prefixed with `jira__` and `confluence__`):

### Jira tools

| Tool | Description |
|---|---|
| `jira__get_issue` | Read a ticket by key (summary, description, status, comments) |
| `jira__search` | JQL search across tickets |
| `jira__create_issue` | Create a new ticket |
| `jira__update_issue` | Update fields on an existing ticket |
| `jira__add_comment` | Add a comment to a ticket |
| `jira__transition_issue` | Move a ticket through workflow states |
| `jira__list_transitions` | List available transitions for a ticket |

### Confluence tools

| Tool | Description |
|---|---|
| `confluence__get_page` | Read a page by ID or title |
| `confluence__search` | CQL search across pages |
| `confluence__create_page` | Create a new page |
| `confluence__update_page` | Update an existing page |

The exact tool names depend on the MCP server implementation. Use
`m run` with a simple agent to discover what's available:

```bash
m run examples/agents/ticket-worker.md "list your available tools"
```

## Tips

- **Ticket key format**: agents expect standard Jira keys like `PROJ-123`.
  Just type the key in chat — the agent knows to fetch it.
- **JQL queries**: the agent can run arbitrary JQL. Ask it natural
  language questions like "what bugs are assigned to me?" and it'll
  translate to JQL.
- **Confluence links**: if a ticket's description contains a Confluence
  URL, the agent extracts the page ID and fetches it automatically.
- **Branch naming**: ticket-worker names branches `<KEY>-short-desc`
  by convention. This makes it easy to trace branches back to tickets.
- **Commit messages**: always prefixed with the ticket key so Jira's
  smart commits can pick them up.

## Next steps

- **[DevOps patterns](devops-patterns.html)** — K8s, Terraform, Helm agents
- **[Custom agents](agents.html)** — write your own ticket-aware agents
- **[Architecture](architecture.html)** — how MCP integration works

[mcp-examples]: https://github.com/subzone/m/tree/main/examples/mcp
