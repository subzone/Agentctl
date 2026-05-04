package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CodeSearchTool provides two search modes:
//   - text: grep/ripgrep for exact pattern matching
//   - symbol: in-memory index of functions, types, classes, imports
//
// The index is built lazily on first symbol search and cached for the session.
type CodeSearchTool struct {
	index *projectIndex
}

func NewCodeSearch() *CodeSearchTool { return &CodeSearchTool{} }

func (c *CodeSearchTool) Name() string { return "code_search" }

func (c *CodeSearchTool) Description() string {
	return "Search the codebase. Two modes:\n" +
		"- type=text: grep for a pattern across files (fast, exact match)\n" +
		"- type=symbol: search function/type/class names and imports from an in-memory index\n" +
		"Returns matching file paths, line numbers, and context. Use this before fs_read to find the right files."
}

func (c *CodeSearchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "query": {"type": "string", "description": "Search pattern (regex for text, substring for symbol)"},
    "type": {"type": "string", "enum": ["text", "symbol"], "description": "Search mode: text (grep) or symbol (functions/types/imports)"},
    "path": {"type": "string", "description": "Directory to search (default: current directory)"}
  },
  "required": ["query"]
}`)
}

func (c *CodeSearchTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Query string `json:"query"`
		Type  string `json:"type"`
		Path  string `json:"path"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.Query == "" {
		return "", fmt.Errorf("query is required")
	}
	if args.Type == "" {
		args.Type = "text"
	}
	if args.Path == "" {
		args.Path = "."
	}

	switch args.Type {
	case "text":
		return c.textSearch(ctx, args.Query, args.Path)
	case "symbol":
		return c.symbolSearch(args.Query, args.Path)
	default:
		return "", fmt.Errorf("unknown search type %q (use text or symbol)", args.Type)
	}
}

// textSearch uses ripgrep or grep for fast pattern matching.
func (c *CodeSearchTool) textSearch(ctx context.Context, query, dir string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Try ripgrep first, fall back to grep.
	var cmd *exec.Cmd
	if _, err := exec.LookPath("rg"); err == nil {
		cmd = exec.CommandContext(cctx, "rg", "--no-heading", "--line-number",
			"--max-count=5", "--max-filesize=1M",
			"-g", "!.git", "-g", "!node_modules", "-g", "!vendor", "-g", "!dist",
			query, dir)
	} else {
		cmd = exec.CommandContext(cctx, "grep", "-rn", "--include=*.go",
			"--include=*.py", "--include=*.js", "--include=*.ts",
			"--include=*.java", "--include=*.rb", "--include=*.rs",
			"--include=*.c", "--include=*.cpp", "--include=*.h",
			"--include=*.yaml", "--include=*.yml", "--include=*.json",
			"--include=*.tf", "--include=*.sh", "--include=*.md",
			"-m", "5", query, dir)
	}

	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))

	if len(text) == 0 {
		if err != nil {
			return "no matches found", nil
		}
		return "no matches found", nil
	}

	// Truncate if too large.
	lines := strings.Split(text, "\n")
	if len(lines) > 50 {
		text = strings.Join(lines[:50], "\n") + fmt.Sprintf("\n... and %d more matches", len(lines)-50)
	}

	return text, nil
}

// symbolSearch queries the in-memory project index.
func (c *CodeSearchTool) symbolSearch(query, dir string) (string, error) {
	if c.index == nil || c.index.root != dir {
		idx, err := buildIndex(dir)
		if err != nil {
			return "", fmt.Errorf("index build failed: %w", err)
		}
		c.index = idx
	}
	return c.index.search(query), nil
}

// projectIndex holds extracted symbols from the project.
type projectIndex struct {
	root    string
	symbols []symbol
}

type symbol struct {
	Name string // function/type/class name
	Kind string // func, type, class, import, interface
	File string // relative path
	Line int    // 1-based line number
}

