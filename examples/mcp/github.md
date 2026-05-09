---
name: github
type: mcp_server
description: GitHub MCP server for PR/issue/repo operations.
version: 1
transport: stdio
command:
  - mcp-server-github
env:
  GITHUB_TOKEN: ${secret:github-token}
install:
  npm: "@modelcontextprotocol/server-github"
tool_prefix: github
allowed_agents:
  - coder
  - developer-hub
---
GitHub MCP server. Tools are exposed under the `github__` prefix to avoid
collisions with other servers that publish similarly-named tools.
