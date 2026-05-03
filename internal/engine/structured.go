package engine

import (
	"encoding/json"
	"fmt"

	"github.com/subzone/Agentctl/internal/llm"
)

const structuredResponseTool = "structured_response"

// extractStructuredResponse processes an assistant message when
// ResponseSchema is active. It handles two cases:
//
//  1. Anthropic response-tool pattern: the model called structured_response
//     as a tool_use block. Extract the tool input JSON as the response.
//  2. OpenAI/Ollama: the model produced a text block containing JSON.
//
// Returns the full JSON string and the user-facing answer extracted from
// the "answer" field (if present). If the response is not valid JSON,
// returns the raw text as both.
func extractStructuredResponse(assistant llm.Message) (fullJSON, display string) {
	// Case 1: look for the structured_response tool call (Anthropic pattern).
	for _, b := range assistant.Content {
		if b.Type == llm.BlockToolUse && b.ToolName == structuredResponseTool {
			fullJSON = string(b.ToolInput)
			display = extractAnswer(fullJSON)
			return fullJSON, display
		}
	}

	// Case 2: concatenate text blocks (OpenAI/Ollama produce text JSON).
	var text string
	for _, b := range assistant.Content {
		if b.Type == llm.BlockText {
			text += b.Text
		}
	}
	if text == "" {
		return "", ""
	}

	// Validate it's JSON.
	if !json.Valid([]byte(text)) {
		return text, text
	}

	display = extractAnswer(text)
	return text, display
}

// extractAnswer pulls the "answer" field from a JSON object. If the field
// is missing or the JSON is invalid, returns the full string as-is.
func extractAnswer(j string) string {
	var obj struct {
		Answer string `json:"answer"`
	}
	if err := json.Unmarshal([]byte(j), &obj); err == nil && obj.Answer != "" {
		return obj.Answer
	}
	return j
}

// validateStructuredResponse checks that raw is valid JSON. Returns an
// error describing the problem if not. Schema-level validation (checking
// required fields, types, etc.) is left to the provider's constrained
// decoding — this is a safety net.
func validateStructuredResponse(raw string) error {
	if !json.Valid([]byte(raw)) {
		return fmt.Errorf("structured response is not valid JSON: %.100s", raw)
	}
	return nil
}

// rewriteAssistantForHistory replaces the assistant message content with
// a single text block containing the full JSON. This ensures the hub sees
// the structured JSON in the tool_result, not the raw tool_use blocks.
func rewriteAssistantForHistory(fullJSON string) llm.Message {
	return llm.Message{
		Role: llm.RoleAssistant,
		Content: []llm.ContentBlock{{
			Type: llm.BlockText,
			Text: fullJSON,
		}},
	}
}
