// Package examples embeds the bundled agent .md files.
package examples

import "embed"

//go:embed agents/*.md
var Agents embed.FS
