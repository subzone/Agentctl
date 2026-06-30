package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type KnowledgeTool struct{}

func NewKnowledge() *KnowledgeTool { return &KnowledgeTool{} }

func (k *KnowledgeTool) Name() string { return "knowledge" }
func (k *KnowledgeTool) Description() string {
	return "Offline local knowledge base (keyword index) for code/docs. Operations: index, search, blast_radius, who_owns, status. " +
		"For a full knowledge graph (semantic search, real blast radius, conventions), install knowledge-master and use the km__ MCP tools instead."
}

func (k *KnowledgeTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "operation": {"type": "string", "enum": ["index", "search", "blast_radius", "who_owns", "status"]},
    "path": {"type": "string", "description": "Directory to index or file to check"},
    "query": {"type": "string", "description": "Search query (for search operation)"},
    "top_k": {"type": "integer", "description": "Number of results (default 5)"}
  },
  "required": ["operation"]
}`)
}

func (k *KnowledgeTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Operation string `json:"operation"`
		Path      string `json:"path"`
		Query     string `json:"query"`
		TopK      int    `json:"top_k"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.TopK <= 0 {
		args.TopK = 5
	}

	db, err := k.openDB()
	if err != nil {
		return "", err
	}
	defer db.Close()

	switch args.Operation {
	case "index":
		return k.index(db, args.Path)
	case "search":
		return k.search(db, args.Query, args.TopK)
	case "blast_radius":
		return k.blastRadius(db, args.Path)
	case "who_owns":
		return k.whoOwns(ctx, args.Path)
	case "status":
		return k.status(db)
	default:
		return "", fmt.Errorf("unknown operation %q", args.Operation)
	}
}

func (k *KnowledgeTool) dbPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "m", "knowledge.db")
}

func (k *KnowledgeTool) openDB() (*sql.DB, error) {
	p := k.dbPath()
	os.MkdirAll(filepath.Dir(p), 0o755)
	db, err := sql.Open("sqlite", p)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS chunks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_path TEXT NOT NULL,
		line_start INTEGER,
		line_end INTEGER,
		content TEXT,
		keywords TEXT,
		indexed_at TEXT
	)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

var indexExts = map[string]bool{
	".go": true, ".md": true, ".yaml": true, ".yml": true,
	".json": true, ".py": true, ".js": true, ".ts": true,
}

var tokenRe = regexp.MustCompile(`[a-zA-Z0-9_]+`)

func extractKeywords(content string) string {
	seen := map[string]bool{}
	var kws []string
	for _, tok := range tokenRe.FindAllString(content, -1) {
		low := strings.ToLower(tok)
		if len(low) < 2 || seen[low] {
			continue
		}
		seen[low] = true
		kws = append(kws, low)
	}
	return strings.Join(kws, " ")
}

func (k *KnowledgeTool) index(db *sql.DB, dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("path required for index operation")
	}
	// Clear old entries for this dir
	db.Exec("DELETE FROM chunks WHERE file_path LIKE ?", dir+"%")

	var count int
	chunkSize := 50 // lines per chunk
	now := time.Now().Format(time.RFC3339)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !indexExts[filepath.Ext(path)] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for i := 0; i < len(lines); i += chunkSize {
			end := i + chunkSize
			if end > len(lines) {
				end = len(lines)
			}
			content := strings.Join(lines[i:end], "\n")
			kw := extractKeywords(content)
			db.Exec("INSERT INTO chunks (file_path, line_start, line_end, content, keywords, indexed_at) VALUES (?,?,?,?,?,?)",
				path, i+1, end, content, kw, now)
			count++
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Indexed %d chunks from %s", count, dir), nil
}

func (k *KnowledgeTool) search(db *sql.DB, query string, topK int) (string, error) {
	if query == "" {
		return "", fmt.Errorf("query required for search operation")
	}
	terms := tokenRe.FindAllString(strings.ToLower(query), -1)
	if len(terms) == 0 {
		return "No valid search terms", nil
	}

	// Build query: match any term, rank by count of terms matched
	var conditions []string
	var params []any
	for _, t := range terms {
		conditions = append(conditions, "keywords LIKE ?")
		params = append(params, "%"+t+"%")
	}

	scoreExprs := make([]string, 0, len(conditions))
	for _, c := range conditions {
		scoreExprs = append(scoreExprs, "CASE WHEN "+c+" THEN 1 ELSE 0 END")
	}
	// The query is assembled only from constant SQL fragments and bound `?`
	// placeholders; user input flows in via params, never string interpolation.
	//nolint:gosec // G201: no user data is formatted into the SQL string.
	sqlQ := fmt.Sprintf(`SELECT file_path, line_start, line_end, content,
		(%s) as score
		FROM chunks WHERE %s ORDER BY score DESC LIMIT ?`,
		strings.Join(scoreExprs, "+"),
		strings.Join(conditions, " OR "))

	// duplicate params for score calc and WHERE (fresh slice to avoid aliasing)
	allParams := make([]any, 0, len(params)*2+1)
	allParams = append(allParams, params...)
	allParams = append(allParams, params...)
	allParams = append(allParams, topK)

	rows, err := db.Query(sqlQ, allParams...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var sb strings.Builder
	for rows.Next() {
		var fp, content string
		var ls, le, score int
		rows.Scan(&fp, &ls, &le, &content, &score)
		snippet := content
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		fmt.Fprintf(&sb, "--- %s [L%d-%d] (score:%d)\n%s\n\n", fp, ls, le, score, snippet)
	}
	if sb.Len() == 0 {
		return "No results found", nil
	}
	return sb.String(), nil
}

func (k *KnowledgeTool) blastRadius(db *sql.DB, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path required for blast_radius operation")
	}
	base := filepath.Base(path)
	pkg := strings.TrimSuffix(base, filepath.Ext(base))

	rows, err := db.Query(`SELECT DISTINCT file_path FROM chunks WHERE file_path != ? AND (content LIKE ? OR content LIKE ?)`,
		path, "%"+base+"%", "%"+pkg+"%")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var fp string
		rows.Scan(&fp)
		files = append(files, fp)
	}
	if len(files) == 0 {
		return "No references found", nil
	}
	return fmt.Sprintf("Files referencing %s:\n%s", base, strings.Join(files, "\n")), nil
}

func (k *KnowledgeTool) whoOwns(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path required for who_owns operation")
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", "blame", "--porcelain", path)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git blame failed: %w", err)
	}

	counts := map[string]int{}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "author ") {
			counts[strings.TrimPrefix(line, "author ")]++
		}
	}
	if len(counts) == 0 {
		return "No authors found", nil
	}

	var best string
	var max int
	for a, c := range counts {
		if c > max {
			best, max = a, c
		}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Primary: %s (%d lines)\n", best, max)
	for a, c := range counts {
		if a != best {
			fmt.Fprintf(&sb, "  %s: %d lines\n", a, c)
		}
	}
	return sb.String(), nil
}

func (k *KnowledgeTool) status(db *sql.DB) (string, error) {
	var files, chunks int
	db.QueryRow("SELECT COUNT(DISTINCT file_path) FROM chunks").Scan(&files)
	db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&chunks)

	var size int64
	if info, err := os.Stat(k.dbPath()); err == nil {
		size = info.Size()
	}
	return fmt.Sprintf("Files: %d\nChunks: %d\nDB size: %d bytes", files, chunks, size), nil
}
