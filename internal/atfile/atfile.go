package atfile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Pattern matches @path/to/file references in user input.
var Pattern = regexp.MustCompile(`@(\.{0,2}[/\\]?[\w.\-/\\]+\.\w+)`)

// Candidate is a file match for @-mention autocomplete.
type Candidate struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Dir  string `json:"dir"`
}

// Expand replaces @file references with inlined file contents.
func Expand(input string) (string, []string) {
	matches := Pattern.FindAllStringSubmatch(input, -1)
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

		abs, err := filepath.Abs(ref)
		if err != nil {
			continue
		}

		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}

		content := strings.TrimRight(string(data), "\n")
		replacement := fmt.Sprintf("\n\n[File: %s]\n```\n%s\n```\n", ref, content)
		input = strings.Replace(input, "@"+ref, replacement, 1)
		included = append(included, ref)
	}

	return input, included
}

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"build": true, ".cursor": true, "frontend/node_modules": true,
}

// Search finds files under root whose path contains query (case-insensitive).
func Search(root, query string, limit int) []Candidate {
	if limit <= 0 {
		limit = 12
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}

	var out []Candidate
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if skipDirs[name] || strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(root, path)
			if skipDirs[rel] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return nil
		}
		if !strings.Contains(strings.ToLower(rel), q) {
			return nil
		}
		out = append(out, Candidate{
			Path: rel,
			Name: filepath.Base(rel),
			Dir:  filepath.Dir(rel),
		})
		if len(out) >= limit*3 {
			return filepath.SkipAll
		}
		return nil
	})

	// Prefer prefix matches, then shorter paths.
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
