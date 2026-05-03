package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/subzone/Agentctl/internal/llm"
)

func TestParseStreamHappyText(t *testing.T) {
	stream := strings.Join([]string{
		`{"message":{"role":"assistant","content":"Hello "},"done":false}`,
		`{"message":{"role":"assistant","content":"world"},"done":false}`,
		`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
		"",
	}, "\n")

	out := make(chan llm.Event, 16)
	if err := parseStream(context.Background(), strings.NewReader(stream), out); err != nil {
		t.Fatalf("parseStream: %v", err)
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
			t.Fatalf("err: %v", ev.Err)
		}
	}
	if text.String() != "Hello world" {
		t.Errorf("text = %q", text.String())
	}
	if !done || reason != "end_turn" {
		t.Errorf("done=%v reason=%q", done, reason)
	}
}

func TestParseStreamToolCalls(t *testing.T) {
	stream := strings.Join([]string{
		`{"message":{"role":"assistant","content":"checking..."},"done":false}`,
		`{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"shell","arguments":{"command":"ls"}}}]},"done":false}`,
		`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`,
		"",
	}, "\n")

	out := make(chan llm.Event, 16)
	if err := parseStream(context.Background(), strings.NewReader(stream), out); err != nil {
		t.Fatalf("parseStream: %v", err)
	}
	close(out)

	var (
		text  strings.Builder
		calls []llm.Event
		stop  string
	)
	for ev := range out {
		switch ev.Kind {
		case llm.EventText:
			text.WriteString(ev.Text)
		case llm.EventToolCall:
			calls = append(calls, ev)
		case llm.EventDone:
			stop = ev.StopReason
		case llm.EventError:
			t.Fatalf("err: %v", ev.Err)
		}
	}
	if text.String() != "checking..." {
		t.Errorf("text = %q", text.String())
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].ToolName != "shell" {
		t.Errorf("tool name = %q", calls[0].ToolName)
	}
	if calls[0].ToolID == "" {
		t.Error("tool id should be minted (synthetic), not empty")
	}
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(calls[0].ToolInput, &args); err != nil || args.Command != "ls" {
		t.Errorf("input = %s (err=%v)", calls[0].ToolInput, err)
	}
	// Tool calls override done_reason → "tool_use" so engine loops correctly.
	if stop != "tool_use" {
		t.Errorf("stop = %q, want tool_use (model used a tool)", stop)
	}
}

func TestParseStreamErrorField(t *testing.T) {
	stream := `{"error":"model not found"}` + "\n"
	out := make(chan llm.Event, 4)
	err := parseStream(context.Background(), strings.NewReader(stream), out)
	close(out)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("got %v", err)
	}
}

func TestStreamEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var got chatPayload
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Model != "llama-test" || !got.Stream {
			t.Errorf("payload = %+v", got)
		}
		if len(got.Messages) != 2 {
			t.Fatalf("messages = %d", len(got.Messages))
		}
		if got.Messages[0].Role != "system" || got.Messages[0].Content != "be brief" {
			t.Errorf("system msg = %+v", got.Messages[0])
		}
		if got.Messages[1].Role != "user" || got.Messages[1].Content != "hi" {
			t.Errorf("user msg = %+v", got.Messages[1])
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		fl, _ := w.(http.Flusher)
		writeLine := func(s string) {
			io.WriteString(w, s+"\n")
			if fl != nil {
				fl.Flush()
			}
		}
		writeLine(`{"message":{"role":"assistant","content":"hey"},"done":false}`)
		writeLine(`{"message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`)
	}))
	defer srv.Close()

	p, err := New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	events, err := p.Stream(context.Background(), llm.Request{
		Model:    "llama-test",
		System:   "be brief",
		Messages: []llm.Message{llm.TextMessage(llm.RoleUser, "hi")},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var text strings.Builder
	for ev := range events {
		if ev.Kind == llm.EventText {
			text.WriteString(ev.Text)
		}
		if ev.Kind == llm.EventError {
			t.Fatalf("err: %v", ev.Err)
		}
	}
	if text.String() != "hey" {
		t.Errorf("text = %q", text.String())
	}
}

func TestStreamRoundTripsToolUse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got chatPayload
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		// Expected: system, user, assistant(tool_calls), tool(role=tool)
		if len(got.Messages) != 4 {
			t.Fatalf("messages = %d, want 4", len(got.Messages))
		}
		assistant := got.Messages[2]
		if assistant.Role != "assistant" || len(assistant.ToolCalls) != 1 {
			t.Errorf("assistant turn = %+v", assistant)
		}
		tc := assistant.ToolCalls[0]
		if tc.Function.Name != "shell" {
			t.Errorf("tool call name = %q", tc.Function.Name)
		}
		// Ollama receives arguments as JSON object, not encoded string.
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(tc.Function.Arguments, &args); err != nil || args.Command != "ls" {
			t.Errorf("tool args raw=%s err=%v", tc.Function.Arguments, err)
		}
		toolMsg := got.Messages[3]
		if toolMsg.Role != "tool" || toolMsg.Content != "file.txt" {
			t.Errorf("tool msg = %+v", toolMsg)
		}

		io.WriteString(w, `{"message":{"role":"assistant","content":"done"},"done":true,"done_reason":"stop"}`+"\n")
	}))
	defer srv.Close()

	p, _ := New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	_, err := p.Stream(context.Background(), llm.Request{
		Model:  "llama-test",
		System: "sys",
		Tools:  []llm.ToolSchema{{Name: "shell"}},
		Messages: []llm.Message{
			llm.TextMessage(llm.RoleUser, "go"),
			{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{
				Type:      llm.BlockToolUse,
				ToolID:    "ollama-tool-1",
				ToolName:  "shell",
				ToolInput: json.RawMessage(`{"command":"ls"}`),
			}}},
			{Role: llm.RoleUser, Content: []llm.ContentBlock{{
				Type:      llm.BlockToolResult,
				ToolUseID: "ollama-tool-1",
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
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer srv.Close()
	p, _ := New(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	_, err := p.Stream(context.Background(), llm.Request{
		Model:    "x",
		Messages: []llm.Message{llm.TextMessage(llm.RoleUser, "hi")},
	})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("got %v", err)
	}
}

func TestNewDefaultsAndHostNormalization(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "")
	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", p.baseURL, defaultBaseURL)
	}

	t.Setenv("OLLAMA_HOST", "host.local:11434")
	p, _ = New()
	if p.baseURL != "http://host.local:11434" {
		t.Errorf("baseURL not normalized: %q", p.baseURL)
	}

	t.Setenv("OLLAMA_HOST", "https://ollama.example.com")
	p, _ = New()
	if p.baseURL != "https://ollama.example.com" {
		t.Errorf("baseURL = %q", p.baseURL)
	}
}

func TestNormalizeStop(t *testing.T) {
	cases := map[string]string{
		"stop":   "end_turn",
		"":       "end_turn",
		"length": "max_tokens",
		"weird":  "weird",
	}
	for in, want := range cases {
		if got := normalizeStop(in); got != want {
			t.Errorf("normalizeStop(%q) = %q, want %q", in, got, want)
		}
	}
}
