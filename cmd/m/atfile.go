package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// atFilePattern matches @path/to/file references in user input.
// Supports: @main.go, @./src/handler.go, @../config.yaml
var atFilePattern = regexp.MustCompile(`@(\.{0,2}[/\\]?[\w.\-/\\]+\.\w+)`)

// expandAtFiles replaces @file references in user input with the file contents
// inlined as context. Returns the expanded text and list of files included.
func expandAtFiles(input string) (string, []string) {
	matches := atFilePattern.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return input, nil
	}

	var included []string
	seen := map[string]bool{}

	for _, match := range matches {
		ref := match[1]
		if seen[ref] {
			continue
		}
		seen[ref] = true

		path := ref
		// Try relative to cwd.
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}

		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}

		content := strings.TrimRight(string(data), "\n")
		// Replace the @ref with inlined content.
		replacement := fmt.Sprintf("\n\n[File: %s]\n```\n%s\n```\n", ref, content)
		input = strings.Replace(input, "@"+ref, replacement, 1)
		included = append(included, ref)
	}

	return input, included
}
