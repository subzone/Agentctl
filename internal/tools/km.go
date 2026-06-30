package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// knowledge-master tools talk to the local FastAPI server started with
// `km serve`. They are HTTP-backed (not MCP) so the desktop app and CLI share
// the single already-running backend that also powers the graph view. All of
// them fail with a clear, actionable message when that server isn't up.

// kmBaseURL returns the knowledge-master HTTP base URL, overridable via KM_URL.
func kmBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("KM_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://127.0.0.1:9999"
}

// kmClient handles the quick read endpoints. Indexing uses its own client
// with no timeout because embedding a repo can take minutes.
var (
	kmClient      = &http.Client{Timeout: 60 * time.Second}
	kmIndexClient = &http.Client{}
)

func kmGet(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, kmBaseURL()+path, nil)
	if err != nil {
		return "", err
	}
	resp, err := kmClient.Do(req)
	if err != nil {
		return "", kmUnreachable(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("knowledge-master returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}

func kmUnreachable(err error) error {
	return fmt.Errorf("knowledge-master is not reachable at %s — start it with `km start && km serve` (%v)", kmBaseURL(), err)
}

// --- km_search ---

// KMSearchTool runs semantic + graph search over the knowledge base.
type KMSearchTool struct{}

func NewKMSearch() *KMSearchTool { return &KMSearchTool{} }

func (t *KMSearchTool) Name() string { return "km_search" }

func (t *KMSearchTool) Description() string {
	return "Search the knowledge-master graph: semantic search over the user's indexed code, " +
		"docs, and notes, returned with graph context (repo, author, relationships). Use this to " +
		"recall how the user's projects work before answering. Requires the local knowledge-master " +
		"server (`km serve`)."
}

func (t *KMSearchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "query": {"type": "string", "description": "Natural-language search query"},
    "top_k": {"type": "integer", "description": "Maximum number of results (default 8)"},
    "source_type": {"type": "string", "description": "Optional filter: code, docs, email, or infra"}
  },
  "required": ["query"]
}`)
}

func (t *KMSearchTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var a struct {
		Query      string `json:"query"`
		TopK       int    `json:"top_k"`
		SourceType string `json:"source_type"`
	}
	if err := json.Unmarshal(input, &a); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if strings.TrimSpace(a.Query) == "" {
		return "", fmt.Errorf("query is required")
	}
	if a.TopK <= 0 {
		a.TopK = 8
	}
	q := url.Values{}
	q.Set("q", a.Query)
	q.Set("top_k", fmt.Sprint(a.TopK))
	if a.SourceType != "" {
		q.Set("source_type", a.SourceType)
	}
	return kmGet(ctx, "/api/v1/search?"+q.Encode())
}

// --- km_index ---

// KMIndexTool adds a repo or directory to the knowledge graph.
type KMIndexTool struct{}

func NewKMIndex() *KMIndexTool { return &KMIndexTool{} }

func (t *KMIndexTool) Name() string { return "km_index" }

func (t *KMIndexTool) Description() string {
	return "Index a folder, git repo, or set of documents into the knowledge-master graph so it " +
		"becomes searchable later. Pass an absolute path. This embeds every file and can take a " +
		"while for large repos. Requires the local knowledge-master server (`km serve`)."
}

func (t *KMIndexTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {"type": "string", "description": "Absolute path to a folder or git repo to index"},
    "type": {"type": "string", "description": "auto (default), repo, or docs", "enum": ["auto", "repo", "docs"]}
  },
  "required": ["path"]
}`)
}

func (t *KMIndexTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var a struct {
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(input, &a); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if strings.TrimSpace(a.Path) == "" {
		return "", fmt.Errorf("path is required")
	}
	if a.Type == "" {
		a.Type = "auto"
	}
	q := url.Values{}
	q.Set("path", a.Path)
	q.Set("type", a.Type)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kmBaseURL()+"/api/v1/index?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := kmIndexClient.Do(req)
	if err != nil {
		return "", kmUnreachable(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("knowledge-master returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}

// --- km_blast_radius ---

// KMBlastRadiusTool reports what depends on a file/function/service.
type KMBlastRadiusTool struct{}

func NewKMBlastRadius() *KMBlastRadiusTool { return &KMBlastRadiusTool{} }

func (t *KMBlastRadiusTool) Name() string { return "km_blast_radius" }

func (t *KMBlastRadiusTool) Description() string {
	return "Analyze impact: list everything that depends on a given file, function, service, or " +
		"technology in the knowledge graph. Use before suggesting a change to understand what it " +
		"could break. Requires the local knowledge-master server (`km serve`)."
}

func (t *KMBlastRadiusTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "target": {"type": "string", "description": "File path, function, service, or technology name"}
  },
  "required": ["target"]
}`)
}

func (t *KMBlastRadiusTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var a struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(input, &a); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if strings.TrimSpace(a.Target) == "" {
		return "", fmt.Errorf("target is required")
	}
	return kmGet(ctx, "/api/v1/blast-radius/"+url.PathEscape(a.Target))
}

// --- km_status ---

// KMStatusTool reports knowledge base statistics.
type KMStatusTool struct{}

func NewKMStatus() *KMStatusTool { return &KMStatusTool{} }

func (t *KMStatusTool) Name() string { return "km_status" }

func (t *KMStatusTool) Description() string {
	return "Get knowledge-master statistics: number of indexed chunks, documents, and repos. " +
		"Useful to check whether something is indexed yet. Requires the local knowledge-master " +
		"server (`km serve`)."
}

func (t *KMStatusTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type": "object", "properties": {}}`)
}

func (t *KMStatusTool) Run(ctx context.Context, _ json.RawMessage) (string, error) {
	return kmGet(ctx, "/api/v1/status")
}

// KMTools returns the knowledge-master tool set. The desktop app merges these
// into every session so the chat agent can search and grow the graph; the CLI
// can reference them via an agent's tools allowlist.
func KMTools() []Tool {
	return []Tool{NewKMSearch(), NewKMIndex(), NewKMBlastRadius(), NewKMStatus()}
}
