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
	"strings"
	"sync"
	"time"

	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/logging"
	"github.com/subzone/Agentctl/internal/ports"
	"github.com/subzone/Agentctl/internal/tools"
)

// Default configuration values. These can be overridden via Config fields.
const (
	// DefaultMaxTurns caps tool-use loops. A productive agent rarely needs more
	// than a handful; this exists to stop runaway loops eating tokens forever.
	DefaultMaxTurns = 15

	// DefaultContextBudget is the default context window size when not specified.
	// 128K is a conservative default that works for most models.
	DefaultContextBudget = 128_000

	// DefaultCompactionThreshold is the percentage of context budget at which
	// auto-compaction triggers. 80% leaves room for the response.
	DefaultCompactionThreshold = 80

	// DefaultCompactionTarget is the percentage of context budget to compact to.
	// 50% provides headroom for continued conversation.
	DefaultCompactionTarget = 50

	// DefaultMaxConcurrentTools limits how many tools can run simultaneously.
	// This prevents resource exhaustion when a model requests many tool calls.
	// 10 is a reasonable balance between parallelism and resource usage.
	DefaultMaxConcurrentTools = 10
)

// Config configures a Run or Session. The zero value is not usable:
// Provider and Model must be set.
type Config struct {
	Provider    llm.Provider
	Model       string
	System      string
	Tools       *tools.Registry
	Temperature *float64
	MaxTokens   int

	// PIIGuard scans outgoing messages for PII and redacts/warns.
	// nil means no PII scanning.
	PIIGuard *tools.PIIGuard

	// MaxContextTokens is the model's context window size. When estimated
	// input tokens exceed 80% of this, the session auto-compacts. Zero
	// means use a conservative default (128K).
	MaxContextTokens int

	// FallbackModels is a list of "provider/model" strings to try when
	// the primary model returns a rate-limit (429) or overload error.
	// Tried in order; the session switches to the first one that works.
	FallbackModels []string

	// ResolveModel resolves a "provider/model" string into a Provider and
	// bare model name. Required when FallbackModels is set. Typically
	// wired to llm.Resolve.
	ResolveModel func(model string) (llm.Provider, string, error)

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

	// PolicyCheck runs before any tool execution. Returning a non-empty
	// deny reason blocks execution and fails the current step.
	PolicyCheck func(ctx context.Context, toolName string, input json.RawMessage) (denyReason string, err error)

	// ContinueConfirm is called when MaxTurns is reached. Return true
	// to continue for another MaxTurns rounds, false to stop.
	// Nil means stop immediately.
	ContinueConfirm func(ctx context.Context, turns int) (bool, error)

	// ErrorIntervention is called when a tool execution fails. The callback
	// receives the tool name and the error message. It may return a hint
	// string that is appended to the error sent back to the LLM, helping
	// steer the model away from repeated failures. Return "" to let the
	// model retry on its own. Nil means no intervention (original behavior).
	ErrorIntervention func(ctx context.Context, toolName string, errMsg string) string

	// MaxConcurrentTools limits how many tools can run simultaneously.
	// Zero uses DefaultMaxConcurrentTools. This prevents resource exhaustion
	// when a model requests many tool calls in a single turn.
	MaxConcurrentTools int

	// Audit receives structured metadata events. Nil means disabled.
	Audit ports.AuditSink
	// AuditSessionID groups events for one run/chat session.
	AuditSessionID string
}

// PolicyViolationError is returned when policy denies a tool call.
type PolicyViolationError struct {
	Tool   string
	Reason string
}

