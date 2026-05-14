package audit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/subzone/Agentctl/internal/ports"
)

type memSink struct {
	mu     sync.Mutex
	events []ports.AuditEvent
}

func (m *memSink) Emit(_ context.Context, ev ports.AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, ev)
	return nil
}
func (m *memSink) Flush(_ context.Context) error { return nil }
func (m *memSink) Close() error                  { return nil }

func TestBatchSinkFlushBySize(t *testing.T) {
	base := &memSink{}
	sinkAny, err := NewBatchSink(base, 2, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	sink := sinkAny.(*batchSink)
	defer sink.Close()

	_ = sink.Emit(context.Background(), ports.AuditEvent{Type: "a"})
	_ = sink.Emit(context.Background(), ports.AuditEvent{Type: "b"})

	time.Sleep(30 * time.Millisecond)
	base.mu.Lock()
	defer base.mu.Unlock()
	if len(base.events) != 2 {
		t.Fatalf("events=%d want 2", len(base.events))
	}
}

func TestBatchSinkFlushOnClose(t *testing.T) {
	base := &memSink{}
	sinkAny, err := NewBatchSink(base, 10, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	sink := sinkAny.(*batchSink)
	_ = sink.Emit(context.Background(), ports.AuditEvent{Type: "x"})
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	base.mu.Lock()
	defer base.mu.Unlock()
	if len(base.events) != 1 {
		t.Fatalf("events=%d want 1", len(base.events))
	}
}
