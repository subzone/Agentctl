package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeSearchText(t *testing.T) {
	tool := NewCodeSearch()
	input, _ := json.Marshal(map[string]string{
		"query": "func NewCodeSearch",
		"type":  "text",
		"path":  ".",
	})
	out, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(out, "codesearch.go") {
		t.Errorf("expected to find codesearch.go in results: %s", out)
	}
}

func TestCodeSearchSymbol(t *testing.T) {
	tool := NewCodeSearch()
	input, _ := json.Marshal(map[string]string{
		"query": "CodeSearchTool",
		"type":  "symbol",
		"path":  ".",
	})
	out, err := tool.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(out, "CodeSearchTool") {
		t.Errorf("expected to find CodeSearchTool symbol: %s", out)
	}
}

func TestBuildIndex(t *testing.T) {
	// Create a temp dir with a Go file.
	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	os.WriteFile(goFile, []byte(`package main

func hello() {}
func world() {}
type Config struct {}
`), 0644)

	idx, err := buildIndex(dir)
	if err != nil {
		t.Fatalf("buildIndex failed: %v", err)
	}
	if len(idx.symbols) < 3 {
		t.Errorf("expected at least 3 symbols, got %d", len(idx.symbols))
	}

	result := idx.search("hello")
	if !strings.Contains(result, "hello") {
		t.Errorf("search for hello failed: %s", result)
	}

	result = idx.search("Config")
	if !strings.Contains(result, "Config") {
		t.Errorf("search for Config failed: %s", result)
	}

	result = idx.search("nonexistent_xyz")
	if !strings.Contains(result, "no symbols") {
		t.Errorf("expected no matches: %s", result)
	}
}

func TestCodeSearchMissingQuery(t *testing.T) {
	tool := NewCodeSearch()
	input, _ := json.Marshal(map[string]string{})
	_, err := tool.Run(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing query")
	}
}