func (e *PolicyViolationError) Error() string {
	if e == nil {
		return "policy violation"
	}
	if e.Reason == "" {
		return fmt.Sprintf("policy violation: tool %s denied", e.Tool)
	}
	return fmt.Sprintf("policy violation: %s", e.Reason)
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
	cfg           Config
	out           io.Writer
	status        io.Writer
	maxTurns      int
	maxConcurrent int // max tools running at once
	contextBudget int // max input tokens before compaction (80% of model window)
	schemas       []llm.ToolSchema
	messages      []llm.Message
	usage         llm.Usage // cumulative across all Steps
	lastIn        int       // input_tokens from the most recent provider call
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

// SetSystem replaces the system prompt for subsequent Steps.
func (s *Session) SetSystem(system string) {
	s.cfg.System = system
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
		maxTurns = DefaultMaxTurns
	}
	contextBudget := cfg.MaxContextTokens
	if contextBudget <= 0 {
		contextBudget = DefaultContextBudget
	}
	maxConcurrent := cfg.MaxConcurrentTools
	if maxConcurrent <= 0 {
		maxConcurrent = DefaultMaxConcurrentTools
	}
	return &Session{
		cfg:           cfg,
		out:           out,
		status:        status,
		maxTurns:      maxTurns,
		maxConcurrent: maxConcurrent,
		schemas:       buildSchemas(cfg.Tools),
		contextBudget: contextBudget,
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

// RestoreMessages replaces the message history with saved messages.
// Used to resume a previous session from persistent storage.
func (s *Session) RestoreMessages(msgs []llm.Message) {
	s.messages = msgs
}

// SetUsage restores cumulative token counts from a saved session.
func (s *Session) SetUsage(u llm.Usage) {
	s.usage = u
}

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

	// PII guard: scan and redact user input before sending to LLM.
	if s.cfg.PIIGuard != nil && s.cfg.PIIGuard.Mode != tools.PIIModeOff {
		if findings := s.cfg.PIIGuard.Scan(task); len(findings) > 0 {
			fmt.Fprintln(s.status, s.cfg.PIIGuard.Summary(findings))
			if s.cfg.PIIGuard.Mode == tools.PIIModeRedact {
				task = s.cfg.PIIGuard.Redact(task)
			}
		}
	}
	s.messages = append(s.messages, llm.TextMessage(llm.RoleUser, task))
	// Auto-compact when estimated tokens exceed threshold.
	if s.shouldCompact() {
		target := s.contextBudget * DefaultCompactionTarget / 100
		s.messages = validateMessages(tokenCompact(s.messages, target, s.cfg.System))
		fmt.Fprintf(s.status, "[auto-compacted to ~%dk tokens]\n", target/1000)
	}
	wroteAny := false

	for turn := 0; turn < s.maxTurns; turn++ {
		endLLM := logging.Span("llm.stream", "model", s.cfg.Model, "turn", turn)
		events, err := s.cfg.Provider.Stream(ctx, llm.Request{
			Model:          s.cfg.Model,
			System:         s.cfg.System,
			Messages:       s.messages,
			Tools:          s.schemas,
			Temperature:    s.cfg.Temperature,
			MaxTokens:      s.cfg.MaxTokens,
			ResponseSchema: s.cfg.ResponseSchema,
		})
		if err != nil && isRateLimitError(err) && len(s.cfg.FallbackModels) > 0 {
			events, err = s.tryFallbacks(ctx)
		}
		endLLM()
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
			results, err := executeTools(ctx, s.cfg.Tools, s.cfg.PolicyCheck, s.cfg.ToolConfirm, s.cfg.ErrorIntervention, s.cfg.Audit, s.cfg.AuditSessionID, s.status, s.maxConcurrent, assistant.Content)
			if err != nil {
				return err
			}
			s.messages = append(s.messages, results)
			if s.shouldCompact() {
				target := s.contextBudget * DefaultCompactionTarget / 100
				s.messages = validateMessages(tokenCompact(s.messages, target, s.cfg.System))
				fmt.Fprintf(s.status, "[auto-compacted to ~%dk tokens]\n", target/1000)
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
			return err
		}
		if ok {
			// Reset turn counter and continue.
			return s.Step(ctx, "continue from where you left off")
		}
	}
	fmt.Fprintf(s.out, "\n[reached %d tool turns, stopping]\n", s.maxTurns)
	return nil
}

// isRateLimitError returns true if the error indicates a rate limit (429)
// or overload (529) from the provider.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "overloaded") ||
		strings.Contains(msg, "529")
}

