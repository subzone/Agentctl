// Package engine drives the agent loop: send messages → consume the
// streamed response → execute requested tools → loop until the model
// stops on its own.
//
// Two surfaces:
//   - Run executes one task to completion in a fresh conversation. Use
//     this for `agent run` and other one-shot callers.
//   - Session keeps message history across calls. Use this for `agent
//     chat` or any caller that needs multi-turn memory.
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/subzone/m/internal/llm"
	"github.com/subzone/m/internal/tools"
)

// defaultMaxTurns caps tool-use loops. A productive agent rarely needs more
// than a handful; this exists to stop runaway loops eating tokens forever.
const defaultMaxTurns = 15

// Config configures a Run or Session. The zero value is not usable:
// Provider and Model must be set.
type Config struct {
	Provider    llm.Provider
	Model       string
	System      string
	Tools       *tools.Registry
	Temperature *float64
	MaxTokens   int

	// ResponseSchema constrains the model to produce valid JSON matching
	// this schema. Nil means unconstrained text output.
	ResponseSchema json.RawMessage

	// Out receives streamed assistant text. nil means discard.
	Out io.Writer
	// Status receives tool-call indicators ("→ shell ls", "← 12 lines").
	// nil means discard. Typically wired to stderr.
	Status io.Writer

	// MaxTurns bounds the tool-use loop within a single Step. Zero uses
	// defaultMaxTurns.
	MaxTurns int

	// ToolConfirm is called before executing any destructive tool.
	// Return true to proceed, false to skip. Nil means auto-approve.
	ToolConfirm func(ctx context.Context, toolName string, input json.RawMessage) (bool, error)

	// ContinueConfirm is called when MaxTurns is reached. Return true
	// to continue for another MaxTurns rounds, false to stop.
	// Nil means stop immediately.
	ContinueConfirm func(ctx context.Context, turns int) (bool, error)
}

// Run executes one task to completion in a fresh conversation. It is a
// thin wrapper around NewSession + Step for callers that don't need
// history.
func Run(ctx context.Context, cfg Config, task string) error {
	return NewSession(cfg).Step(ctx, task)
}

// Session is a stateful, multi-turn conversation driver. Step appends a
// user turn and runs the engine loop; the resulting message history is
// retained on the Session and reused on subsequent Steps.
type Session struct {
	cfg      Config
	out      io.Writer
	status   io.Writer
	maxTurns int
	schemas  []llm.ToolSchema
	messages []llm.Message
	usage    llm.Usage // cumulative across all Steps
	lastIn   int       // input_tokens from the most recent provider call
}

// Usage returns the cumulative token counts across all Steps in this session.
func (s *Session) Usage() llm.Usage { return s.usage }

// LastInputTokens returns the input_tokens from the most recent provider
// call. This approximates the current context window consumption.
func (s *Session) LastInputTokens() int { return s.lastIn }

// SetModel hot-swaps the provider and model for subsequent Steps.
// Existing message history is preserved.
func (s *Session) SetModel(p llm.Provider, model string) {
	s.cfg.Provider = p
	s.cfg.Model = model
}

// Model returns the current bare model id.
func (s *Session) Model() string { return s.cfg.Model }

// NewSession constructs a Session. It validates required Config fields
// lazily on the first Step call (so a Session can be built before all
// dependencies are wired, e.g. in tests).
func NewSession(cfg Config) *Session {
	out := cfg.Out
	if out == nil {
		out = io.Discard
	}
	status := cfg.Status
	if status == nil {
		status = io.Discard
	}
	maxTurns := cfg.MaxTurns
	if maxTurns <= 0 {
		maxTurns = defaultMaxTurns
	}
	return &Session{
		cfg:      cfg,
		out:      out,
		status:   status,
		maxTurns: maxTurns,
		schemas:  buildSchemas(cfg.Tools),
	}
}

// Messages returns a snapshot of the rolling history. The outer slice and
// each Message's Content slice are fresh copies; the ContentBlock values
// themselves (and any contained byte slices) are shared, so callers
// shouldn't mutate field-level data either.
func (s *Session) Messages() []llm.Message {
	out := make([]llm.Message, len(s.messages))
	for i, m := range s.messages {
		out[i] = m
		out[i].Content = append([]llm.ContentBlock(nil), m.Content...)
	}
	return out
}

