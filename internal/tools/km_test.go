package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKMTools(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	t.Setenv("KM_URL", srv.URL)
	ctx := context.Background()

	t.Run("search", func(t *testing.T) {
		out, err := NewKMSearch().Run(ctx, json.RawMessage(`{"query":"auth flow","top_k":3}`))
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if !strings.Contains(out, `"ok":true`) {
			t.Errorf("output = %q", out)
		}
		if gotMethod != http.MethodGet || gotPath != "/api/v1/search" {
			t.Errorf("got %s %s", gotMethod, gotPath)
		}
		if !strings.Contains(gotQuery, "q=auth+flow") || !strings.Contains(gotQuery, "top_k=3") {
			t.Errorf("query = %q", gotQuery)
		}
	})

	t.Run("search requires query", func(t *testing.T) {
		if _, err := NewKMSearch().Run(ctx, json.RawMessage(`{}`)); err == nil {
			t.Fatal("expected error for empty query")
		}
	})

	t.Run("index defaults type to auto", func(t *testing.T) {
		if _, err := NewKMIndex().Run(ctx, json.RawMessage(`{"path":"/tmp/repo"}`)); err != nil {
			t.Fatalf("index: %v", err)
		}
		if gotMethod != http.MethodPost || gotPath != "/api/v1/index" {
			t.Errorf("got %s %s", gotMethod, gotPath)
		}
		if !strings.Contains(gotQuery, "type=auto") || !strings.Contains(gotQuery, "path=%2Ftmp%2Frepo") {
			t.Errorf("query = %q", gotQuery)
		}
	})

	t.Run("blast_radius escapes target", func(t *testing.T) {
		if _, err := NewKMBlastRadius().Run(ctx, json.RawMessage(`{"target":"internal/engine/engine.go"}`)); err != nil {
			t.Fatalf("blast: %v", err)
		}
		// The client sends %2F-escaped segments; net/http decodes them back
		// into URL.Path, so the server observes the original slashes.
		if gotPath != "/api/v1/blast-radius/internal/engine/engine.go" {
			t.Errorf("path = %q", gotPath)
		}
	})

	t.Run("status", func(t *testing.T) {
		if _, err := NewKMStatus().Run(ctx, json.RawMessage(`{}`)); err != nil {
			t.Fatalf("status: %v", err)
		}
		if gotPath != "/api/v1/status" {
			t.Errorf("path = %q", gotPath)
		}
	})
}

func TestKMUnreachable(t *testing.T) {
	// Point at a closed port so the request fails fast with a helpful message.
	t.Setenv("KM_URL", "http://127.0.0.1:1")
	_, err := NewKMStatus().Run(context.Background(), json.RawMessage(`{}`))
	if err == nil || !strings.Contains(err.Error(), "knowledge-master is not reachable") {
		t.Fatalf("expected unreachable error, got %v", err)
	}
}