// tryFallbacks attempts each fallback model in order. On success, the
// session switches to that model for subsequent turns.
func (s *Session) tryFallbacks(ctx context.Context) (<-chan llm.Event, error) {
	if s.cfg.ResolveModel == nil {
		return nil, fmt.Errorf("no model resolver configured for fallbacks")
	}
	for _, fb := range s.cfg.FallbackModels {
		p, model, err := s.cfg.ResolveModel(fb)
		if err != nil {
			fmt.Fprintf(s.status, "[fallback %s: resolve error: %v]\n", fb, err)
			continue
		}
		events, err := p.Stream(ctx, llm.Request{
			Model:          model,
			System:         s.cfg.System,
			Messages:       s.messages,
			Tools:          s.schemas,
			Temperature:    s.cfg.Temperature,
			MaxTokens:      s.cfg.MaxTokens,
			ResponseSchema: s.cfg.ResponseSchema,
		})
		if err != nil {
			fmt.Fprintf(s.status, "[fallback %s: %v]\n", fb, err)
			continue
		}
		// Success — switch to this model for the rest of the session.
		s.cfg.Provider = p
		s.cfg.Model = model
		fmt.Fprintf(s.status, "[switched to fallback: %s]\n", fb)
		logging.Info("model.fallback", "to", fb)
		return events, nil
	}
	return nil, fmt.Errorf("all fallback models failed")
}

func validateMessages(msgs []llm.Message) []llm.Message {
	var clean []llm.Message
	for _, m := range msgs {
		if m.Role == llm.RoleUser && !allTextBlocks(m) {
			if len(clean) == 0 {
				continue
			}
			prev := clean[len(clean)-1]
			if prev.Role != llm.RoleAssistant {
				continue
			}
			hasTC := false
			for _, b := range prev.Content {
				if b.Type == llm.BlockToolUse {
					hasTC = true
					break
				}
			}
			if !hasTC {
				continue
			}
		}
		clean = append(clean, m)
	}
	if len(clean) == 0 {
		return []llm.Message{llm.TextMessage(llm.RoleUser, "(conversation compacted)")}
	}
	return clean
}

// estimateTokens approximates the token count for a message list.
// Uses ~4 chars per token heuristic — good enough for compaction decisions.
func estimateTokens(msgs []llm.Message) int {
	total := 0
	for _, m := range msgs {
		for _, b := range m.Content {
			total += len(b.Text) + len(b.Output) + len(b.ToolInput) + len(b.ToolName) + 20 // overhead
		}
	}
	return total / 4
}

// shouldCompact returns true when estimated tokens exceed the compaction threshold.
func (s *Session) shouldCompact() bool {
	est := estimateTokens(s.messages)
	// Also count system prompt.
	est += len(s.cfg.System) / 4
	return est > (s.contextBudget * DefaultCompactionThreshold / 100)
}

// tokenCompact removes the oldest messages until estimated tokens are under
// targetTokens. Cuts at safe boundaries (user text messages) and prepends
// a summary of what was dropped.
func tokenCompact(msgs []llm.Message, targetTokens int, system string) []llm.Message {
	systemTokens := len(system) / 4
	current := estimateTokens(msgs) + systemTokens
	if current <= targetTokens {
		return msgs
	}

	// Build a summary of the dropped portion.
	var summaryParts []string
	dropIdx := 0
	for dropIdx < len(msgs)-2 && current > targetTokens {
		m := msgs[dropIdx]
		// Estimate tokens for this message.
		msgTokens := 0
		for _, b := range m.Content {
			msgTokens += (len(b.Text) + len(b.Output) + len(b.ToolInput) + 20) / 4
		}
		// Collect summary snippets from user messages.
		if m.Role == llm.RoleUser && allTextBlocks(m) && len(m.Content) > 0 {
			snippet := m.Content[0].Text
			if len(snippet) > 80 {
				snippet = snippet[:80] + "..."
			}
			summaryParts = append(summaryParts, snippet)
		}
		current -= msgTokens
		dropIdx++
	}

	// Find a safe cut point (user text message) at or after dropIdx.
	cut := dropIdx
	for cut < len(msgs) {
		if msgs[cut].Role == llm.RoleUser && allTextBlocks(msgs[cut]) {
			break
		}
		cut++
	}
	if cut >= len(msgs)-1 {
		cut = dropIdx // fallback
	}

	// Build the compacted result with a meaningful summary.
	summary := fmt.Sprintf("(Earlier conversation compacted — %d messages removed. ", cut)
	if len(summaryParts) > 0 {
		max := 5
		if len(summaryParts) < max {
			max = len(summaryParts)
		}
		summary += "Topics discussed: "
		for i, s := range summaryParts[:max] {
			if i > 0 {
				summary += "; "
			}
			summary += s
		}
		if len(summaryParts) > max {
			summary += fmt.Sprintf(" and %d more", len(summaryParts)-max)
		}
		summary += ". "
	}
	summary += "Focus on the current task.)"

	result := make([]llm.Message, 0, len(msgs)-cut+1)
	result = append(result, llm.TextMessage(llm.RoleUser, summary))
	result = append(result, msgs[cut:]...)
	return result
}

