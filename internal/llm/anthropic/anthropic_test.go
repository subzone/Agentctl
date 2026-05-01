package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/subzone/m/internal/llm"
)

func TestParseSSEHappyPath(t *testing.T) {
	stream := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_1","model":"x"}}`,
		"",
		": ping comment",
		"event: ping",
		`data: {"type":"ping"}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello "}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}`,
		"",
		"event: content_block_stop",
		`data: {"type":"content_block_stop","index":0}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	out := make(chan llm.Event, 16)
	if err := parseSSE(context.Background(), strings.NewReader(stream), out); err != nil {
		t.Fatalf("parseSSE: %v", err)
	}
	close(out)

	var (
		got    strings.Builder
		done   bool
		reason string
	)
	for ev := range out {
		switch ev.Kind {
		case llm.EventText:
			got.WriteString(ev.Text)
		case llm.EventDone:
			done = true
			reason = ev.StopReason
		case llm.EventError:
			t.Fatalf("unexpected error event: %v", ev.Err)
		case llm.EventToolCall:
			t.Errorf("unexpected tool call: %+v", ev)
		}
	}
	if got.String() != "Hello world" {
		t.Errorf("text = %q, want %q", got.String(), "Hello world")
	}
	if !done {
		t.Error("missing EventDone")
	}
	if reason != "end_turn" {
		t.Errorf("stop reason = %q, want end_turn", reason)
	}
}

func TestParseSSEToolUse(t *testing.T) {
	stream := strings.Join([]string{
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Let me check."}}`,
		"",
		"event: content_block_stop",
		`data: {"type":"content_block_stop","index":0}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_abc","name":"shell","input":{}}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"comm"}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"and\":\"ls\"}"}}`,
		"",
		"event: content_block_stop",
		`data: {"type":"content_block_stop","index":1}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	out := make(chan llm.Event, 16)
	if err := parseSSE(context.Background(), strings.NewReader(stream), out); err != nil {
		t.Fatalf("parseSSE: %v", err)
	}
	close(out)

	var (
		text       strings.Builder
		toolCalls  []llm.Event
		stopReason string
	)
	for ev := range out {
		switch ev.Kind {
		case llm.EventText:
			text.WriteString(ev.Text)
		case llm.EventToolCall:
			toolCalls = append(toolCalls, ev)
		case llm.EventDone:
			stopReason = ev.StopReason
		case llm.EventError:
			t.Fatalf("unexpected error: %v", ev.Err)
		}
	}
	if text.String() != "Let me check." {
		t.Errorf("text = %q", text.String())
	}
	if len(toolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(toolCalls))
	}
	tc := toolCalls[0]
	if tc.ToolID != "toolu_abc" || tc.ToolName != "shell" {
		t.Errorf("tool call meta: %+v", tc)
	}
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(tc.ToolInput, &args); err != nil {
		t.Fatalf("tool input not valid JSON: %v (raw=%s)", err, tc.ToolInput)
	}
	if args.Command != "ls" {
		t.Errorf("command = %q, want ls", args.Command)
	}
	if stopReason != "tool_use" {
		t.Errorf("stop reason = %q", stopReason)
	}
}

func TestParseSSEError(t *testing.T) {
	stream := "event: error\n" +
		`data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}` +
		"\n\n"

	out := make(chan llm.Event, 4)
	err := parseSSE(context.Background(), strings.NewReader(stream), out)
	close(out)
	if err == nil {
		t.Fatal("expected error from parseSSE")
	}
	if !strings.Contains(err.Error(), "Overloaded") {
		t.Errorf("error %v missing 'Overloaded'", err)
	}
}

func TestStreamEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("missing/incorrect x-api-key: %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != apiVersion {
			t.Errorf("anthropic-version = %q", r.Header.Get("anthropic-version"))
		}

		var got messagePayload
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if !got.Stream {
			t.Error("payload.Stream = false")
		}
		if got.Model != "claude-test" {
			t.Errorf("payload.Model = %q", got.Model)
		}
		if got.MaxTokens != 256 {
			t.Errorf("payload.MaxTokens = %d", got.MaxTokens)
		}
		if got.System != "be brief" {
			t.Errorf("payload.System = %q", got.System)
		}
		if len(got.Messages) != 1 {
			t.Fatalf("messages = %+v", got.Messages)
		}
		// Content blocks come through as []any of map[string]any.
		blocks, _ := got.Messages[0].Content, got.Messages[0].Role
		if len(blocks) != 1 {
			t.Fatalf("blocks = %+v", blocks)
		}
		blk, _ := blocks[0].(map[string]any)
		if blk["type"] != "text" || blk["text"] != "hi" {
			t.Errorf("first block = %+v", blk)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		writeEvent := func(ev, data string) {
			io.WriteString(w, "event: "+ev+"\ndata: "+data+"\n\n")
			if fl != nil {
				fl.Flush()
			}
		}
		writeEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		writeEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hey"}}`)
		writeEvent("content_block_stop", `{"type":"content_block_stop","index":0}`)
		writeEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`)
		writeEvent("message_stop", `{"type":"message_stop"}`)
	}))
	defer srv.Close()

	p, err := New(WithAPIKey("test-key"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	events, err := p.Stream(context.Background(), llm.Request{
		Model:     "claude-test",
		System:    "be brief",
		Messages:  []llm.Message{llm.TextMessage(llm.RoleUser, "hi")},
		MaxTokens: 256,
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
			t.Fatalf("error event: %v", ev.Err)
		}
	}
	if text.String() != "hey" {
		t.Errorf("text = %q, want hey", text.String())
	}
	if !done {
		t.Error("missing done event")
	}
}

func TestStreamSendsToolsAndContentBlocks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got messagePayload
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(got.Tools) != 1 || got.Tools[0].Name != "shell" {
			t.Errorf("tools = %+v", got.Tools)
		}
		if len(got.Messages) != 3 {
			t.Fatalf("expected 3 messages, got %d", len(got.Messages))
		}
		// 1: user text "list", 2: assistant tool_use, 3: user tool_result
		assistantBlocks := got.Messages[1].Content
		if len(assistantBlocks) != 1 {
			t.Fatalf("assistant blocks = %+v", assistantBlocks)
		}
		ab := assistantBlocks[0].(map[string]any)
		if ab["type"] != "tool_use" || ab["name"] != "shell" || ab["id"] != "toolu_1" {
			t.Errorf("assistant tool_use = %+v", ab)
		}
		input, _ := ab["input"].(map[string]any)
		if input["command"] != "ls" {
			t.Errorf("input = %+v", ab["input"])
		}
		resBlocks := got.Messages[2].Content
		rb := resBlocks[0].(map[string]any)
		if rb["type"] != "tool_result" || rb["tool_use_id"] != "toolu_1" || rb["content"] != "file.txt" {
			t.Errorf("tool_result block = %+v", rb)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	}))
	defer srv.Close()

	p, err := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = p.Stream(context.Background(), llm.Request{
		Model: "x",
		Tools: []llm.ToolSchema{{
			Name:        "shell",
			Description: "run a command",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
		Messages: []llm.Message{
			llm.TextMessage(llm.RoleUser, "list"),
			{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{
				Type:      llm.BlockToolUse,
				ToolID:    "toolu_1",
				ToolName:  "shell",
				ToolInput: json.RawMessage(`{"command":"ls"}`),
			}}},
			{Role: llm.RoleUser, Content: []llm.ContentBlock{{
				Type:      llm.BlockToolResult,
				ToolUseID: "toolu_1",
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
		http.Error(w, `{"type":"error","error":{"type":"invalid_request_error","message":"nope"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	p, err := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = p.Stream(context.Background(), llm.Request{
		Model:    "x",
		Messages: []llm.Message{llm.TextMessage(llm.RoleUser, "hi")},
	})
	if err == nil {
		t.Fatal("expected error from Stream on 400")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %v missing body text", err)
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	if _, err := New(); err == nil {
		t.Fatal("expected error when ANTHROPIC_API_KEY missing")
	}
}
