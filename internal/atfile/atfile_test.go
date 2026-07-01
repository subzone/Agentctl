package atfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subzone/Agentctl/internal/atfile"
)

func TestExpand(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(p, []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	out, included := atfile.Expand("see @hello.txt please")
	if len(included) != 1 || included[0] != "hello.txt" {
		t.Fatalf("included: %v", included)
	}
	if !strings.Contains(out, "world") || !strings.Contains(out, "[File: hello.txt]") {
		t.Fatalf("expand: %q", out)
	}
}

func TestSearch(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "node_modules", "x"), 0o755)

	hits := atfile.Search(dir, "main", 5)
	if len(hits) == 0 || hits[0].Name != "main.go" {
		t.Fatalf("hits: %+v", hits)
	}
}