// executeTools runs each tool_use block in assistant. When multiple tools
// are requested in the same turn, they run concurrently (each tool call
// is independent) but limited by maxConcurrent to prevent resource exhaustion.
// Results are collected in order.
func executeTools(ctx context.Context, reg *tools.Registry, policyCheck func(context.Context, string, json.RawMessage) (string, error), confirm func(context.Context, string, json.RawMessage) (bool, error), errIntervene func(context.Context, string, string) string, auditSink ports.AuditSink, auditSessionID string, status io.Writer, maxConcurrent int, assistant []llm.ContentBlock) (llm.Message, error) {
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
		result, err := runToolBlock(ctx, reg, policyCheck, confirm, errIntervene, auditSink, auditSessionID, status, b)
		if err != nil {
			return llm.Message{}, err
		}
		return llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{result}}, nil
	}

	// Multiple tool calls — run concurrently with semaphore to limit parallelism.
	// This prevents resource exhaustion when model requests many tool calls.
	type indexed struct {
		idx    int
		result llm.ContentBlock
		err    error
	}
	ch := make(chan indexed, len(calls))

	// Semaphore to limit concurrent tool executions
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for i, b := range calls {
		wg.Add(1)
		fmt.Fprintf(status, "→ %s %s\n", b.ToolName, summarizeInput(b.ToolInput))
		go func(i int, b llm.ContentBlock) {
			defer wg.Done()
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }() // Release semaphore

			result, err := runToolBlock(ctx, reg, policyCheck, confirm, errIntervene, auditSink, auditSessionID, status, b)
			ch <- indexed{idx: i, result: result, err: err}
		}(i, b)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(ch)
	}()

	results := make([]llm.ContentBlock, len(calls))
	var firstErr error
	for r := range ch {
		results[r.idx] = r.result
		if firstErr == nil && r.err != nil {
			firstErr = r.err
		}
	}
	if firstErr != nil {
		return llm.Message{}, firstErr
	}
	return llm.Message{Role: llm.RoleUser, Content: results}, nil
}

// readOnlyTools is the set of tool names that never need user confirmation
// because they only read data without making any changes.
var readOnlyTools = map[string]bool{"fs_read": true, "fs_list": true, "web_fetch": true, "code_search": true}

