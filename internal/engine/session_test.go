package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/subzone/m/internal/llm"
	"github.com/subzone/m/internal/tools"
)

// recordingProvider captures every Stream call's request, then replays a
// scripted sequence of events for that turn.
type recordingProvider struct {
	turns    [][]llm.Event
	requests []llm.Request
}

func (r *recordingProvider) Stream(_ context.Context, req llm.Request) (<-chan llm.Event, error) {
	r.requests = append(r.requests, req)
	turn := r.turns[0]
	r.turns = r.turns[1:]
	ch := make(chan llm.Event, len(turn))
	for _, ev := range turn {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func TestSessionTwoStepsCarryHistory(t *testing.T) {
	p := &recordingProvider{turns: [][]llm.Event{
		{{Kind: llm.EventText, Text: "hi"}, {Kind: llm.EventDone, StopReason: "end_turn"}},
		{{Kind: llm.EventText, Text: "still here"}, {Kind: llm.EventDone, StopReason: "end_turn"}},
	}}
	var out bytes.Buffer
	s := NewSession(Config{Provider: p, Model: "m", Out: &out})

	if err := s.Step(context.Background(), "hello"); err != nil {
		t.Fatalf("Step 1: %v", err)
	}
	if err := s.Step(context.Background(), "again"); err != nil {
		t.Fatalf("Step 2: %v", err)
	}

	// Second turn's request should include both prior turns plus the new
	// user message: u, a, u — three messages.
	if len(p.requests) != 2 {
		t.Fatalf("requests = %d", len(p.requests))
	}
	second := p.requests[1].Messages
	if len(second) != 3 {
		t.Fatalf("turn-2 messages = %d, want 3 (user, assistant, user)", len(second))
	}
	if second[0].Role != llm.RoleUser || second[1].Role != llm.RoleAssistant || second[2].Role != llm.RoleUser {
		t.Errorf("roles = %s/%s/%s", second[0].Role, second[1].Role, second[2].Role)
	}
	if second[2].Content[0].Text != "again" {
		t.Errorf("latest user msg = %q", second[2].Content[0].Text)
	}
	// Output should include both replies.
	if got := out.String(); !strings.Contains(got, "hi") || !strings.Contains(got, "still here") {
		t.Errorf("out = %q", got)
	}
}

func TestSessionMessagesCopy(t *testing.T) {
	p := &recordingProvider{turns: [][]llm.Event{
		{{Kind: llm.EventText, Text: "x"}, {Kind: llm.EventDone}},
	}}
	s := NewSession(Config{Provider: p, Model: "m"})
	_ = s.Step(context.Background(), "hi")
	got := s.Messages()
	if len(got) != 2 {
		t.Fatalf("messages = %d", len(got))
	}
	// Mutating the returned slice must not affect the session.
	got[0].Content[0].Text = "MUTATED"
	if s.Messages()[0].Content[0].Text == "MUTATED" {
		t.Error("Messages() returned a shallow alias; should be a copy")
	}
}

func TestSessionReset(t *testing.T) {
	p := &recordingProvider{turns: [][]llm.Event{
		{{Kind: llm.EventText, Text: "x"}, {Kind: llm.EventDone}},
		{{Kind: llm.EventText, Text: "y"}, {Kind: llm.EventDone}},
	}}
	s := NewSession(Config{Provider: p, Model: "m"})
	_ = s.Step(context.Background(), "hi")
	s.Reset()
	if len(s.Messages()) != 0 {
		t.Errorf("history not cleared: %d", len(s.Messages()))
	}
	_ = s.Step(context.Background(), "hello")
	// After reset + one step, the second request should include only the new user msg.
	last := p.requests[len(p.requests)-1].Messages
	if len(last) != 1 || last[0].Content[0].Text != "hello" {
		t.Errorf("post-reset messages = %+v", last)
	}
}

func TestSessionTruncateKeepsLastN(t *testing.T) {
	p := &recordingProvider{turns: [][]llm.Event{
		{{Kind: llm.EventDone}},
		{{Kind: llm.EventDone}},
		{{Kind: llm.EventDone}},
		{{Kind: llm.EventDone}},
	}}
	s := NewSession(Config{Provider: p, Model: "m"})
	for i, q := range []string{"q1", "q2", "q3", "q4"} {
		if err := s.Step(context.Background(), q); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	// Before: 4 user + 4 assistant = 8 messages.
	if got := len(s.Messages()); got != 8 {
		t.Fatalf("pre-truncate = %d", got)
	}
	s.Truncate(2)
	got := s.Messages()
	if len(got) != 4 {
		t.Errorf("post-truncate = %d, want 4 (last 2 exchanges)", len(got))
	}
	if got[0].Content[0].Text != "q3" {
		t.Errorf("oldest kept = %q, want q3", got[0].Content[0].Text)
	}
}

func TestSessionTruncateNoOpWhenSmall(t *testing.T) {
	p := &recordingProvider{turns: [][]llm.Event{{{Kind: llm.EventDone}}}}
	s := NewSession(Config{Provider: p, Model: "m"})
	_ = s.Step(context.Background(), "q1")
	before := s.Messages()
	s.Truncate(10)
	after := s.Messages()
	if len(before) != len(after) {
		t.Errorf("truncate should be no-op when history below limit")
	}
}

func TestSessionTruncateKeepsToolPair(t *testing.T) {
	// Construct a session where exchange 1 has a tool call (so the user
	// messages aren't all "exchange-start"); truncate to 1 and verify
	// the tool_use/tool_result pair stays together.
	p := &recordingProvider{turns: [][]llm.Event{
		// Exchange 1: text → tool_use → text
		{
			{Kind: llm.EventToolCall, ToolID: "tu", ToolName: "shell", ToolInput: json.RawMessage(`{"command":"ls"}`)},
			{Kind: llm.EventDone, StopReason: "tool_use"},
		},
		{{Kind: llm.EventText, Text: "done"}, {Kind: llm.EventDone, StopReason: "end_turn"}},
		// Exchange 2: just text
		{{Kind: llm.EventText, Text: "second"}, {Kind: llm.EventDone, StopReason: "end_turn"}},
	}}
	reg := tools.NewRegistry(&fakeTool{})
	s := NewSession(Config{Provider: p, Model: "m", Tools: reg})
	if err := s.Step(context.Background(), "first"); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if err := s.Step(context.Background(), "second"); err != nil {
		t.Fatalf("step 2: %v", err)
	}

	// Pre-truncate history:
	//   user(first), assistant(tool_use), user(tool_result), assistant(done),
	//   user(second), assistant(second-reply)
	pre := s.Messages()
	if len(pre) != 6 {
		t.Fatalf("pre-truncate len = %d, want 6", len(pre))
	}
	s.Truncate(1)
	post := s.Messages()
	// Should keep just exchange 2: user(second) + assistant.
	if len(post) != 2 {
		t.Errorf("post-truncate len = %d, want 2 (just last exchange)", len(post))
	}
	if post[0].Content[0].Text != "second" {
		t.Errorf("kept wrong start: %+v", post[0])
	}
}

// fakeTool keeps engine_test's recordedTool alternative simple here.
type fakeTool struct{}

func (fakeTool) Name() string                    { return "shell" }
func (fakeTool) Description() string             { return "" }
func (fakeTool) InputSchema() json.RawMessage    { return json.RawMessage(`{"type":"object"}`) }
func (fakeTool) Run(_ context.Context, _ json.RawMessage) (string, error) {
	return "ok", nil
}

func TestRunWrapsSession(t *testing.T) {
	// Run must continue to work — it's the public single-shot surface.
	p := &recordingProvider{turns: [][]llm.Event{
		{{Kind: llm.EventText, Text: "hello"}, {Kind: llm.EventDone, StopReason: "end_turn"}},
	}}
	var out bytes.Buffer
	if err := Run(context.Background(), Config{Provider: p, Model: "m", Out: &out}, "hi"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), "hello") {
		t.Errorf("out = %q", out.String())
	}
}
