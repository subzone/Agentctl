package desktop

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// kmBaseURL is where the knowledge-master FastAPI server (`km serve`) listens
// by default. knowledge-master is driven over HTTP because it is a separate
// Python/FalkorDB service; the desktop app never embeds it.
const kmBaseURL = "http://127.0.0.1:9999"

// kmHTTP has no overall timeout; every call uses a per-request context so the
// long-running index stream isn't cut off.
var kmHTTP = &http.Client{}

// KMStats mirrors GET /api/v1/status.
type KMStats struct {
	Chunks    int `json:"chunks"`
	Documents int `json:"documents"`
	Repos     int `json:"repos"`
}

// KMNode is one graph vertex from GET /api/graph.
type KMNode struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Label    string `json:"label"`
	Category string `json:"category,omitempty"`
	Source   string `json:"source,omitempty"`
}

// KMLink is one graph edge from GET /api/graph.
type KMLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

// KMGraphData is the node/edge payload from GET /api/graph.
type KMGraphData struct {
	Nodes []KMNode `json:"nodes"`
	Links []KMLink `json:"links"`
}

// KMHealth reports whether the knowledge-master HTTP server is reachable.
type KMHealth struct {
	Available bool   `json:"available"`
	BaseURL   string `json:"baseUrl"`
	Error     string `json:"error,omitempty"`
}

// KMHealth pings the status endpoint with a short timeout.
func (a *App) KMHealth() KMHealth {
	var s KMStats
	if err := a.kmGetJSON(3*time.Second, "/api/v1/status", &s); err != nil {
		return KMHealth{Available: false, BaseURL: kmBaseURL, Error: err.Error()}
	}
	return KMHealth{Available: true, BaseURL: kmBaseURL}
}

// KMStatus returns knowledge base stats (chunks/documents/repos).
func (a *App) KMStatus() (*KMStats, error) {
	var s KMStats
	if err := a.kmGetJSON(10*time.Second, "/api/v1/status", &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// KMGraph returns the graph nodes and links for visualization.
func (a *App) KMGraph() (*KMGraphData, error) {
	var g KMGraphData
	if err := a.kmGetJSON(30*time.Second, "/api/graph", &g); err != nil {
		return nil, err
	}
	return &g, nil
}

func (a *App) kmGetJSON(timeout time.Duration, path string, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, kmBaseURL+path, nil)
	resp, err := kmHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("knowledge-master not reachable at %s — start it with `km start && km serve`", kmBaseURL)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("knowledge-master returned %d for %s", resp.StatusCode, path)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// KMPickDirectory opens a native folder picker for choosing what to index.
func (a *App) KMPickDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select a folder or git repo to index",
	})
}

// KMIndex kicks off indexing of path in the background. type is "auto"
// (repo if a .git exists, else docs), "repo", or "docs". Progress is reported
// to the frontend via events: "kmIndexProgress" {current,total,file},
// "kmIndexDone" {message}, and "kmIndexError" {error}.
func (a *App) KMIndex(path, typ string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("no path provided")
	}
	if typ == "" {
		typ = "auto"
	}
	go a.runKMIndex(path, typ)
	return nil
}

func (a *App) runKMIndex(path, typ string) {
	q := url.Values{}
	q.Set("path", path)
	q.Set("type", typ)

	// Indexing embeds every chunk via Ollama; allow a long deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, kmBaseURL+"/api/index_stream?"+q.Encode(), nil)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := kmHTTP.Do(req)
	if err != nil {
		a.emitKM("kmIndexError", map[string]any{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		a.emitKM("kmIndexError", map[string]any{"error": fmt.Sprintf("knowledge-master returned %d", resp.StatusCode)})
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}
		switch {
		case ev["error"] != nil:
			a.emitKM("kmIndexError", map[string]any{"error": fmt.Sprintf("%v", ev["error"])})
			return
		case ev["done"] != nil:
			a.emitKM("kmIndexDone", map[string]any{"message": ev["message"]})
			return
		default:
			a.emitKM("kmIndexProgress", ev)
		}
	}
	if err := scanner.Err(); err != nil {
		a.emitKM("kmIndexError", map[string]any{"error": err.Error()})
		return
	}
	a.emitKM("kmIndexDone", map[string]any{"message": "Indexing complete"})
}

func (a *App) emitKM(event string, data map[string]any) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, event, data)
}

// KMSearch runs semantic search against the knowledge graph for the UI panel.
func (a *App) KMSearch(query string, topK int) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	if topK <= 0 {
		topK = 8
	}
	q := url.Values{}
	q.Set("q", query)
	q.Set("top_k", fmt.Sprint(topK))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, kmBaseURL+"/api/v1/search?"+q.Encode(), nil)
	resp, err := kmHTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("knowledge-master not reachable at %s — start it with `km start && km serve`", kmBaseURL)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("knowledge-master returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}
