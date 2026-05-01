package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFSWriteCreate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	w := NewFSWrite(nil) // auto-approve
	out, err := w.Run(context.Background(), json.RawMessage(`{"path":"`+path+`","mode":"create","content":"hello\n"}`))
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
	w := NewFSWrite(nil)
	_, err := w.Run(context.Background(), json.RawMessage(`{"path":"`+path+`","mode":"create","content":"ok"}`))
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
	w := NewFSWrite(nil)
	out, err := w.Run(context.Background(), json.RawMessage(`{"path":"`+path+`","mode":"patch","old_str":"world","new_str":"go"}`))
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
	w := NewFSWrite(nil)
	_, err := w.Run(context.Background(), json.RawMessage(`{"path":"`+path+`","mode":"patch","old_str":"zzz","new_str":"x"}`))
	if err == nil {
		t.Fatal("expected error for missing old_str")
	}
}

func TestFSWriteDeclined(t *testing.T) {
	decline := func(_ context.Context, _ string) (bool, error) { return false, nil }
	dir := t.TempDir()
	path := filepath.Join(dir, "no.txt")
	w := NewFSWrite(decline)
	out, err := w.Run(context.Background(), json.RawMessage(`{"path":"`+path+`","mode":"create","content":"nope"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "user declined the write" {
		t.Errorf("out = %q", out)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("file should not exist after decline")
	}
}

func TestFSWriteInvalidMode(t *testing.T) {
	w := NewFSWrite(nil)
	_, err := w.Run(context.Background(), json.RawMessage(`{"path":"/tmp/x","mode":"bad"}`))
	if err == nil {
		t.Fatal("expected error for bad mode")
	}
}
