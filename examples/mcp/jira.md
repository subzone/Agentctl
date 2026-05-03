---
name: jira
type: mcp_server
description: Jira MCP server for issue search, read, create, update, and transition.
version: 1
transport: stdio
command:
  - mcp-server-atlassian
  - --jira
env:
  JIRA_URL: ${env:JIRA_URL}
  JIRA_TOKEN: ${secret:jira-token}
  JIRA_EMAIL: ${env:JIRA_EMAIL}
tool_prefix: jira
allowed_agents:
  - ticket-worker
  - ticket-reviewer
---
Atlassian Jira MCP server. Exposes tools under the `jira__` prefix.

Requires [mcp-server-atlassian](https://github.com/sooperset/mcp-atlassian)
or any Jira-compatible MCP server installed on PATH.

Setup:
```bash
pip install mcp-server-atlassian
```

Environment variables:
- `JIRA_URL` — your Jira instance (e.g. `https://myteam.atlassian.net`)
- `JIRA_EMAIL` — your Atlassian account email
- `JIRA_TOKEN` — API token (stored in OS keychain via `m`)
