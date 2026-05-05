package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFSWriteCreate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	w := NewFSWrite(nil, nil) // auto-approve
	out, err := w.Run(context.Background(), json.RawMessage(`{"path":"`+jsonPath(path)+`","mode":"create","content":"hello\n"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "hello\n" {
		t.Errorf("file = %q", got)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestFSWriteCreateSubdir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "deep", "file.txt")
	w := NewFSWrite(nil, nil)
	_, err := w.Run(context.Background(), json.RawMessage(`{"path":"`+jsonPath(path)+`","mode":"create","content":"ok"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "ok" {
		t.Errorf("file = %q", got)
	}
}

func TestFSWritePatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "patch.txt")
	if err := writeFile(path, "hello world"); err != nil {
		t.Fatal(err)
	}
	w := NewFSWrite(nil, nil)
	out, err := w.Run(context.Background(), json.RawMessage(`{"path":"`+jsonPath(path)+`","mode":"patch","old_str":"world","new_str":"go"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "hello go" {
		t.Errorf("file = %q", got)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestFSWritePatchNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := writeFile(path, "abc"); err != nil {
		t.Fatal(err)
	}
	w := NewFSWrite(nil, nil)
	_, err := w.Run(context.Background(), json.RawMessage(`{"path":"`+jsonPath(path)+`","mode":"patch","old_str":"zzz","new_str":"x"}`))
	if err == nil {
		t.Fatal("expected error for missing old_str")
	}
}

func TestFSWriteDeclined(t *testing.T) {
	decline := func(_ context.Context, _ string) (bool, error) { return false, nil }
	dir := t.TempDir()
	path := filepath.Join(dir, "no.txt")
	w := NewFSWrite(decline, nil)
	out, err := w.Run(context.Background(), json.RawMessage(`{"path":"`+jsonPath(path)+`","mode":"create","content":"nope"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "user declined the write" {
		t.Errorf("out = %q, want %q", out, "user declined the write")
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("file should NOT exist after user declined")
	}
}

func TestFSWriteInvalidMode(t *testing.T) {
	w := NewFSWrite(nil, nil)
	_, err := w.Run(context.Background(), json.RawMessage(`{"path":"/tmp/x","mode":"bad"}`))
	if err == nil {
		t.Fatal("expected error for bad mode")
	}
}

func TestFSWriteCreateFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file permissions not applicable on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "mode.txt")
	w := NewFSWrite(nil, nil)
	if _, err := w.Run(context.Background(), json.RawMessage(`{"path":"`+jsonPath(path)+`","mode":"create","content":"hello"}`)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("file mode = %o, want 0644", got)
	}
}

func TestFSWritePatchPreservesFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file permissions not applicable on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "exec.sh")
	if err := os.WriteFile(path, []byte("echo hello"), 0o755); err != nil {
		t.Fatal(err)
	}
	w := NewFSWrite(nil, nil)
	if _, err := w.Run(context.Background(), json.RawMessage(`{"path":"`+jsonPath(path)+`","mode":"patch","old_str":"hello","new_str":"world"}`)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("file mode = %o, want 0755 (original should be preserved)", got)
	}
}
