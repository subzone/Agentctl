package adapters

import (
	"context"
	"testing"

	"github.com/subzone/Agentctl/internal/ports"
)

func TestMemoryStoreSaveAndLoad(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	data := &ports.SessionData{
		Provider: "anthropic",
		Model:    "claude-sonnet-4-6",
		Messages: []ports.Message{{Role: "user", Content: []ports.ContentBlock{{Type: "text", Text: "hi"}}}},
	}
	if err := s.SaveSession(ctx, "s1", data); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.LoadSession(ctx, "s1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Provider != "anthropic" || got.Model != "claude-sonnet-4-6" {
		t.Errorf("got %+v", got)
	}
	if len(got.Messages) != 1 || got.Messages[0].Content[0].Text != "hi" {
		t.Errorf("messages = %+v", got.Messages)
	}
}

func TestMemoryStoreNotFound(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.LoadSession(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*ports.ErrNotFound); !ok {
		t.Errorf("got %T, want *ErrNotFound", err)
	}
}

func TestMemoryStoreListAndDelete(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	s.SaveSession(ctx, "a", &ports.SessionData{})
	s.SaveSession(ctx, "b", &ports.SessionData{})

	ids, _ := s.ListSessions(ctx)
	if len(ids) != 2 {
		t.Errorf("list = %v", ids)
	}

	s.DeleteSession(ctx, "a")
	ids, _ = s.ListSessions(ctx)
	if len(ids) != 1 {
		t.Errorf("after delete = %v", ids)
	}
}