// Reset clears the conversation history. Tool registry and provider
// remain configured.
func (s *Session) Reset() { s.messages = nil }

// Truncate keeps at most the last maxExchanges user-initiated exchanges.
// A user-initiated exchange begins at a user message containing only
// text blocks (i.e., not a tool_result-bearing follow-up). All messages
// from that point through the next exchange-start are retained as a
// unit, so an assistant turn's tool_use is never split from its matching
// tool_result. maxExchanges <= 0 is a no-op.
func (s *Session) Truncate(maxExchanges int) {
	if maxExchanges <= 0 {
		return
	}
	var starts []int
	for i, m := range s.messages {
		if m.Role == llm.RoleUser && allTextBlocks(m) {
			starts = append(starts, i)
		}
	}
	if len(starts) <= maxExchanges {
		return
	}
	keepFrom := starts[len(starts)-maxExchanges]
	s.messages = append([]llm.Message(nil), s.messages[keepFrom:]...)
}

// Step appends task as a user turn and runs the engine loop until the
// model stops naturally (or hits MaxTurns). The assistant's response
// streams to s.cfg.Out; full message turns are appended to the history.
func (s *Session) Step(ctx context.Context, task string) error {
	if s.cfg.Provider == nil {
		return errors.New("engine: Provider is required")
	}
	if s.cfg.Model == "" {
		return errors.New("engine: Model is required")
	}

	s.messages = append(s.messages, llm.TextMessage(llm.RoleUser, task))
	// Auto-compact if message history is too large to prevent context overflow.
	if len(s.messages) > 30 {
		s.messages = safeCompact(s.messages, 10)
		fmt.Fprintln(s.status, "[auto-compacted: context was too large]")
	}
	wroteAny := false

	for turn := 0; turn < s.maxTurns; turn++ {
		events, err := s.cfg.Provider.Stream(ctx, llm.Request{
			Model:          s.cfg.Model,
			System:         s.cfg.System,
			Messages:       s.messages,
			Tools:          s.schemas,
			Temperature:    s.cfg.Temperature,
			MaxTokens:      s.cfg.MaxTokens,
			ResponseSchema: s.cfg.ResponseSchema,
		})
		if err != nil {
			return err
		}

		assistant := llm.Message{Role: llm.RoleAssistant}
		var (
			stopReason string
			textIdx    = -1
		)
		hasSchema := len(s.cfg.ResponseSchema) > 0

		for ev := range events {
			switch ev.Kind {
			case llm.EventText:
				if textIdx < 0 {
					assistant.Content = append(assistant.Content, llm.ContentBlock{Type: llm.BlockText})
					textIdx = len(assistant.Content) - 1
				}
				assistant.Content[textIdx].Text += ev.Text
				// When structured output is active, buffer instead of streaming.
				if !hasSchema {
					if _, err := io.WriteString(s.out, ev.Text); err != nil {
						return err
					}
					wroteAny = true
				}
			case llm.EventToolCall:
				textIdx = -1
				assistant.Content = append(assistant.Content, llm.ContentBlock{
					Type:      llm.BlockToolUse,
					ToolID:    ev.ToolID,
					ToolName:  ev.ToolName,
					ToolInput: ev.ToolInput,
				})
			case llm.EventUsage:
				s.usage.InputTokens += ev.Usage.InputTokens
				s.usage.OutputTokens += ev.Usage.OutputTokens
				s.lastIn = ev.Usage.InputTokens
			case llm.EventDone:
				stopReason = ev.StopReason
			case llm.EventError:
				if wroteAny {
					fmt.Fprintln(s.out)
				}
				return ev.Err
			}
		}

		// Structured output post-processing: extract JSON, validate,
		// display only the answer field, store full JSON in history.
		if hasSchema {
			fullJSON, display := extractStructuredResponse(assistant)
			if fullJSON != "" {
				if err := validateStructuredResponse(fullJSON); err != nil {
					fmt.Fprintf(s.status, "warning: %v\n", err)
				}
				if _, err := io.WriteString(s.out, display); err != nil {
					return err
				}
				wroteAny = true
			}
			// Rewrite history so the hub sees clean JSON, not tool_use blocks.
			if fullJSON != "" {
				assistant = rewriteAssistantForHistory(fullJSON)
			}
		}

		s.messages = append(s.messages, assistant)

		switch stopReason {
		case "tool_use":
			// If structured output extracted the response from a tool call,
			// don't execute tools — the response-tool is synthetic.
			if hasSchema {
				if wroteAny {
					fmt.Fprintln(s.out)
				}
				return nil
			}
			results, err := executeTools(ctx, s.cfg.Tools, s.cfg.ToolConfirm, s.status, assistant.Content)
			if err != nil {
				return err
			}
			s.messages = append(s.messages, results)
			if len(s.messages) > 30 {
				s.messages = safeCompact(s.messages, 10)
			}

		case "max_tokens":
			// Output was truncated — ask the model to continue seamlessly.
			s.messages = append(s.messages, llm.TextMessage(llm.RoleUser, "continue"))

		default:
			// Normal end ("end_turn" or anything else) — done.
			if wroteAny {
				fmt.Fprintln(s.out)
			}
			return nil
		}
	}

	if s.cfg.ContinueConfirm != nil {
		ok, err := s.cfg.ContinueConfirm(ctx, s.maxTurns)
		if err != nil {
			return nil
		}
		if ok {
			// Reset turn counter and continue.
			return s.Step(ctx, "continue from where you left off")
		}
	}
	fmt.Fprintf(s.out, "\n[reached %d tool turns, stopping]\n", s.maxTurns)
	return nil
}

