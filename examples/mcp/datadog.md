---
name: datadog
type: mcp_server
description: Datadog MCP server — monitoring, alerts, dashboards.
transport: http
url: https://mcp.datadoghq.com/api/v1
tool_prefix: datadog
env:
  DD_API_KEY: ${DD_API_KEY}
  DD_APP_KEY: ${DD_APP_KEY}
---
Datadog MCP server for monitoring and observability operations.
Provides tools for querying metrics, managing alerts, and dashboard operations.

Requires DD_API_KEY and DD_APP_KEY environment variables.
