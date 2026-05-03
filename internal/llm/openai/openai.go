// Package openai implements the llm.Provider contract against the OpenAI
// Chat Completions API. Stdlib-only.
//
// Translation notes:
//   - llm.ContentBlock is folded into OpenAI's flatter message shape: an
//     assistant turn collapses text blocks into one message.content and
//     tool_use blocks into message.tool_calls; a user turn's tool_result
//     blocks become separate role:"tool" messages.
//   - Stop reasons are normalized to the engine's vocabulary
//     ("stop"→"end_turn", "tool_calls"→"tool_use", "length"→"max_tokens").
package openai

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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/subzone/m/internal/llm"
)

const defaultBaseURL = "https://api.openai.com"

func init() {
	llm.Register("openai", func() (llm.Provider, error) { return New() })
}

// Provider is an OpenAI Chat Completions client.
type Provider struct {
	apiKey   string
	baseURL  string
	client   *http.Client
	noStreamOpts bool
	noJsonSchema bool
	chatPath string // defaults to "/v1/chat/completions"
}

// Option configures a Provider.
type Option func(*Provider)

// WithAPIKey overrides the key picked up from OPENAI_API_KEY.
func WithAPIKey(k string) Option { return func(p *Provider) { p.apiKey = k } }

// WithBaseURL points the client at a custom origin (Azure, proxies, tests).
func WithBaseURL(u string) Option { return func(p *Provider) { p.baseURL = u } }

// WithChatPath overrides the chat completions path (default "/v1/chat/completions").
func WithChatPath(p string) Option { return func(pr *Provider) { pr.chatPath = p } }

// WithCompat marks this provider as targeting a non-OpenAI endpoint.
// Disables json_schema response_format (not supported by most compat endpoints).
// stream_options is kept enabled since most compat endpoints support it.
func WithCompat() Option { return func(p *Provider) { p.noJsonSchema = true } }

// WithNoStreamOptions disables stream_options (for endpoints like Gemini that reject it).
func WithNoStreamOptions() Option { return func(p *Provider) { p.noStreamOpts = true; p.noJsonSchema = true } }

// WithHTTPClient swaps the underlying http.Client.
func WithHTTPClient(c *http.Client) Option { return func(p *Provider) { p.client = c } }

// New constructs a Provider. OPENAI_API_KEY is required unless WithAPIKey
// is supplied. OPENAI_BASE_URL overrides the default origin.
func New(opts ...Option) (*Provider, error) {
	p := &Provider{
		apiKey:  os.Getenv("OPENAI_API_KEY"),
		baseURL: getenvOr("OPENAI_BASE_URL", defaultBaseURL),
		client:  http.DefaultClient,
	}
	for _, o := range opts {
		o(p)
	}
	if p.apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY is not set")
	}
	return p, nil
}

func getenvOr(k, dflt string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return dflt
}

type chatPayload struct {
	Model          string          `json:"model"`
	Messages       []chatMsg       `json:"messages"`
	Tools          []chatTool      `json:"tools,omitempty"`
	Temperature    *float64        `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Stream         bool            `json:"stream"`
	StreamOptions  *streamOptions  `json:"stream_options,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type responseFormat struct {
	Type       string          `json:"type"`
	JSONSchema *jsonSchemaSpec `json:"json_schema,omitempty"`
}

type jsonSchemaSpec struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type chatMsg struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type chatToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatTool struct {
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}

type chatToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Stream sends req to /v1/chat/completions with stream=true and forwards
// streamed events onto the returned channel.
func (p *Provider) Stream(ctx context.Context, req llm.Request) (<-chan llm.Event, error) {
	payload := chatPayload{
		Model:       req.Model,
		Messages:    buildMessages(req.System, req.Messages),
		Tools:       buildTools(req.Tools),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}
	if !p.noStreamOpts {
		payload.StreamOptions = &streamOptions{IncludeUsage: true}
	}
	if !p.noJsonSchema && len(req.ResponseSchema) > 0 {
		payload.ResponseFormat = &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchemaSpec{
				Name:   "response",
				Strict: true,
				Schema: req.ResponseSchema,
			},
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	chatPath := p.chatPath
	if chatPath == "" {
		chatPath = "/v1/chat/completions"
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+chatPath, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.doWithRetry(ctx, httpReq) //nolint:bodyclose // closed in goroutine below
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		body := strings.TrimSpace(string(slurp))
		if resp.StatusCode == 404 {
			return nil, fmt.Errorf("%s 404: model %q not found at %s (check model name and endpoint)", resp.Status, req.Model, p.baseURL)
		}
		return nil, fmt.Errorf("%s %s: %s", p.baseURL, resp.Status, body)
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

// doWithRetry executes the request with retry on 429 (rate limit).
func (p *Provider) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	const maxRetries = 3
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}

	backoff := time.Second
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}
		resp.Body.Close()
		if attempt == maxRetries {
			return nil, fmt.Errorf("openai rate limited (429) after %d retries — wait a moment and try again", maxRetries)
		}
		wait := backoff
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				wait = time.Duration(secs) * time.Second
			}
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		backoff *= 2
	}
	return nil, errors.New("unreachable")
}

