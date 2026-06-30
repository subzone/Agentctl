package a2a

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/subzone/Agentctl/internal/config"
)

func newTestServer(exec Executor) *httptest.Server {
	spec := &config.AgentSpec{
		Meta:  config.Meta{Name: "echo", Type: config.TypeAgent, Description: "echoes input"},
		Model: "groq/llama-3.3-70b-versatile",
		Tools: []string{"fs_read", "shell"},
	}
	card := CardFromAgent(spec, "", "1.2.3")
	return httptest.NewServer(NewServer(card, exec).Handler())
}

func TestCardRoundTrip(t *testing.T) {
	srv := newTestServer(func(_ context.Context, task string) (string, error) {
		return "ok", nil
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	card, err := c.Card(context.Background())
	if err != nil {
		t.Fatalf("Card: %v", err)
	}
	if card.Name != "echo" {
		t.Errorf("name = %q, want echo", card.Name)
	}
	if card.Version != "1.2.3" {
		t.Errorf("version = %q, want 1.2.3", card.Version)
	}
	if len(card.Skills) != 1 || card.Skills[0].ID != "echo" {
		t.Errorf("skills = %+v", card.Skills)
	}
	if card.Capabilities.Streaming {
		t.Error("streaming should be false")
	}
}

func TestSendRoundTrip(t *testing.T) {
	srv := newTestServer(func(_ context.Context, task string) (string, error) {
		return "you said: " + task, nil
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	reply, err := c.Send(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if reply != "you said: hello world" {
		t.Errorf("reply = %q", reply)
	}
}

func TestSendExecutorError(t *testing.T) {
	srv := newTestServer(func(_ context.Context, _ string) (string, error) {
		return "", errors.New("boom")
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Send(context.Background(), "trigger")
	if err == nil {
		t.Fatal("expected error from remote executor")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want it to contain remote message", err)
	}
}

func TestSendEmptyMessageRejected(t *testing.T) {
	called := false
	srv := newTestServer(func(_ context.Context, _ string) (string, error) {
		called = true
		return "", nil
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Send(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty message")
	}
	if called {
		t.Error("executor should not run for an empty message")
	}
}
