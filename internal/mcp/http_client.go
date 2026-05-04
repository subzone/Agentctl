package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Transport is the interface both stdio and HTTP/SSE MCP clients implement.
type Transport interface {
	Initialize(ctx context.Context, clientName, clientVersion string) error
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, args json.RawMessage) (string, bool, error)
	Close() error
}

// HTTPClient talks to an MCP server over HTTP. Supports both plain HTTP
// (POST JSON-RPC, get JSON-RPC response) and SSE (POST, receive events).
type HTTPClient struct {
	baseURL string
	client  *http.Client
	nextID  atomic.Int64
	useSSE  bool
}

// NewHTTPClient creates an HTTP/SSE MCP client.
func NewHTTPClient(baseURL string, sse bool) *HTTPClient {
	return &HTTPClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{Timeout: 60 * time.Second},
		useSSE:  sse,
	}
}

func (h *HTTPClient) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := h.nextID.Add(1)
	idRaw, _ := json.Marshal(id)

	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		paramsRaw = b
	}

	body, err := json.Marshal(rpcRequest{
		JSONRPC: JSONRPCVersion,
		ID:      idRaw,
		Method:  method,
		Params:  paramsRaw,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.useSSE {
		req.Header.Set("Accept", "text/event-stream")
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("mcp http: %s %s", resp.Status, string(slurp))
	}

	if h.useSSE {
		return h.parseSSEResponse(resp.Body)
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("mcp http: decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp %s: %s (code %d)", method, rpcResp.Error.Message, rpcResp.Error.Code)
	}
	return rpcResp.Result, nil
}

// parseSSEResponse reads an SSE stream and extracts the JSON-RPC response
// from the first "data:" event that contains a result or error.
func (h *HTTPClient) parseSSEResponse(r io.Reader) (json.RawMessage, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var rpcResp rpcResponse
		if err := json.Unmarshal([]byte(data), &rpcResp); err != nil {
			continue // skip non-JSON-RPC events
		}
		if rpcResp.Error != nil {
			return nil, fmt.Errorf("mcp sse: %s (code %d)", rpcResp.Error.Message, rpcResp.Error.Code)
		}
		if len(rpcResp.Result) > 0 {
			return rpcResp.Result, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("mcp sse: read: %w", err)
	}
	return nil, fmt.Errorf("mcp sse: no result in event stream")
}

func (h *HTTPClient) Initialize(ctx context.Context, clientName, clientVersion string) error {
	params := map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": clientName, "version": clientVersion},
	}
	_, err := h.call(ctx, "initialize", params)
	return err
}

func (h *HTTPClient) ListTools(ctx context.Context) ([]Tool, error) {
	raw, err := h.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}
	var result toolsListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("mcp http: parse tools/list: %w", err)
	}
	return result.Tools, nil
}

func (h *HTTPClient) CallTool(ctx context.Context, name string, args json.RawMessage) (string, bool, error) {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	raw, err := h.call(ctx, "tools/call", toolsCallParams{Name: name, Arguments: args})
	if err != nil {
		return "", false, err
	}
	var result toolsCallResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", false, fmt.Errorf("mcp http: parse tools/call: %w", err)
	}
	var out strings.Builder
	for _, item := range result.Content {
		if item.Type == "text" {
			out.WriteString(item.Text)
		}
	}
	return out.String(), result.IsError, nil
}

func (h *HTTPClient) Close() error { return nil }