// safeCompact keeps the last n messages but ensures the cut point is at
// a user text message, not in the middle of a tool_calls/tool_result pair.
// This prevents "tool message must follow tool_calls" errors from providers.
func safeCompact(msgs []llm.Message, keep int) []llm.Message {
	if len(msgs) <= keep {
		return msgs
	}
	target := len(msgs) - keep
	if target < 1 {
		target = 1
	}
	// Search forward from target for a user text message (safe cut point).
	cut := -1
	for i := target; i < len(msgs); i++ {
		if msgs[i].Role == llm.RoleUser && allTextBlocks(msgs[i]) {
			cut = i
			break
		}
	}
	if cut < 0 {
		// Search backward.
		for i := target - 1; i >= 0; i-- {
			if msgs[i].Role == llm.RoleUser && allTextBlocks(msgs[i]) {
				cut = i
				break
			}
		}
	}
	if cut < 0 || cut >= len(msgs)-1 {
		// Fallback: prepend synthetic user message + keep last few.
		n := 4
		if len(msgs) < n {
			n = len(msgs)
		}
		result := make([]llm.Message, 0, n+1)
		result = append(result, llm.TextMessage(llm.RoleUser, "(conversation compacted)"))
		tail := msgs[len(msgs)-n:]
		// Skip any leading tool result messages in the tail.
		for len(tail) > 0 && tail[0].Role == llm.RoleUser && !allTextBlocks(tail[0]) {
			tail = tail[1:]
		}
		// Also skip orphaned assistant tool_calls at the start.
		for len(tail) > 0 && tail[0].Role == llm.RoleAssistant && hasToolCalls(tail[0]) {
			tail = tail[1:]
		}
		result = append(result, tail...)
		return result
	}
	return append([]llm.Message(nil), msgs[cut:]...)
}

// hasToolCalls reports whether an assistant message contains tool_use blocks.
func hasToolCalls(m llm.Message) bool {
	for _, b := range m.Content {
		if b.Type == llm.BlockToolUse {
			return true
		}
	}
	return false
}