func buildIndex(root string) (*projectIndex, error) {
	idx := &projectIndex{root: root}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "node_modules" || base == "vendor" || base == "dist" || base == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Size() > 512*1024 { // skip files > 512KB
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		patterns, ok := langPatterns[ext]
		if !ok {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		lines := bytes.Split(raw, []byte("\n"))
		for i, line := range lines {
			s := string(line)
			for _, p := range patterns {
				if m := p.re.FindStringSubmatch(s); len(m) > 1 {
					idx.symbols = append(idx.symbols, symbol{
						Name: m[1],
						Kind: p.kind,
						File: rel,
						Line: i + 1,
					})
				}
			}
		}
		return nil
	})
	return idx, err
}

func (idx *projectIndex) search(query string) string {
	q := strings.ToLower(query)
	var matches []string
	for _, s := range idx.symbols {
		if strings.Contains(strings.ToLower(s.Name), q) {
			matches = append(matches, fmt.Sprintf("  %s %s — %s:%d", s.Kind, s.Name, s.File, s.Line))
		}
	}
	if len(matches) == 0 {
		return fmt.Sprintf("no symbols matching %q (indexed %d symbols)", query, len(idx.symbols))
	}
	if len(matches) > 30 {
		return strings.Join(matches[:30], "\n") + fmt.Sprintf("\n... and %d more", len(matches)-30)
	}
	return fmt.Sprintf("%d matches:\n%s", len(matches), strings.Join(matches, "\n"))
}

// langPatterns maps file extensions to regex patterns for symbol extraction.
type pattern struct {
	kind string
	re   *regexp.Regexp
}

var langPatterns = map[string][]pattern{
	".go": {
		{kind: "func", re: regexp.MustCompile(`^func\s+(?:\([^)]+\)\s+)?(\w+)\s*\(`)},
		{kind: "type", re: regexp.MustCompile(`^type\s+(\w+)\s+(?:struct|interface)`)},
		{kind: "import", re: regexp.MustCompile(`^\s+"([^"]+)"`)},
	},
	".py": {
		{kind: "func", re: regexp.MustCompile(`^(?:async\s+)?def\s+(\w+)\s*\(`)},
		{kind: "class", re: regexp.MustCompile(`^class\s+(\w+)`)},
		{kind: "import", re: regexp.MustCompile(`^(?:from\s+(\S+)|import\s+(\S+))`)},
	},
	".js": {
		{kind: "func", re: regexp.MustCompile(`(?:function\s+(\w+)|(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\()`)},
		{kind: "class", re: regexp.MustCompile(`class\s+(\w+)`)},
		{kind: "export", re: regexp.MustCompile(`export\s+(?:default\s+)?(?:function|class|const|let)\s+(\w+)`)},
	},
	".ts": {
		{kind: "func", re: regexp.MustCompile(`(?:function\s+(\w+)|(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\()`)},
		{kind: "class", re: regexp.MustCompile(`class\s+(\w+)`)},
		{kind: "interface", re: regexp.MustCompile(`interface\s+(\w+)`)},
		{kind: "export", re: regexp.MustCompile(`export\s+(?:default\s+)?(?:function|class|const|let|interface|type)\s+(\w+)`)},
	},
	".java": {
		{kind: "class", re: regexp.MustCompile(`(?:public|private|protected)?\s*class\s+(\w+)`)},
		{kind: "interface", re: regexp.MustCompile(`interface\s+(\w+)`)},
		{kind: "func", re: regexp.MustCompile(`(?:public|private|protected)\s+\w+\s+(\w+)\s*\(`)},
	},
	".rb": {
		{kind: "func", re: regexp.MustCompile(`def\s+(\w+)`)},
		{kind: "class", re: regexp.MustCompile(`class\s+(\w+)`)},
		{kind: "module", re: regexp.MustCompile(`module\s+(\w+)`)},
	},
	".rs": {
		{kind: "func", re: regexp.MustCompile(`(?:pub\s+)?fn\s+(\w+)`)},
		{kind: "type", re: regexp.MustCompile(`(?:pub\s+)?(?:struct|enum|trait)\s+(\w+)`)},
	},
	".tf": {
		{kind: "resource", re: regexp.MustCompile(`resource\s+"(\S+)"`)},
		{kind: "module", re: regexp.MustCompile(`module\s+"(\S+)"`)},
		{kind: "variable", re: regexp.MustCompile(`variable\s+"(\S+)"`)},
	},
	".sh": {
		{kind: "func", re: regexp.MustCompile(`^(\w+)\s*\(\)`)},
	},
}
