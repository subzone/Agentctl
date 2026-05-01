package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/subzone/m/internal/llm"
	"github.com/subzone/m/internal/tools"
)

// scriptedProvider replays a pre-built sequence of event-streams, one per
// turn, and records the request that produced each.
type scriptedProvider struct {
	turns    [][]llm.Event
	requests []llm.Request
}

func (s *scriptedProvider) Stream(ctx context.Context, req llm.Request) (<-chan llm.Event, error) {
	if len(s.turns) == 0 {
		return nil, errors.New("scriptedProvider: ran out of turns")
	}
	turn := s.turns[0]
	s.turns = s.turns[1:]
	s.requests = append(s.requests, req)

	ch := make(chan llm.Event, len(turn))
	for _, ev := range turn {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

// recordedTool captures inputs and returns canned outputs.
type recordedTool struct {
	name   string
	inputs []json.RawMessage
	out    string
	err    error
}

func (r *recordedTool) Name() string                 { return r.name }
func (r *recordedTool) Description() string          { return "test tool" }
func (r *recordedTool) InputSchema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (r *recordedTool) Run(ctx context.Context, in json.RawMessage) (string, error) {
	r.inputs = append(r.inputs, append(json.RawMessage(nil), in...))
	return r.out, r.err
}

func TestRunSingleTurnText(t *testing.T) {
	p := &scriptedProvider{turns: [][]llm.Event{{
		{Kind: llm.EventText, Text: "hello "},
		{Kind: llm.EventText, Text: "there"},
		{Kind: llm.EventDone, StopReason: "end_turn"},
	}}}
	var out bytes.Buffer
	err := Run(context.Background(), Config{
		Provider: p, Model: "x", Out: &out,
	}, "hi")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := out.String(); got != "hello there\n" {
		t.Errorf("out = %q, want %q", got, "hello there\n")
	}
	if len(p.requests) != 1 {
		t.Fatalf("turns sent = %d, want 1", len(p.requests))
	}
	first := p.requests[0]
	if len(first.Messages) != 1 || first.Messages[0].Role != llm.RoleUser {
		t.Errorf("initial messages = %+v", first.Messages)
	}
}

func TestRunToolLoop(t *testing.T) {
	shell := &recordedTool{name: "shell", out: "file_a\nfile_b\n"}
	reg := tools.NewRegistry(shell)

	p := &scriptedProvider{turns: [][]llm.Event{
		// turn 1: assistant says it'll look, then calls shell
		{
			{Kind: llm.EventText, Text: "Listing..."},
			{Kind: llm.EventToolCall, ToolID: "tu_1", ToolName: "shell", ToolInput: json.RawMessage(`{"command":"ls"}`)},
			{Kind: llm.EventDone, StopReason: "tool_use"},
		},
		// turn 2: assistant summarizes
		{
			{Kind: llm.EventText, Text: "two files."},
			{Kind: llm.EventDone, StopReason: "end_turn"},
		},
	}}

	var out, status bytes.Buffer
	err := Run(context.Background(), Config{
		Provider: p, Model: "x", Tools: reg, Out: &out, Status: &status,
	}, "what's there?")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := out.String(); got != "Listing...two files.\n" {
		t.Errorf("out = %q", got)
	}
	if !strings.Contains(status.String(), "→ shell") {
		t.Errorf("status missing shell indicator: %q", status.String())
	}

	if len(shell.inputs) != 1 {
		t.Fatalf("shell calls = %d, want 1", len(shell.inputs))
	}
	if string(shell.inputs[0]) != `{"command":"ls"}` {
		t.Errorf("shell input = %s", shell.inputs[0])
	}

	if len(p.requests) != 2 {
		t.Fatalf("provider calls = %d, want 2", len(p.requests))
	}
	// Second turn should include: original user, assistant w/ tool_use, user w/ tool_result.
	second := p.requests[1].Messages
	if len(second) != 3 {
		t.Fatalf("turn-2 messages = %d, want 3", len(second))
	}
	if second[1].Role != llm.RoleAssistant {
		t.Errorf("msg 1 role = %s", second[1].Role)
	}
	var sawToolUse bool
	for _, b := range second[1].Content {
		if b.Type == llm.BlockToolUse && b.ToolID == "tu_1" && b.ToolName == "shell" {
			sawToolUse = true
		}
	}
	if !sawToolUse {
		t.Errorf("assistant turn missing tool_use block: %+v", second[1].Content)
	}
	if second[2].Role != llm.RoleUser || len(second[2].Content) != 1 {
		t.Fatalf("msg 2 = %+v", second[2])
	}
	tr := second[2].Content[0]
	if tr.Type != llm.BlockToolResult || tr.ToolUseID != "tu_1" || tr.Output != "file_a\nfile_b\n" {
		t.Errorf("tool_result = %+v", tr)
	}
	if tr.IsError {
		t.Error("expected IsError=false")
	}
}

func TestRunToolErrorPropagatesAsResult(t *testing.T) {
	bad := &recordedTool{name: "shell", err: errors.New("boom")}
	reg := tools.NewRegistry(bad)

	p := &scriptedProvider{turns: [][]llm.Event{
		{
			{Kind: llm.EventToolCall, ToolID: "tu_1", ToolName: "shell", ToolInput: json.RawMessage(`{"command":"x"}`)},
			{Kind: llm.EventDone, StopReason: "tool_use"},
		},
		{
			{Kind: llm.EventText, Text: "ok, recovered"},
			{Kind: llm.EventDone, StopReason: "end_turn"},
		},
	}}
	err := Run(context.Background(), Config{Provider: p, Model: "x", Tools: reg}, "go")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	tr := p.requests[1].Messages[2].Content[0]
	if !tr.IsError {
		t.Error("expected IsError=true")
	}
	if !strings.Contains(tr.Output, "boom") {
		t.Errorf("error not in tool result: %q", tr.Output)
	}
}

func TestRunStreamErrorAborts(t *testing.T) {
	p := &scriptedProvider{turns: [][]llm.Event{{
		{Kind: llm.EventText, Text: "partial "},
		{Kind: llm.EventError, Err: errors.New("network gone")},
	}}}
	var out bytes.Buffer
	err := Run(context.Background(), Config{Provider: p, Model: "x", Out: &out}, "go")
	if err == nil || !strings.Contains(err.Error(), "network gone") {
		t.Errorf("got %v", err)
	}
	if out.String() != "partial \n" {
		t.Errorf("out = %q", out.String())
	}
}

func TestRunMaxTurns(t *testing.T) {
	tool := &recordedTool{name: "shell", out: "more"}
	reg := tools.NewRegistry(tool)
	// Always responds tool_use, never stops naturally.
	loopTurn := []llm.Event{
		{Kind: llm.EventToolCall, ToolID: "tu", ToolName: "shell", ToolInput: json.RawMessage(`{"command":"x"}`)},
		{Kind: llm.EventDone, StopReason: "tool_use"},
	}
	p := &scriptedProvider{turns: [][]llm.Event{loopTurn, loopTurn, loopTurn}}
	err := Run(context.Background(), Config{
		Provider: p, Model: "x", Tools: reg, MaxTurns: 3,
	}, "go")
	if err == nil || !strings.Contains(err.Error(), "MaxTurns") {
		t.Errorf("got %v, want MaxTurns error", err)
	}
}

func TestRunRequiresProvider(t *testing.T) {
	if err := Run(context.Background(), Config{Model: "x"}, "hi"); err == nil {
		t.Fatal("expected error when Provider missing")
	}
}

func TestRunRequiresModel(t *testing.T) {
	p := &scriptedProvider{turns: [][]llm.Event{{{Kind: llm.EventDone}}}}
	if err := Run(context.Background(), Config{Provider: p}, "hi"); err == nil {
		t.Fatal("expected error when Model missing")
	}
}