// executeTools runs each tool_use block in assistant. When multiple tools
// are requested in the same turn, they run concurrently (each tool call
// is independent). Results are collected in order.
func executeTools(ctx context.Context, reg *tools.Registry, confirm func(context.Context, string, json.RawMessage) (bool, error), status io.Writer, assistant []llm.ContentBlock) (llm.Message, error) {
	var calls []llm.ContentBlock
	for _, b := range assistant {
		if b.Type == llm.BlockToolUse {
			calls = append(calls, b)
		}
	}
	if len(calls) == 0 {
		return llm.Message{}, errors.New("engine: stop_reason=tool_use but no tool_use blocks present")
	}

	// Single tool call — run inline, no goroutine overhead.
	if len(calls) == 1 {
		b := calls[0]
		fmt.Fprintf(status, "→ %s %s\n", b.ToolName, summarizeInput(b.ToolInput))
		result := runToolBlock(ctx, reg, confirm, status, b)
		return llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{result}}, nil
	}

	// Multiple tool calls — run concurrently.
	type indexed struct {
		idx    int
		result llm.ContentBlock
	}
	ch := make(chan indexed, len(calls))
	for i, b := range calls {
		fmt.Fprintf(status, "→ %s %s\n", b.ToolName, summarizeInput(b.ToolInput))
		go func(i int, b llm.ContentBlock) {
			ch <- indexed{idx: i, result: runToolBlock(ctx, reg, confirm, status, b)}
		}(i, b)
	}

	results := make([]llm.ContentBlock, len(calls))
	for range calls {
		r := <-ch
		results[r.idx] = r.result
	}
	return llm.Message{Role: llm.RoleUser, Content: results}, nil
}

func runToolBlock(ctx context.Context, reg *tools.Registry, confirm func(context.Context, string, json.RawMessage) (bool, error), status io.Writer, b llm.ContentBlock) llm.ContentBlock {
	// Read-only tools skip confirmation.
	readOnly := map[string]bool{"fs_read": true, "fs_list": true}
	if confirm != nil && !readOnly[b.ToolName] {
		ok, err := confirm(ctx, b.ToolName, b.ToolInput)
		if err != nil {
			return llm.ContentBlock{
				Type: llm.BlockToolResult, ToolUseID: b.ToolID,
				Output: "user cancelled", IsError: true,
			}
		}
		if !ok {
			return llm.ContentBlock{
				Type: llm.BlockToolResult, ToolUseID: b.ToolID,
				Output: "user declined this action", IsError: false,
			}
		}
	}
	output, err := runOne(ctx, reg, b)
	isErr := false
	content := output
	if err != nil {
		isErr = true
		if content == "" {
			content = err.Error()
		} else {
			content = output + "\nERROR: " + err.Error()
		}
		fmt.Fprintf(status, "← error: %v\n", err)
	} else {
		fmt.Fprintf(status, "← %d bytes\n", len(output))
	}
	return llm.ContentBlock{
		Type:      llm.BlockToolResult,
		ToolUseID: b.ToolID,
		Output:    content,
		IsError:   isErr,
	}
}

func runOne(ctx context.Context, reg *tools.Registry, b llm.ContentBlock) (string, error) {
	if reg == nil {
		return "", fmt.Errorf("no tool registry; cannot run %q", b.ToolName)
	}
	return reg.Run(ctx, b.ToolName, b.ToolInput)
}

func buildSchemas(reg *tools.Registry) []llm.ToolSchema {
	if reg == nil {
		return nil
	}
	all := reg.All()
	if len(all) == 0 {
		return nil
	}
	out := make([]llm.ToolSchema, 0, len(all))
	for _, t := range all {
		out = append(out, llm.ToolSchema{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return out
}

// summarizeInput produces a short single-line representation of a tool's
// JSON input for the status log.
func summarizeInput(input json.RawMessage) string {
	const limit = 120
	s := string(input)
	if len(s) > limit {
		return s[:limit] + "..."
	}
	return s
}

// allTextBlocks reports whether every block in m is a text block (and the
// message has at least one). Used by Truncate to identify exchange starts.
func allTextBlocks(m llm.Message) bool {
	if len(m.Content) == 0 {
		return false
	}
	for _, b := range m.Content {
		if b.Type != llm.BlockText {
			return false
		}
	}
	return true
}
