package llm_test

import (
	"testing"

	"github.com/subzone/Agentctl/internal/llm"
)

func TestSupportsVision(t *testing.T) {
	if !llm.SupportsVision("openai/gpt-4o") {
		t.Fatal("gpt-4o should support vision")
	}
	if !llm.SupportsVision("anthropic/claude-sonnet-4-6") {
		t.Fatal("claude sonnet 4 should support vision")
	}
	if llm.SupportsVision("ollama/llama3.2") {
		t.Fatal("llama3.2 should not match vision heuristic")
	}
}

func TestUserBlocksWithImage(t *testing.T) {
	msg := llm.UserBlocks("describe this", llm.ImageBlock("image/png", []byte{1, 2, 3}))
	if len(msg.Content) != 2 {
		t.Fatalf("blocks: %d", len(msg.Content))
	}
	if msg.Content[1].Type != llm.BlockImage {
		t.Fatalf("type: %s", msg.Content[1].Type)
	}
}
