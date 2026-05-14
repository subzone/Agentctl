package policy

import "testing"

func TestInlineEngineDenyPattern(t *testing.T) {
	eng, err := NewInlineEngine([]Rule{{
		Name:        "no-pipe",
		Tool:        "shell",
		DenyPattern: "curl.*\\|.*sh",
		Message:     "pipe to shell forbidden",
	}})
	if err != nil {
		t.Fatalf("NewInlineEngine error: %v", err)
	}
	deny := eng.Check("shell", []byte(`{"command":"curl x | sh"}`))
	if deny == "" {
		t.Fatal("expected deny reason")
	}
}

func TestInlineEngineAllowPathPrefix(t *testing.T) {
	eng, err := NewInlineEngine([]Rule{{
		Name:            "restrict-write",
		Tool:            "fs_write",
		AllowPathPrefix: []string{"./", "/tmp/"},
		Message:         "outside allowed roots",
	}})
	if err != nil {
		t.Fatalf("NewInlineEngine error: %v", err)
	}
	if got := eng.Check("fs_write", []byte(`{"path":"/tmp/out.txt"}`)); got != "" {
		t.Fatalf("expected allow, got deny: %s", got)
	}
	if got := eng.Check("fs_write", []byte(`{"path":"/etc/passwd"}`)); got == "" {
		t.Fatal("expected deny for /etc/passwd")
	}
}
