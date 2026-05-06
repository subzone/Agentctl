package engine

import (
	"context"
	"io"
	"testing"

	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/tools"
)

// mockProvider returns canned responses for benchmarking.
type mockProvider struct{}

func (m *mockProvider) Stream(ctx context.Context, req llm.Request) (<-chan llm.Event, error) {
	ch := make(chan llm.Event)
	go func() {
		defer close(ch)
		// Send text response
		ch <- llm.Event{Kind: llm.EventText, Text: "Hello"}
		ch <- llm.Event{Kind: llm.EventUsage, Usage: llm.Usage{InputTokens: 100, OutputTokens: 50}}
		ch <- llm.Event{Kind: llm.EventDone, StopReason: "end_turn"}
	}()
	return ch, nil
}

// BenchmarkStep measures single turn latency with minimal tool execution.
func BenchmarkStep(b *testing.B) {
	reg := tools.NewRegistry(
		tools.NewFSRead(),
		tools.NewFSList(),
	)
	cfg := Config{
		Provider: &mockProvider{},
		Model:    "test-model",
		System:   "You are a helpful assistant.",
		Tools:    reg,
		Out:      io.Discard,
		Status:   io.Discard,
		MaxTurns: 1,
	}
	sess := NewSession(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sess.Reset()
		_ = sess.Step(context.Background(), "test message")
	}
}

// BenchmarkTokenCompact measures compaction performance.
func BenchmarkTokenCompact(b *testing.B) {
	// Create a large message history
	msgs := make([]llm.Message, 1000)
	for i := 0; i < 1000; i++ {
		role := llm.RoleUser
		if i%2 == 1 {
			role = llm.RoleAssistant
		}
		msgs[i] = llm.Message{
			Role: role,
			Content: []llm.ContentBlock{
				{Type: llm.BlockText, Text: "This is a test message with some content to make it realistic size for benchmarking purposes. We need enough text to get meaningful token estimates."},
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tokenCompact(msgs, 50000, "system prompt")
	}
}

// BenchmarkEstimateTokens measures token estimation speed.
func BenchmarkEstimateTokens(b *testing.B) {
	msgs := make([]llm.Message, 100)
	for i := 0; i < 100; i++ {
		msgs[i] = llm.Message{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{
				{Type: llm.BlockText, Text: "Test message for benchmarking token estimation performance"},
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = estimateTokens(msgs)
	}
}

// BenchmarkMessageCopy measures the cost of copying message history.
func BenchmarkMessageCopy(b *testing.B) {
	msgs := make([]llm.Message, 100)
	for i := 0; i < 100; i++ {
		msgs[i] = llm.Message{
			Role: llm.RoleAssistant,
			Content: []llm.ContentBlock{
				{Type: llm.BlockText, Text: "Response text"},
				{Type: llm.BlockToolUse, ToolName: "shell", ToolInput: []byte(`{"command":"ls"}`)},
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = append([]llm.Message(nil), msgs...)
	}
}