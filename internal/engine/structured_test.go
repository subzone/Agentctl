package engine

import (
	"encoding/json"
	"testing"

	"github.com/subzone/m/internal/llm"
)

func TestExtractStructuredResponseFromToolCall(t *testing.T) {
	// Anthropic pattern: structured_response tool call.
	msg := llm.Message{
		Role: llm.RoleAssistant,
		Content: []llm.ContentBlock{
			{Type: llm.BlockText, Text: "Let me analyze this."},
			{Type: llm.BlockToolUse, ToolName: "structured_response", ToolInput: json.RawMessage(`{"answer":"the code is clean","sources":[],"confidence":"high","caveats":[]}`)},
		},
	}
	full, display := extractStructuredResponse(msg)
	if full != `{"answer":"the code is clean","sources":[],"confidence":"high","caveats":[]}` {
		t.Errorf("full = %q", full)
	}
	if display != "the code is clean" {
		t.Errorf("display = %q", display)
	}
}

func TestExtractStructuredResponseFromText(t *testing.T) {
	// OpenAI/Ollama pattern: text block containing JSON.
	msg := llm.Message{
		Role: llm.RoleAssistant,
		Content: []llm.ContentBlock{
			{Type: llm.BlockText, Text: `{"answer":"hello","sources":[],"confidence":"medium","caveats":["none"]}`},
		},
	}
	full, display := extractStructuredResponse(msg)
	if display != "hello" {
		t.Errorf("display = %q", display)
	}
	if !json.Valid([]byte(full)) {
		t.Errorf("full is not valid JSON: %q", full)
	}
}

func TestExtractStructuredResponseInvalidJSON(t *testing.T) {
	msg := llm.Message{
		Role:    llm.RoleAssistant,
		Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "not json at all"}},
	}
	full, display := extractStructuredResponse(msg)
	if full != "not json at all" || display != "not json at all" {
		t.Errorf("full=%q display=%q", full, display)
	}
}

func TestExtractStructuredResponseEmpty(t *testing.T) {
	msg := llm.Message{Role: llm.RoleAssistant}
	full, display := extractStructuredResponse(msg)
	if full != "" || display != "" {
		t.Errorf("full=%q display=%q", full, display)
	}
}

func TestValidateStructuredResponse(t *testing.T) {
	if err := validateStructuredResponse(`{"answer":"ok"}`); err != nil {
		t.Errorf("valid JSON rejected: %v", err)
	}
	if err := validateStructuredResponse("not json"); err == nil {
		t.Error("invalid JSON accepted")
	}
}

func TestRewriteAssistantForHistory(t *testing.T) {
	msg := rewriteAssistantForHistory(`{"answer":"ok"}`)
	if msg.Role != llm.RoleAssistant {
		t.Errorf("role = %s", msg.Role)
	}
	if len(msg.Content) != 1 || msg.Content[0].Type != llm.BlockText {
		t.Errorf("content = %+v", msg.Content)
	}
	if msg.Content[0].Text != `{"answer":"ok"}` {
		t.Errorf("text = %q", msg.Content[0].Text)
	}
}

func TestExtractAnswer(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{`{"answer":"hello","confidence":"high"}`, "hello"},
		{`{"confidence":"low"}`, `{"confidence":"low"}`},       // no answer field
		{`not json`, `not json`},                                // invalid JSON
		{`{"answer":"","confidence":"low"}`, `{"answer":"","confidence":"low"}`}, // empty answer
	}
	for _, tt := range tests {
		if got := extractAnswer(tt.in); got != tt.want {
			t.Errorf("extractAnswer(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
