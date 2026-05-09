---
name: confluence
type: mcp_server
description: Confluence MCP server for page search, read, create, and update.
version: 1
transport: stdio
command:
  - mcp-server-atlassian
  - --confluence
env:
  CONFLUENCE_URL: ${env:CONFLUENCE_URL}
  CONFLUENCE_TOKEN: ${secret:confluence-token}
  CONFLUENCE_EMAIL: ${env:CONFLUENCE_EMAIL}
install:
  pip: mcp-server-atlassian
tool_prefix: confluence
allowed_agents:
  - ticket-worker
  - ticket-reviewer
  - developer-hub
---
Atlassian Confluence MCP server. Exposes tools under the `confluence__` prefix.

Requires [mcp-server-atlassian](https://github.com/sooperset/mcp-atlassian)
or any Confluence-compatible MCP server installed on PATH.

Setup:
```bash
pip install mcp-server-atlassian
```

Environment variables:
- `CONFLUENCE_URL` — your Confluence instance (e.g. `https://myteam.atlassian.net/wiki`)
- `CONFLUENCE_EMAIL` — your Atlassian account email
- `CONFLUENCE_TOKEN` — API token (stored in OS keychain via `m`)