// buildMessages translates llm.Messages into OpenAI's flat shape. The
// system prompt is hoisted into a leading system message.
func buildMessages(system string, msgs []llm.Message) []chatMsg {
	out := []chatMsg{}
	if system != "" {
		out = append(out, chatMsg{Role: "system", Content: system})
	}
	for _, m := range msgs {
		switch m.Role {
		case llm.RoleUser:
			var userText strings.Builder
			var toolMsgs []chatMsg
			for _, b := range m.Content {
				switch b.Type {
				case llm.BlockText:
					userText.WriteString(b.Text)
				case llm.BlockToolResult:
					toolMsgs = append(toolMsgs, chatMsg{
						Role:       "tool",
						Content:    b.Output,
						ToolCallID: b.ToolUseID,
					})
				}
			}
			if userText.Len() > 0 {
				out = append(out, chatMsg{Role: "user", Content: userText.String()})
			}
			out = append(out, toolMsgs...)
		case llm.RoleAssistant:
			var text strings.Builder
			var toolCalls []chatToolCall
			for _, b := range m.Content {
				switch b.Type {
				case llm.BlockText:
					text.WriteString(b.Text)
				case llm.BlockToolUse:
					args := string(b.ToolInput)
					if args == "" {
						args = "{}"
					}
					toolCalls = append(toolCalls, chatToolCall{
						ID:       b.ToolID,
						Type:     "function",
						Function: chatFunction{Name: b.ToolName, Arguments: args},
					})
				}
			}
			out = append(out, chatMsg{Role: "assistant", Content: text.String(), ToolCalls: toolCalls})
		}
	}
	return out
}

func buildTools(tools []llm.ToolSchema) []chatTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]chatTool, 0, len(tools))
	for _, t := range tools {
		params := t.InputSchema
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object"}`)
		}
		out = append(out, chatTool{
			Type: "function",
			Function: chatToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return out
}

// chunkChoice / chunkDelta / chunkToolCall mirror the streaming response.
type chunkChoice struct {
	Delta        chunkDelta      `json:"delta"`
	FinishReason string          `json:"finish_reason"`
}

type chunkDelta struct {
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	ToolCalls []chunkToolCall `json:"tool_calls"`
}

type chunkToolCall struct {
	Index    int           `json:"index"`
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Function chunkFunction `json:"function"`
}

type chunkFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// toolCallState accumulates a tool call across delta chunks.
type toolCallState struct {
	id   string
	name string
	args strings.Builder
}

// parseSSE reads OpenAI's text/event-stream response and forwards relevant
// events. Each `data: {...}` line is a self-contained chunk; `data: [DONE]`
// is the stream terminator.
func parseSSE(ctx context.Context, r io.Reader, out chan<- llm.Event) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	send := func(ev llm.Event) error {
		select {
		case out <- ev:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	pending := map[int]*toolCallState{}

	flushToolCalls := func() error {
		idxs := make([]int, 0, len(pending))
		for i := range pending {
			idxs = append(idxs, i)
		}
		sort.Ints(idxs)
		for _, i := range idxs {
			tc := pending[i]
			args := tc.args.String()
			if args == "" {
				args = "{}"
			}
			if err := send(llm.Event{
				Kind:      llm.EventToolCall,
				ToolID:    tc.id,
				ToolName:  tc.name,
				ToolInput: json.RawMessage(args),
			}); err != nil {
				return err
			}
		}
		pending = map[int]*toolCallState{}
		return nil
	}

	var data strings.Builder

	dispatch := func() error {
		if data.Len() == 0 {
			return nil
		}
		raw := data.String()
		data.Reset()
		if raw == "[DONE]" {
			return nil
		}

		var chunk struct {
			Choices []chunkChoice `json:"choices"`
			Usage   *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
			Error *struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
			return fmt.Errorf("parse chunk: %w", err)
		}
		if chunk.Error != nil {
			return fmt.Errorf("openai %s: %s", chunk.Error.Type, chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			// Usage-only chunk (no choices) — emit usage and return.
			if chunk.Usage != nil && (chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0) {
				if err := send(llm.Event{Kind: llm.EventUsage, Usage: llm.Usage{
					InputTokens:  chunk.Usage.PromptTokens,
					OutputTokens: chunk.Usage.CompletionTokens,
				}}); err != nil {
					return err
				}
			}
			return nil
		}
		c := chunk.Choices[0]

		if c.Delta.Content != "" {
			if err := send(llm.Event{Kind: llm.EventText, Text: c.Delta.Content}); err != nil {
				return err
			}
		}
		for _, tc := range c.Delta.ToolCalls {
			st := pending[tc.Index]
			if st == nil {
				st = &toolCallState{}
				pending[tc.Index] = st
			}
			if tc.ID != "" {
				st.id = tc.ID
			}
			if tc.Function.Name != "" {
				st.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				st.args.WriteString(tc.Function.Arguments)
			}
		}
		if c.FinishReason != "" {
			if err := flushToolCalls(); err != nil {
				return err
			}
			if chunk.Usage != nil && (chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0) {
				if err := send(llm.Event{Kind: llm.EventUsage, Usage: llm.Usage{
					InputTokens:  chunk.Usage.PromptTokens,
					OutputTokens: chunk.Usage.CompletionTokens,
				}}); err != nil {
					return err
				}
			}
			if err := send(llm.Event{
				Kind:       llm.EventDone,
				StopReason: normalizeStop(c.FinishReason),
			}); err != nil {
				return err
			}
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
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		v := strings.TrimPrefix(line, "data:")
		v = strings.TrimPrefix(v, " ")
		if data.Len() > 0 {
			data.WriteByte('\n')
		}
		data.WriteString(v)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return dispatch()
}

func normalizeStop(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "tool_calls", "function_call":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		return reason
	}
}
