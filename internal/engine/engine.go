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

	"github.com/milenkom81/m/internal/llm"
	"github.com/milenkom81/m/internal/tools"
)

// defaultMaxTurns caps tool-use loops. A productive agent rarely needs more
// than a handful; this exists to stop runaway loops eating tokens forever.
const defaultMaxTurns = 12

// Config configures a Run or Session. The zero value is not usable:
// Provider and Model must be set.
type Config struct {
	Provider    llm.Provider
	Model       string
	System      string
	Tools       *tools.Registry
	Temperature *float64
	MaxTokens   int

	// Out receives streamed assistant text. nil means discard.
	Out io.Writer
	// Status receives tool-call indicators ("→ shell ls", "← 12 lines").
	// nil means discard. Typically wired to stderr.
	Status io.Writer

	// MaxTurns bounds the tool-use loop within a single Step. Zero uses
	// defaultMaxTurns.
	MaxTurns int
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
}

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
	wroteAny := false

	for turn := 0; turn < s.maxTurns; turn++ {
		events, err := s.cfg.Provider.Stream(ctx, llm.Request{
			Model:       s.cfg.Model,
			System:      s.cfg.System,
			Messages:    s.messages,
			Tools:       s.schemas,
			Temperature: s.cfg.Temperature,
			MaxTokens:   s.cfg.MaxTokens,
		})
		if err != nil {
			return err
		}

		assistant := llm.Message{Role: llm.RoleAssistant}
		var (
			stopReason string
			textIdx    = -1 // index of the in-progress text block, -1 if none
		)

		for ev := range events {
			switch ev.Kind {
			case llm.EventText:
				if textIdx < 0 {
					assistant.Content = append(assistant.Content, llm.ContentBlock{Type: llm.BlockText})
					textIdx = len(assistant.Content) - 1
				}
				assistant.Content[textIdx].Text += ev.Text
				if _, err := io.WriteString(s.out, ev.Text); err != nil {
					return err
				}
				wroteAny = true
			case llm.EventToolCall:
				textIdx = -1
				assistant.Content = append(assistant.Content, llm.ContentBlock{
					Type:      llm.BlockToolUse,
					ToolID:    ev.ToolID,
					ToolName:  ev.ToolName,
					ToolInput: ev.ToolInput,
				})
			case llm.EventDone:
				stopReason = ev.StopReason
			case llm.EventError:
				if wroteAny {
					fmt.Fprintln(s.out)
				}
				return ev.Err
			}
		}

		s.messages = append(s.messages, assistant)

		if stopReason != "tool_use" {
			if wroteAny {
				fmt.Fprintln(s.out)
			}
			return nil
		}

		results, err := executeTools(ctx, s.cfg.Tools, s.status, assistant.Content)
		if err != nil {
			return err
		}
		s.messages = append(s.messages, results)
	}

	return fmt.Errorf("engine: hit MaxTurns=%d without natural stop", s.maxTurns)
}

// executeTools runs each tool_use block in assistant and returns a single
// user message containing tool_result blocks in matching order.
func executeTools(ctx context.Context, reg *tools.Registry, status io.Writer, assistant []llm.ContentBlock) (llm.Message, error) {
	results := llm.Message{Role: llm.RoleUser}
	for _, b := range assistant {
		if b.Type != llm.BlockToolUse {
			continue
		}
		fmt.Fprintf(status, "→ %s %s\n", b.ToolName, summarizeInput(b.ToolInput))

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

		results.Content = append(results.Content, llm.ContentBlock{
			Type:      llm.BlockToolResult,
			ToolUseID: b.ToolID,
			Output:    content,
			IsError:   isErr,
		})
	}
	if len(results.Content) == 0 {
		// Should not happen — stop_reason was tool_use but no blocks. Surface it
		// rather than silently looping.
		return results, errors.New("engine: stop_reason=tool_use but no tool_use blocks present")
	}
	return results, nil
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
