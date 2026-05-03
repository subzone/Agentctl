package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/subzone/Agentctl/internal/engine"
	"github.com/subzone/Agentctl/internal/llm"
)

// chatScriptedProvider replays one or more turns. Unlike the engine_test
// version, it loops back to the first turn after exhaustion so a long
// chat session in tests doesn't have to predeclare every reply.
type chatScriptedProvider struct {
	turns [][]llm.Event
	calls int
}

func (c *chatScriptedProvider) Stream(_ context.Context, _ llm.Request) (<-chan llm.Event, error) {
	t := c.turns[c.calls%len(c.turns)]
	c.calls++
	ch := make(chan llm.Event, len(t))
	for _, ev := range t {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func newChatTestSession(p llm.Provider, out, status io.Writer) *engine.Session {
	return engine.NewSession(engine.Config{
		Provider: p, Model: "m",
		Out: out, Status: status,
	})
}

func TestChatLoopGreetsAndExits(t *testing.T) {
	p := &chatScriptedProvider{turns: [][]llm.Event{
		{{Kind: llm.EventText, Text: "hi back"}, {Kind: llm.EventDone, StopReason: "end_turn"}},
	}}
	var out, status bytes.Buffer
	in := strings.NewReader("hello\n/exit\n")
	sess := newChatTestSession(p, &out, &status)

	if err := chatLoop(context.Background(), sess, nil, in, &out, &status, "tester"); err != nil {
		t.Fatalf("chatLoop: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "hi back") {
		t.Errorf("output missing assistant reply: %q", got)
	}
	if !strings.Contains(got, chatPrompt) {
		t.Errorf("output missing prompt char: %q", got)
	}
	if !strings.Contains(status.String(), "chat with tester") {
		t.Errorf("status missing greeting: %q", status.String())
	}
	if p.calls != 1 {
		t.Errorf("provider calls = %d, want 1 (only the real turn, not the slash command)", p.calls)
	}
}

func TestChatLoopMultiTurn(t *testing.T) {
	p := &chatScriptedProvider{turns: [][]llm.Event{
		{{Kind: llm.EventText, Text: "one"}, {Kind: llm.EventDone, StopReason: "end_turn"}},
		{{Kind: llm.EventText, Text: "two"}, {Kind: llm.EventDone, StopReason: "end_turn"}},
	}}
	var out, status bytes.Buffer
	in := strings.NewReader("first\nsecond\n/exit\n")
	sess := newChatTestSession(p, &out, &status)
	if err := chatLoop(context.Background(), sess, nil, in, &out, &status, "agent"); err != nil {
		t.Fatalf("chatLoop: %v", err)
	}
	if p.calls != 2 {
		t.Errorf("provider calls = %d, want 2", p.calls)
	}
	if !strings.Contains(out.String(), "one") || !strings.Contains(out.String(), "two") {
		t.Errorf("missing replies: %q", out.String())
	}
	// Session should have all 4 messages (u/a/u/a).
	if got := len(sess.Messages()); got != 4 {
		t.Errorf("history len = %d, want 4", got)
	}
}

func TestChatLoopReset(t *testing.T) {
	p := &chatScriptedProvider{turns: [][]llm.Event{
		{{Kind: llm.EventText, Text: "ok"}, {Kind: llm.EventDone, StopReason: "end_turn"}},
	}}
	var out, status bytes.Buffer
	in := strings.NewReader("hello\n/reset\nagain\n/exit\n")
	sess := newChatTestSession(p, &out, &status)
	if err := chatLoop(context.Background(), sess, nil, in, &out, &status, "a"); err != nil {
		t.Fatalf("chatLoop: %v", err)
	}
	if !strings.Contains(status.String(), "history cleared") {
		t.Errorf("missing reset notice: %q", status.String())
	}
	// After /reset and then "again", history should hold just the last
	// exchange — 2 messages, not 4.
	if got := len(sess.Messages()); got != 2 {
		t.Errorf("post-reset history = %d, want 2", got)
	}
}

func TestChatLoopHelpAndUnknownSlash(t *testing.T) {
	p := &chatScriptedProvider{turns: [][]llm.Event{
		{{Kind: llm.EventDone, StopReason: "end_turn"}},
	}}
	var out, status bytes.Buffer
	in := strings.NewReader("/help\n/whatever\n/exit\n")
	sess := newChatTestSession(p, &out, &status)
	if err := chatLoop(context.Background(), sess, nil, in, &out, &status, "a"); err != nil {
		t.Fatalf("chatLoop: %v", err)
	}
	if !strings.Contains(status.String(), "commands:") {
		t.Errorf("help missing: %q", status.String())
	}
	if !strings.Contains(status.String(), `unknown command "/whatever"`) {
		t.Errorf("unknown-command notice missing: %q", status.String())
	}
	if p.calls != 0 {
		t.Errorf("slash commands should not invoke provider; calls = %d", p.calls)
	}
}

func TestChatLoopSkipsBlankLines(t *testing.T) {
	p := &chatScriptedProvider{turns: [][]llm.Event{
		{{Kind: llm.EventText, Text: "x"}, {Kind: llm.EventDone, StopReason: "end_turn"}},
	}}
	var out, status bytes.Buffer
	in := strings.NewReader("\n   \nactual\n/exit\n")
	sess := newChatTestSession(p, &out, &status)
	if err := chatLoop(context.Background(), sess, nil, in, &out, &status, "a"); err != nil {
		t.Fatalf("chatLoop: %v", err)
	}
	if p.calls != 1 {
		t.Errorf("provider calls = %d, want 1 (blank lines skipped)", p.calls)
	}
}

func TestChatLoopExitsOnEOF(t *testing.T) {
	p := &chatScriptedProvider{turns: [][]llm.Event{
		{{Kind: llm.EventText, Text: "ok"}, {Kind: llm.EventDone, StopReason: "end_turn"}},
	}}
	var out, status bytes.Buffer
	in := strings.NewReader("hi\n") // no /exit; EOF after this turn
	sess := newChatTestSession(p, &out, &status)
	done := make(chan error, 1)
	go func() {
		done <- chatLoop(context.Background(), sess, nil, in, &out, &status, "a")
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("loop returned err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("chatLoop did not return on EOF")
	}
}

func TestChatLoopExitsOnContextCancel(t *testing.T) {
	// Stream that hangs forever inside Step.
	p := &hangingProvider{}
	var out, status bytes.Buffer
	in := strings.NewReader("trigger\n") // one user message, then EOF
	sess := newChatTestSession(p, &out, &status)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- chatLoop(ctx, sess, nil, in, &out, &status, "a")
	}()
	// Give the loop a moment to dispatch into Step.
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("loop returned err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("chatLoop did not return on ctx cancel")
	}
}

// hangingProvider returns a channel that blocks until ctx is cancelled.
type hangingProvider struct{}

func (hangingProvider) Stream(ctx context.Context, _ llm.Request) (<-chan llm.Event, error) {
	ch := make(chan llm.Event)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

func TestChatLoopHistoryTruncatesAcrossManyTurns(t *testing.T) {
	// Simulate more turns than chatMaxExchanges to verify truncation
	// engages and keeps only the most recent exchanges.
	p := &chatScriptedProvider{turns: [][]llm.Event{
		{{Kind: llm.EventText, Text: "."}, {Kind: llm.EventDone, StopReason: "end_turn"}},
	}}
	var out, status bytes.Buffer
	var lines strings.Builder
	for i := 0; i < chatMaxExchanges+3; i++ {
		fmt := byte('a' + byte(i))
		lines.WriteByte(fmt)
		lines.WriteByte('\n')
	}
	lines.WriteString("/exit\n")
	sess := newChatTestSession(p, &out, &status)
	if err := chatLoop(context.Background(), sess, nil, strings.NewReader(lines.String()), &out, &status, "a"); err != nil {
		t.Fatalf("chatLoop: %v", err)
	}
	// History should contain at most chatMaxExchanges*2 messages (u/a per exchange).
	if got := len(sess.Messages()); got > chatMaxExchanges*2 {
		t.Errorf("history len = %d, want <= %d", got, chatMaxExchanges*2)
	}
}

func TestHandleSlashUnknown(t *testing.T) {
	sess := newChatTestSession(&chatScriptedProvider{}, io.Discard, io.Discard)
	var status bytes.Buffer
	handled, exit := handleSlash("/dunno", sess, nil, &status)
	if !handled || exit {
		t.Errorf("expected handled=true exit=false, got %v %v", handled, exit)
	}
	if !strings.Contains(status.String(), "unknown command") {
		t.Errorf("status = %q", status.String())
	}
}

func TestHandleSlashNonCommandPassesThrough(t *testing.T) {
	sess := newChatTestSession(&chatScriptedProvider{}, io.Discard, io.Discard)
	handled, _ := handleSlash("hello world", sess, nil, io.Discard)
	if handled {
		t.Error("regular input should not be reported as handled")
	}
}

func TestHandleSlashCompact(t *testing.T) {
	p := &chatScriptedProvider{turns: [][]llm.Event{
		{{Kind: llm.EventDone}},
		{{Kind: llm.EventDone}},
		{{Kind: llm.EventDone}},
		{{Kind: llm.EventDone}},
		{{Kind: llm.EventDone}},
		{{Kind: llm.EventDone}},
	}}
	var out, status bytes.Buffer
	in := strings.NewReader("a\nb\nc\nd\ne\nf\n/compact\n/exit\n")
	sess := newChatTestSession(p, &out, &status)
	if err := chatLoop(context.Background(), sess, nil, in, &out, &status, "a"); err != nil {
		t.Fatalf("chatLoop: %v", err)
	}
	if !strings.Contains(status.String(), "compacted") {
		t.Errorf("missing compact notice: %q", status.String())
	}
	// After 6 exchanges + compact(4), should have at most 8 messages.
	if got := len(sess.Messages()); got > 8 {
		t.Errorf("post-compact messages = %d, want <= 8", got)
	}
}

func TestHandleSlashModelSwitch(t *testing.T) {
	// Register a test provider so /model can resolve it.
	llm.Register("testprov", func() (llm.Provider, error) {
		return &chatScriptedProvider{turns: [][]llm.Event{
			{{Kind: llm.EventDone}},
		}}, nil
	})
	sess := newChatTestSession(&chatScriptedProvider{}, io.Discard, io.Discard)
	var status bytes.Buffer
	handled, exit := handleSlash("/model testprov/new-model", sess, nil, &status)
	if !handled || exit {
		t.Errorf("handled=%v exit=%v", handled, exit)
	}
	if !strings.Contains(status.String(), "switched to testprov/new-model") {
		t.Errorf("status = %q", status.String())
	}
	if sess.Model() != "new-model" {
		t.Errorf("Model() = %q", sess.Model())
	}
}

func TestHandleSlashModelInvalid(t *testing.T) {
	sess := newChatTestSession(&chatScriptedProvider{}, io.Discard, io.Discard)
	var status bytes.Buffer
	handled, _ := handleSlash("/model badformat", sess, nil, &status)
	if !handled {
		t.Error("should be handled")
	}
	if !strings.Contains(status.String(), "error") {
		t.Errorf("status = %q, want error", status.String())
	}
}
