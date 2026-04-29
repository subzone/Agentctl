// Package anthropic implements the llm.Provider contract against the
// Anthropic Messages API. Stdlib-only: net/http for transport, bufio for
// SSE, encoding/json for payloads.
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/milenkom81/m/internal/llm"
)

const (
	defaultBaseURL = "https://api.anthropic.com"
	apiVersion     = "2023-06-01"
)

func init() {
	llm.Register("anthropic", func() (llm.Provider, error) { return New() })
}

// Provider is an Anthropic Messages API client.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// Option configures a Provider.
type Option func(*Provider)

// WithAPIKey overrides the key picked up from ANTHROPIC_API_KEY.
func WithAPIKey(k string) Option { return func(p *Provider) { p.apiKey = k } }

// WithBaseURL points the client at a custom origin (used by tests).
func WithBaseURL(u string) Option { return func(p *Provider) { p.baseURL = u } }

// WithHTTPClient swaps the underlying http.Client (used by tests).
func WithHTTPClient(c *http.Client) Option { return func(p *Provider) { p.client = c } }

// New constructs a Provider. ANTHROPIC_API_KEY is required unless WithAPIKey
// is supplied.
func New(opts ...Option) (*Provider, error) {
	p := &Provider{
		apiKey:  os.Getenv("ANTHROPIC_API_KEY"),
		baseURL: defaultBaseURL,
		client:  http.DefaultClient,
	}
	for _, o := range opts {
		o(p)
	}
	if p.apiKey == "" {
		return nil, errors.New("ANTHROPIC_API_KEY is not set")
	}
	return p, nil
}

type messagePayload struct {
	Model       string         `json:"model"`
	System      string         `json:"system,omitempty"`
	Messages    []apiMessage   `json:"messages"`
	Tools       []apiTool      `json:"tools,omitempty"`
	MaxTokens   int            `json:"max_tokens"`
	Temperature *float64       `json:"temperature,omitempty"`
	Stream      bool           `json:"stream"`
}

type apiMessage struct {
	Role    string `json:"role"`
	Content []any  `json:"content"`
}

type apiTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Stream sends req to /v1/messages with stream=true and forwards SSE events
// onto the returned channel.
func (p *Provider) Stream(ctx context.Context, req llm.Request) (<-chan llm.Event, error) {
	msgs, err := buildMessages(req.Messages)
	if err != nil {
		return nil, err
	}
	tools := buildTools(req.Tools)

	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = 1024
	}
	body, err := json.Marshal(messagePayload{
		Model:       req.Model,
		System:      req.System,
		Messages:    msgs,
		Tools:       tools,
		MaxTokens:   maxTok,
		Temperature: req.Temperature,
		Stream:      true,
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("anthropic %s: %s", resp.Status, strings.TrimSpace(string(slurp)))
	}

	out := make(chan llm.Event, 8)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		if err := parseSSE(ctx, resp.Body, out); err != nil {
			select {
			case out <- llm.Event{Kind: llm.EventError, Err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return out, nil
}

// buildMessages translates llm.Messages into the API's content-block shape.
func buildMessages(msgs []llm.Message) ([]apiMessage, error) {
	out := make([]apiMessage, 0, len(msgs))
	for i, m := range msgs {
		blocks := make([]any, 0, len(m.Content))
		for j, b := range m.Content {
			ab, err := toAPIBlock(b)
			if err != nil {
				return nil, fmt.Errorf("message %d block %d: %w", i, j, err)
			}
			blocks = append(blocks, ab)
		}
		out = append(out, apiMessage{Role: string(m.Role), Content: blocks})
	}
	return out, nil
}

func toAPIBlock(b llm.ContentBlock) (any, error) {
	switch b.Type {
	case llm.BlockText:
		return map[string]any{"type": "text", "text": b.Text}, nil
	case llm.BlockToolUse:
		// input is an object on the wire; default to {} when no input was
		// captured (which can happen for parameterless tools).
		input := json.RawMessage(b.ToolInput)
		if len(input) == 0 {
			input = json.RawMessage("{}")
		}
		return map[string]any{
			"type":  "tool_use",
			"id":    b.ToolID,
			"name":  b.ToolName,
			"input": input,
		}, nil
	case llm.BlockToolResult:
		out := map[string]any{
			"type":        "tool_result",
			"tool_use_id": b.ToolUseID,
			"content":     b.Output,
		}
		if b.IsError {
			out["is_error"] = true
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown block type %q", b.Type)
	}
}

func buildTools(tools []llm.ToolSchema) []apiTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]apiTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, apiTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	return out
}

// blockState tracks an in-flight content block during SSE streaming.
type blockState struct {
	typ       string
	toolID    string
	toolName  string
	inputJSON strings.Builder
}

// parseSSE reads an Anthropic event-stream and translates relevant events
// (content_block_*, message_delta stop_reason, message_stop, error) into
// llm.Events on out. Unknown events are ignored.
func parseSSE(ctx context.Context, r io.Reader, out chan<- llm.Event) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	var (
		event      string
		data       strings.Builder
		stopReason string
		blocks     = map[int]*blockState{}
	)

	send := func(ev llm.Event) error {
		select {
		case out <- ev:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	dispatch := func() error {
		if event == "" && data.Len() == 0 {
			return nil
		}
		raw := data.String()
		ev, dat := event, raw
		event = ""
		data.Reset()

		switch ev {
		case "content_block_start":
			var d struct {
				Index        int `json:"index"`
				ContentBlock struct {
					Type string `json:"type"`
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"content_block"`
			}
			if err := json.Unmarshal([]byte(dat), &d); err != nil {
				return fmt.Errorf("parse content_block_start: %w", err)
			}
			blocks[d.Index] = &blockState{
				typ:      d.ContentBlock.Type,
				toolID:   d.ContentBlock.ID,
				toolName: d.ContentBlock.Name,
			}
		case "content_block_delta":
			var d struct {
				Index int `json:"index"`
				Delta struct {
					Type        string `json:"type"`
					Text        string `json:"text"`
					PartialJSON string `json:"partial_json"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(dat), &d); err != nil {
				return fmt.Errorf("parse content_block_delta: %w", err)
			}
			switch d.Delta.Type {
			case "text_delta":
				if d.Delta.Text != "" {
					if err := send(llm.Event{Kind: llm.EventText, Text: d.Delta.Text}); err != nil {
						return err
					}
				}
			case "input_json_delta":
				st := blocks[d.Index]
				if st == nil {
					st = &blockState{typ: "tool_use"}
					blocks[d.Index] = st
				}
				st.inputJSON.WriteString(d.Delta.PartialJSON)
			}
		case "content_block_stop":
			var d struct {
				Index int `json:"index"`
			}
			_ = json.Unmarshal([]byte(dat), &d)
			st := blocks[d.Index]
			if st != nil && st.typ == "tool_use" {
				input := st.inputJSON.String()
				if input == "" {
					input = "{}"
				}
				if err := send(llm.Event{
					Kind:      llm.EventToolCall,
					ToolID:    st.toolID,
					ToolName:  st.toolName,
					ToolInput: json.RawMessage(input),
				}); err != nil {
					return err
				}
			}
			delete(blocks, d.Index)
		case "message_delta":
			var d struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(dat), &d); err == nil {
				if d.Delta.StopReason != "" {
					stopReason = d.Delta.StopReason
				}
			}
		case "message_stop":
			return send(llm.Event{Kind: llm.EventDone, StopReason: stopReason})
		case "error":
			var d struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.Unmarshal([]byte(dat), &d)
			if d.Error.Message != "" {
				return fmt.Errorf("anthropic %s: %s", d.Error.Type, d.Error.Message)
			}
			return errors.New("anthropic stream error")
		}
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimPrefix(val, " ")
		switch key {
		case "event":
			event = val
		case "data":
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(val)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return dispatch()
}
