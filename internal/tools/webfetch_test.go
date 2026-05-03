package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebFetchHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html><head><title>Test</title>
<script>var x = 1;</script>
<style>body { color: red; }</style>
</head><body>
<h1>Hello World</h1>
<p>This is a <strong>test</strong> page.</p>
<p>Second paragraph.</p>
</body></html>`))
	}))
	defer srv.Close()

	tool := NewWebFetch()
	input, _ := json.Marshal(map[string]string{"url": srv.URL})
	out, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(out, "Hello World") {
		t.Errorf("missing 'Hello World' in output: %s", out)
	}
	if !strings.Contains(out, "test page") {
		t.Errorf("missing 'test page' in output: %s", out)
	}
	if strings.Contains(out, "var x = 1") {
		t.Error("script content should be stripped")
	}
	if strings.Contains(out, "color: red") {
		t.Error("style content should be stripped")
	}
}

func TestWebFetchPlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("plain text response"))
	}))
	defer srv.Close()

	tool := NewWebFetch()
	input, _ := json.Marshal(map[string]string{"url": srv.URL})
	out, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if out != "plain text response" {
		t.Errorf("got %q, want %q", out, "plain text response")
	}
}

func TestWebFetchTruncation(t *testing.T) {
	big := strings.Repeat("x", 50000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(big))
	}))
	defer srv.Close()

	tool := NewWebFetch()
	input, _ := json.Marshal(map[string]string{"url": srv.URL})
	out, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.HasSuffix(out, "[output truncated]") {
		t.Error("expected truncation marker")
	}
}

func TestWebFetchMissingURL(t *testing.T) {
	tool := NewWebFetch()
	input, _ := json.Marshal(map[string]string{})
	_, err := tool.Run(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing URL")
	}
}

func TestWebFetch404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	tool := NewWebFetch()
	input, _ := json.Marshal(map[string]string{"url": srv.URL})
	_, err := tool.Run(context.Background(), input)
	if err == nil {
		t.Error("expected error for 404")
	}
}

func TestHtmlToText(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{"entities", "5 &gt; 3 &amp; 2 &lt; 4", "5 > 3 & 2 < 4"},
		{"nested tags", "<div><p>hello <b>world</b></p></div>", "hello world"},
		{"comment", "before<!-- hidden -->after", "beforeafter"},
	}
	for _, tt := range tests {
		got := htmlToText([]byte(tt.html))
		got = strings.TrimSpace(got)
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
