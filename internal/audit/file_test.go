package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/subzone/Agentctl/internal/ports"
)

func TestFileSinkWritesJSONL(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "audit.jsonl")
	sink, err := NewFileSink(p, "secret")
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()

	err = sink.Emit(context.Background(), ports.AuditEvent{
		Type:      "session_start",
		SessionID: "s1",
		Model:     "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(b))
	if line == "" {
		t.Fatal("expected one line")
	}
	var ev ports.AuditEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Type != "session_start" {
		t.Fatalf("type=%q", ev.Type)
	}
	if !strings.HasPrefix(ev.HMAC, "sha256:") {
		t.Fatalf("hmac=%q", ev.HMAC)
	}
}
