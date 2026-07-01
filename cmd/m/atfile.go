package main

import "github.com/subzone/Agentctl/internal/atfile"

// expandAtFiles replaces @file references in user input with the file contents
// inlined as context. Returns the expanded text and list of files included.
func expandAtFiles(input string) (string, []string) {
	return atfile.Expand(input)
}
