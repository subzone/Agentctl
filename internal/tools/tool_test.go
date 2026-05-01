package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeTool struct {
	name string
	run  func(ctx context.Context, input json.RawMessage) (string, error)
}

func (f *fakeTool) Name() string                  { return f.name }
func (f *fakeTool) Description() string           { return "fake" }
func (f *fakeTool) InputSchema() json.RawMessage  { return json.RawMessage(`{"type":"object"}`) }
func (f *fakeTool) Run(ctx context.Context, in json.RawMessage) (string, error) {
	return f.run(ctx, in)
}

func TestRegistryGetAndAll(t *testing.T) {
	a := &fakeTool{name: "a", run: func(context.Context, json.RawMessage) (string, error) { return "ok", nil }}
	b := &fakeTool{name: "b", run: func(context.Context, json.RawMessage) (string, error) { return "ok", nil }}
	r := NewRegistry(a, b)

	if got, ok := r.Get("a"); !ok || got != a {
		t.Errorf("Get(a) failed")
	}
	if _, ok := r.Get("zzz"); ok {
		t.Error("Get(zzz) should miss")
	}
	if all := r.All(); len(all) != 2 {
		t.Errorf("All len = %d, want 2", len(all))
	}
}

func TestRegistryFilter(t *testing.T) {
	r := NewRegistry(&fakeTool{name: "shell"}, &fakeTool{name: "fs_read"})
	out, missing := r.Filter([]string{"shell", "fs.write", "fs_read"})

	if _, ok := out.Get("shell"); !ok {
		t.Error("filter missing shell")
	}
	if _, ok := out.Get("fs_read"); !ok {
		t.Error("filter missing fs.read")
	}
	if _, ok := out.Get("fs.write"); ok {
		t.Error("filter should not contain fs.write")
	}
	if len(missing) != 1 || missing[0] != "fs.write" {
		t.Errorf("missing = %v, want [fs.write]", missing)
	}
}

func TestRegistryRunUnknown(t *testing.T) {
	r := NewRegistry()
	_, err := r.Run(context.Background(), "nope", nil)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestShellSimple(t *testing.T) {
	out, err := NewShell().Run(context.Background(), json.RawMessage(`{"command":"echo hi"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "hi\n" {
		t.Errorf("output = %q, want %q", out, "hi\n")
	}
}

func TestShellNonZeroExit(t *testing.T) {
	out, err := NewShell().Run(context.Background(), json.RawMessage(`{"command":"sh -c 'echo nope >&2; exit 7'"}`))
	if err != nil {
		t.Fatalf("non-zero exit should not be a tool error, got %v", err)
	}
	if want := "nope\n\n[exit 7]"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestShellInvalidInput(t *testing.T) {
	_, err := NewShell().Run(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestShellTruncates(t *testing.T) {
	s := NewShell()
	s.MaxOut = 4
	out, err := s.Run(context.Background(), json.RawMessage(`{"command":"printf abcdefghij"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "abcd\n[output truncated]" {
		t.Errorf("output = %q", out)
	}
}

func TestFSReadHappy(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/hello.txt"
	if err := writeFile(path, "hello world\n"); err != nil {
		t.Fatal(err)
	}
	out, err := NewFSRead().Run(context.Background(), json.RawMessage(`{"path":"`+path+`"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "hello world\n" {
		t.Errorf("got %q", out)
	}
}

func TestFSReadMissing(t *testing.T) {
	_, err := NewFSRead().Run(context.Background(), json.RawMessage(`{"path":"/nonexistent/zzz"}`))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFSReadTruncates(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/big.txt"
	if err := writeFile(path, "0123456789"); err != nil {
		t.Fatal(err)
	}
	f := NewFSRead()
	f.MaxBytes = 4
	out, err := f.Run(context.Background(), json.RawMessage(`{"path":"`+path+`"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "0123\n[output truncated]" {
		t.Errorf("got %q", out)
	}
}

func TestFSReadInvalidInput(t *testing.T) {
	_, err := NewFSRead().Run(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestBuiltinsContainsShellAndFSRead(t *testing.T) {
	r := Builtins(nil, nil)
	if _, ok := r.Get("shell"); !ok {
		t.Error("missing shell")
	}
	if _, ok := r.Get("fs_read"); !ok {
		t.Error("missing fs.read")
	}
}
