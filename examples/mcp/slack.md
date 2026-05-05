---
name: slack
type: mcp_server
description: Slack MCP server — channels, messages, users.
transport: sse
url: https://mcp.slack.com/sse
tool_prefix: slack
env:
  SLACK_BOT_TOKEN: ${SLACK_BOT_TOKEN}
---
Slack MCP server for team communication operations.
Provides tools for sending messages, managing channels, and searching conversations.

Requires SLACK_BOT_TOKEN environment variable.
