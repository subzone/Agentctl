package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCommandToolEchoesStdin(t *testing.T) {
	tool := NewCommandTool("echo", "echoes input", []string{"cat"}, nil, nil, 5*time.Second)
	out, err := tool.Run(context.Background(), json.RawMessage(`{"a":1}`))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out, `"a":1`) {
		t.Errorf("output = %q, want it to contain the stdin JSON", out)
	}
}

func TestCommandToolExitErrorIsInline(t *testing.T) {
	tool := NewCommandTool("fail", "fails", []string{"bash", "-c", "echo boom; exit 3"}, nil, nil, 5*time.Second)
	out, err := tool.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("exit codes should surface inline, not as error: %v", err)
	}
	if !strings.Contains(out, "boom") || !strings.Contains(out, "exit error") {
		t.Errorf("output = %q", out)
	}
}

func TestCommandToolTimeout(t *testing.T) {
	tool := NewCommandTool("slow", "sleeps", []string{"sleep", "5"}, nil, nil, 100*time.Millisecond)
	_, err := tool.Run(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestCommandToolEnv(t *testing.T) {
	tool := NewCommandTool("env", "reads env", []string{"bash", "-c", "echo $MY_VAR"}, map[string]string{"MY_VAR": "hello"}, nil, 5*time.Second)
	out, err := tool.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("output = %q, want env var value", out)
	}
}
