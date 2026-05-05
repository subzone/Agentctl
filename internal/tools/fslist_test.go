package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFSListFlat(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	writeFile(filepath.Join(dir, "a.txt"), "a")
	writeFile(filepath.Join(dir, "b.txt"), "b")

	l := NewFSList()
	out, err := l.Run(context.Background(), json.RawMessage(`{"path":"`+jsonPath(dir)+`"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "a.txt") || !strings.Contains(out, "b.txt") {
		t.Errorf("missing files: %q", out)
	}
	if !strings.Contains(out, "sub/") && !strings.Contains(out, "sub\\") {
		t.Errorf("missing dir with trailing slash: %q", out)
	}
}

func TestFSListRecursive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755)
	writeFile(filepath.Join(dir, "top.txt"), "t")
	writeFile(filepath.Join(dir, "a", "mid.txt"), "m")
	writeFile(filepath.Join(dir, "a", "b", "deep.txt"), "d")

	l := NewFSList()
	out, err := l.Run(context.Background(), json.RawMessage(`{"path":"`+jsonPath(dir)+`","recursive":true}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "top.txt") || !strings.Contains(out, "mid.txt") || !strings.Contains(out, "deep.txt") {
		t.Errorf("missing files: %q", out)
	}
}

func TestFSListSkipsGit(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0o755)
	writeFile(filepath.Join(dir, ".git", "HEAD"), "ref")
	writeFile(filepath.Join(dir, "real.txt"), "r")

	l := NewFSList()
	out, err := l.Run(context.Background(), json.RawMessage(`{"path":"`+jsonPath(dir)+`","recursive":true}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(out, "HEAD") || strings.Contains(out, "objects") {
		t.Errorf(".git should be skipped: %q", out)
	}
	if !strings.Contains(out, "real.txt") {
		t.Errorf("missing real.txt: %q", out)
	}
}

func TestFSListMissingPath(t *testing.T) {
	l := NewFSList()
	_, err := l.Run(context.Background(), json.RawMessage(`{"path":"/nonexistent/zzz"}`))
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestFSListEmptyPath(t *testing.T) {
	l := NewFSList()
	_, err := l.Run(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}
