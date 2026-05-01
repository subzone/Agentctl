// Package ollama implements the llm.Provider contract against the Ollama
// /api/chat endpoint. Stdlib-only.
//
// Ollama uses NDJSON streaming (one JSON object per line) rather than SSE.
// Tool support requires a model that advertises function calling and
// Ollama 0.4+. Tool calls don't carry server-assigned IDs, so we mint
// synthetic ones for the engine; on the wire to Ollama, tool_result
// messages are sent without IDs.
package ollama

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

	"github.com/subzone/m/internal/llm"
)

const defaultBaseURL = "http://localhost:11434"

func init() {
	llm.Register("ollama", func() (llm.Provider, error) { return New() })
}

// Provider is an Ollama /api/chat client.
type Provider struct {
	baseURL string
	client  *http.Client
}

// Option configures a Provider.
type Option func(*Provider)

// WithBaseURL points the client at a non-default Ollama host.
func WithBaseURL(u string) Option { return func(p *Provider) { p.baseURL = u } }

// WithHTTPClient swaps the underlying http.Client.
func WithHTTPClient(c *http.Client) Option { return func(p *Provider) { p.client = c } }

// New constructs a Provider. OLLAMA_HOST overrides the default origin
// (http://localhost:11434).
func New(opts ...Option) (*Provider, error) {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = defaultBaseURL
	} else if !strings.Contains(host, "://") {
		// Be friendly: Ollama's own conventions accept "localhost:11434".
		host = "http://" + host
	}
	p := &Provider{baseURL: host, client: http.DefaultClient}
	for _, o := range opts {
		o(p)
	}
	return p, nil
}

type chatPayload struct {
	Model    string     `json:"model"`
	Messages []chatMsg  `json:"messages"`
	Tools    []chatTool `json:"tools,omitempty"`
	Stream   bool       `json:"stream"`
	Options  *chatOpts  `json:"options,omitempty"`
}

type chatOpts struct {
	Temperature *float64 `json:"temperature,omitempty"`
	NumPredict  *int     `json:"num_predict,omitempty"`
}

type chatMsg struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []chatToolCall `json:"tool_calls,omitempty"`
}

type chatToolCall struct {
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
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

// Stream sends req to /api/chat with stream=true and forwards events onto
// the returned channel.
func (p *Provider) Stream(ctx context.Context, req llm.Request) (<-chan llm.Event, error) {
	payload := chatPayload{
		Model:    req.Model,
		Messages: buildMessages(req.System, req.Messages),
		Tools:    buildTools(req.Tools),
		Stream:   true,
	}
	if req.Temperature != nil || req.MaxTokens > 0 {
		opts := &chatOpts{}
		if req.Temperature != nil {
			opts.Temperature = req.Temperature
		}
		if req.MaxTokens > 0 {
			mt := req.MaxTokens
			opts.NumPredict = &mt
		}
		payload.Options = opts
	} else {
		// When MaxTokens is 0, set a generous default so Ollama doesn't
		// use its built-in limit (often 128-2048 tokens).
		defaultPredict := 8192
		payload.Options = &chatOpts{NumPredict: &defaultPredict}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq) //nolint:bodyclose // closed in goroutine below
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("ollama %s: %s", resp.Status, strings.TrimSpace(string(slurp)))
	}

	out := make(chan llm.Event, 8)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		if err := parseStream(ctx, resp.Body, out); err != nil {
			select {
			case out <- llm.Event{Kind: llm.EventError, Err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return out, nil
}

// buildMessages flattens content blocks into Ollama's message shape.
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
					toolMsgs = append(toolMsgs, chatMsg{Role: "tool", Content: b.Output})
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
					args := b.ToolInput
					if len(args) == 0 {
						args = json.RawMessage(`{}`)
					}
					toolCalls = append(toolCalls, chatToolCall{
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
				Name: t.Name, Description: t.Description, Parameters: params,
			},
		})
	}
	return out
}

// streamChunk mirrors a single NDJSON line from /api/chat.
type streamChunk struct {
	Message struct {
		Role      string `json:"role"`
		Content   string `json:"content"`
		ToolCalls []struct {
			Function struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"message"`
	Done            bool   `json:"done"`
	DoneReason      string `json:"done_reason"`
	Error           string `json:"error"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
}

// parseStream reads NDJSON chunks and forwards relevant events.
func parseStream(ctx context.Context, r io.Reader, out chan<- llm.Event) error {
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

	var (
		hadToolCall bool
		toolCounter int
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return fmt.Errorf("parse ollama chunk: %w", err)
		}
		if chunk.Error != "" {
			return errors.New("ollama: " + chunk.Error)
		}
		if chunk.Message.Content != "" {
			if err := send(llm.Event{Kind: llm.EventText, Text: chunk.Message.Content}); err != nil {
				return err
			}
		}
		for _, tc := range chunk.Message.ToolCalls {
			hadToolCall = true
			toolCounter++
			args := tc.Function.Arguments
			if len(args) == 0 {
				args = json.RawMessage(`{}`)
			}
			if err := send(llm.Event{
				Kind:      llm.EventToolCall,
				ToolID:    fmt.Sprintf("ollama-tool-%d", toolCounter),
				ToolName:  tc.Function.Name,
				ToolInput: args,
			}); err != nil {
				return err
			}
		}
		if chunk.Done {
			stop := normalizeStop(chunk.DoneReason)
			if hadToolCall {
				stop = "tool_use"
			}
			if chunk.PromptEvalCount > 0 || chunk.EvalCount > 0 {
				if err := send(llm.Event{Kind: llm.EventUsage, Usage: llm.Usage{
					InputTokens:  chunk.PromptEvalCount,
					OutputTokens: chunk.EvalCount,
				}}); err != nil {
					return err
				}
			}
			return send(llm.Event{Kind: llm.EventDone, StopReason: stop})
		}
	}
	return scanner.Err()
}

func normalizeStop(reason string) string {
	switch reason {
	case "stop", "":
		return "end_turn"
	case "length":
		return "max_tokens"
	default:
		return reason
	}
}
