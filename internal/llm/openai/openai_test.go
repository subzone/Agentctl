package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/milenkom81/m/internal/llm"
)

func TestParseSSEHappyText(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"choices":[{"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
		"",
		`data: {"choices":[{"delta":{"content":"Hello "},"finish_reason":null}]}`,
		"",
		`data: {"choices":[{"delta":{"content":"world"},"finish_reason":null}]}`,
		"",
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	out := make(chan llm.Event, 16)
	if err := parseSSE(context.Background(), strings.NewReader(stream), out); err != nil {
		t.Fatalf("parseSSE: %v", err)
	}
	close(out)

	var (
		text   strings.Builder
		done   bool
		reason string
	)
	for ev := range out {
		switch ev.Kind {
		case llm.EventText:
			text.WriteString(ev.Text)
		case llm.EventDone:
			done = true
			reason = ev.StopReason
		case llm.EventError:
			t.Fatalf("error: %v", ev.Err)
		case llm.EventToolCall:
			t.Errorf("unexpected tool call")
		}
	}
	if text.String() != "Hello world" {
		t.Errorf("text = %q", text.String())
	}
	if !done {
		t.Error("missing done")
	}
	if reason != "end_turn" {
		t.Errorf("stop = %q, want end_turn (normalized from 'stop')", reason)
	}
}

func TestParseSSEToolCalls(t *testing.T) {
	// Two tool calls, arguments split across multiple chunks.
	stream := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"Looking..."},"finish_reason":null}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"shell","arguments":""}}]},"finish_reason":null}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"comm"}}]},"finish_reason":null}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"and\":\"ls\"}"}}]},"finish_reason":null}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"fs.read","arguments":"{\"path\":\"x\"}"}}]},"finish_reason":null}]}`,
		"",
		`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
		"",
	}, "\n")

	out := make(chan llm.Event, 16)
	if err := parseSSE(context.Background(), strings.NewReader(stream), out); err != nil {
		t.Fatalf("parseSSE: %v", err)
	}
	close(out)

	var (
		text   strings.Builder
		calls  []llm.Event
		reason string
	)
	for ev := range out {
		switch ev.Kind {
		case llm.EventText:
			text.WriteString(ev.Text)
		case llm.EventToolCall:
			calls = append(calls, ev)
		case llm.EventDone:
			reason = ev.StopReason
		case llm.EventError:
			t.Fatalf("error: %v", ev.Err)
		}
	}
	if text.String() != "Looking..." {
		t.Errorf("text = %q", text.String())
	}
	if reason != "tool_use" {
		t.Errorf("stop = %q, want tool_use (normalized from 'tool_calls')", reason)
	}
	if len(calls) != 2 {
		t.Fatalf("got %d tool calls, want 2", len(calls))
	}
	if calls[0].ToolID != "call_a" || calls[0].ToolName != "shell" {
		t.Errorf("call 0 meta: %+v", calls[0])
	}
	var args0 struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(calls[0].ToolInput, &args0); err != nil || args0.Command != "ls" {
		t.Errorf("call 0 input: %s (err=%v)", calls[0].ToolInput, err)
	}
	if calls[1].ToolID != "call_b" || calls[1].ToolName != "fs.read" {
		t.Errorf("call 1 meta: %+v", calls[1])
	}
}

func TestParseSSEErrorChunk(t *testing.T) {
	stream := `data: {"error":{"type":"server_error","message":"something broke"}}` + "\n\n"
	out := make(chan llm.Event, 4)
	err := parseSSE(context.Background(), strings.NewReader(stream), out)
	close(out)
	if err == nil || !strings.Contains(err.Error(), "something broke") {
		t.Errorf("got %v", err)
	}
}

func TestStreamEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		var got chatPayload
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !got.Stream || got.Model != "gpt-test" {
			t.Errorf("payload meta: %+v", got)
		}
		// system + user
		if len(got.Messages) != 2 {
			t.Fatalf("messages = %d", len(got.Messages))
		}
		if got.Messages[0].Role != "system" || got.Messages[0].Content != "be brief" {
			t.Errorf("system msg = %+v", got.Messages[0])
		}
		if got.Messages[1].Role != "user" || got.Messages[1].Content != "hi" {
			t.Errorf("user msg = %+v", got.Messages[1])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		writeChunk := func(s string) {
			io.WriteString(w, "data: "+s+"\n\n")
			if fl != nil {
				fl.Flush()
			}
		}
		writeChunk(`{"choices":[{"delta":{"content":"hey"},"finish_reason":null}]}`)
		writeChunk(`{"choices":[{"delta":{},"finish_reason":"stop"}]}`)
		writeChunk(`[DONE]`)
	}))
	defer srv.Close()

	p, err := New(WithAPIKey("test-key"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	events, err := p.Stream(context.Background(), llm.Request{
		Model:    "gpt-test",
		System:   "be brief",
		Messages: []llm.Message{llm.TextMessage(llm.RoleUser, "hi")},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var text strings.Builder
	var done bool
	for ev := range events {
		switch ev.Kind {
		case llm.EventText:
			text.WriteString(ev.Text)
		case llm.EventDone:
			done = true
		case llm.EventError:
			t.Fatalf("error: %v", ev.Err)
		}
	}
	if text.String() != "hey" || !done {
		t.Errorf("got %q done=%v", text.String(), done)
	}
}

func TestStreamSendsToolsAndRoundTripsToolUse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got chatPayload
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got.Tools) != 1 || got.Tools[0].Function.Name != "shell" {
			t.Errorf("tools = %+v", got.Tools)
		}
		// Expect: system, user(text), assistant(tool_calls), tool(role=tool with id+content)
		if len(got.Messages) != 4 {
			t.Fatalf("messages = %d, want 4", len(got.Messages))
		}
		if got.Messages[2].Role != "assistant" || len(got.Messages[2].ToolCalls) != 1 {
			t.Errorf("assistant turn = %+v", got.Messages[2])
		}
		tc := got.Messages[2].ToolCalls[0]
		if tc.ID != "call_a" || tc.Function.Name != "shell" || tc.Function.Arguments != `{"command":"ls"}` {
			t.Errorf("tool call = %+v", tc)
		}
		if got.Messages[3].Role != "tool" || got.Messages[3].ToolCallID != "call_a" || got.Messages[3].Content != "file.txt" {
			t.Errorf("tool result = %+v", got.Messages[3])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	p, _ := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	_, err := p.Stream(context.Background(), llm.Request{
		Model:  "gpt-test",
		System: "sys",
		Tools: []llm.ToolSchema{{
			Name:        "shell",
			Description: "run a command",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
		Messages: []llm.Message{
			llm.TextMessage(llm.RoleUser, "list"),
			{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{
				Type:      llm.BlockToolUse,
				ToolID:    "call_a",
				ToolName:  "shell",
				ToolInput: json.RawMessage(`{"command":"ls"}`),
			}}},
			{Role: llm.RoleUser, Content: []llm.ContentBlock{{
				Type:      llm.BlockToolResult,
				ToolUseID: "call_a",
				Output:    "file.txt",
			}}},
		},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
}

func TestStreamHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"bad key"}}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	p, _ := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	_, err := p.Stream(context.Background(), llm.Request{
		Model:    "x",
		Messages: []llm.Message{llm.TextMessage(llm.RoleUser, "hi")},
	})
	if err == nil || !strings.Contains(err.Error(), "bad key") {
		t.Errorf("got %v", err)
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	if _, err := New(); err == nil {
		t.Fatal("expected error when OPENAI_API_KEY missing")
	}
}

func TestNormalizeStop(t *testing.T) {
	cases := map[string]string{
		"stop":          "end_turn",
		"tool_calls":    "tool_use",
		"function_call": "tool_use",
		"length":        "max_tokens",
		"":              "",
		"weird":         "weird",
	}
	for in, want := range cases {
		if got := normalizeStop(in); got != want {
			t.Errorf("normalizeStop(%q) = %q, want %q", in, got, want)
		}
	}
}