func runToolBlock(ctx context.Context, reg *tools.Registry, policyCheck func(context.Context, string, json.RawMessage) (string, error), confirm func(context.Context, string, json.RawMessage) (bool, error), errIntervene func(context.Context, string, string) string, auditSink ports.AuditSink, auditSessionID string, status io.Writer, b llm.ContentBlock) (llm.ContentBlock, error) {
	// Read-only tools skip confirmation.
	endTool := logging.Span("tool.exec", "tool", b.ToolName)
	defer endTool()
	logging.Debug("tool.call", "tool", b.ToolName, "input_size", len(b.ToolInput))
	started := time.Now()
	emitAuditEvent(ctx, auditSink, ports.AuditEvent{Type: "tool_call", SessionID: auditSessionID, Tool: b.ToolName, Args: b.ToolInput})
	if policyCheck != nil {
		denyReason, err := policyCheck(ctx, b.ToolName, b.ToolInput)
		if err != nil {
			emitAuditEvent(ctx, auditSink, ports.AuditEvent{Type: "tool_result", SessionID: auditSessionID, Tool: b.ToolName, Outcome: "policy_error", Error: err.Error(), DurationMS: time.Since(started).Milliseconds()})
			return llm.ContentBlock{Type: llm.BlockToolResult, ToolUseID: b.ToolID, Output: "policy check failed", IsError: true}, err
		}
		if denyReason != "" {
			fmt.Fprintf(status, "← policy deny: %s\n", denyReason)
			emitAuditEvent(ctx, auditSink, ports.AuditEvent{Type: "tool_result", SessionID: auditSessionID, Tool: b.ToolName, Outcome: "policy_denied", Error: denyReason, DurationMS: time.Since(started).Milliseconds()})
			return llm.ContentBlock{Type: llm.BlockToolResult, ToolUseID: b.ToolID, Output: "policy denied: " + denyReason, IsError: true}, &PolicyViolationError{Tool: b.ToolName, Reason: denyReason}
		}
	}
	if confirm != nil && !readOnlyTools[b.ToolName] {
		ok, err := confirm(ctx, b.ToolName, b.ToolInput)
		if err != nil {
			emitAuditEvent(ctx, auditSink, ports.AuditEvent{Type: "tool_result", SessionID: auditSessionID, Tool: b.ToolName, Outcome: "cancelled", Error: err.Error(), DurationMS: time.Since(started).Milliseconds()})
			return llm.ContentBlock{
				Type: llm.BlockToolResult, ToolUseID: b.ToolID,
				Output: "user cancelled", IsError: true,
			}, nil
		}
		if !ok {
			emitAuditEvent(ctx, auditSink, ports.AuditEvent{Type: "tool_result", SessionID: auditSessionID, Tool: b.ToolName, Outcome: "declined", DurationMS: time.Since(started).Milliseconds()})
			return llm.ContentBlock{
				Type: llm.BlockToolResult, ToolUseID: b.ToolID,
				Output: "user declined this action", IsError: false,
			}, nil
		}
	}
	// Run tool with progress indicator for long-running operations.
	output, err := runWithProgress(ctx, reg, status, b)
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
		// Let the user inject a hint before the error goes back to the LLM.
		if errIntervene != nil {
			if hint := errIntervene(ctx, b.ToolName, content); hint != "" {
				content += "\nUser hint: " + hint
			}
		}
		emitAuditEvent(ctx, auditSink, ports.AuditEvent{Type: "tool_result", SessionID: auditSessionID, Tool: b.ToolName, Outcome: "error", Error: err.Error(), DurationMS: time.Since(started).Milliseconds()})
	} else {
		fmt.Fprintf(status, "← %s\n", summarizeToolOutput(b.ToolName, output))
		emitAuditEvent(ctx, auditSink, ports.AuditEvent{Type: "tool_result", SessionID: auditSessionID, Tool: b.ToolName, Outcome: "success", DurationMS: time.Since(started).Milliseconds()})
	}
	return llm.ContentBlock{
		Type:      llm.BlockToolResult,
		ToolUseID: b.ToolID,
		Output:    content,
		IsError:   isErr,
	}, nil
}

func emitAuditEvent(ctx context.Context, sink ports.AuditSink, ev ports.AuditEvent) {
	if sink == nil {
		return
	}
	if err := sink.Emit(ctx, ev); err != nil {
		logging.Warn("audit.emit.failed", "error", err)
	}
}

func runOne(ctx context.Context, reg *tools.Registry, b llm.ContentBlock) (string, error) {
	if reg == nil {
		return "", fmt.Errorf("no tool registry; cannot run %q", b.ToolName)
	}
	return reg.Run(ctx, b.ToolName, b.ToolInput)
}

// runWithProgress runs a tool and prints elapsed time every 5s for long operations.
func runWithProgress(ctx context.Context, reg *tools.Registry, status io.Writer, b llm.ContentBlock) (string, error) {
	type result struct {
		output string
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := runOne(ctx, reg, b)
		ch <- result{out, err}
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	elapsed := 0
	for {
		select {
		case r := <-ch:
			return r.output, r.err
		case <-ticker.C:
			elapsed += 5
			fmt.Fprintf(status, "  ⏳ %s running... %ds\n", b.ToolName, elapsed)
		}
	}
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

// summarizeToolOutput produces a human-friendly summary of tool output.
func summarizeToolOutput(toolName, output string) string {
	lines := strings.Count(output, "\n")
	size := len(output)
	switch {
	case size == 0:
		return "(empty)"
	case lines > 1:
		return fmt.Sprintf("%d lines, %d bytes", lines, size)
	default:
		if size > 80 {
			return fmt.Sprintf("%d bytes", size)
		}
		return strings.TrimSpace(output)
	}
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
