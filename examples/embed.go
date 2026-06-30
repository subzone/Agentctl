// Package examples embeds the bundled agent and package .md files.
package examples

import "embed"

//go:embed agents/*.md
var Agents embed.FS

//go:embed packages/*.md
var Packages embed.FS

//go:embed mcp/*.md
var MCP embed.FS
