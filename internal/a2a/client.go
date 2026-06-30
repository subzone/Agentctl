package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client talks to a remote A2A agent.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient builds a client for the agent rooted at baseURL
// (e.g. "http://localhost:8080"). Trailing slashes are trimmed.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 5 * time.Minute},
	}
}

// Card fetches the remote agent's Agent Card.
func (c *Client) Card(ctx context.Context) (AgentCard, error) {
	var card AgentCard
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/.well-known/agent.json", nil)
	if err != nil {
		return card, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return card, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return card, fmt.Errorf("agent card: unexpected status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return card, fmt.Errorf("decode agent card: %w", err)
	}
	return card, nil
}

// Send delivers a text task via message/send and returns the agent's reply
// text. A JSON-RPC error is surfaced as a Go error.
func (c *Client) Send(ctx context.Context, text string) (string, error) {
	params, err := json.Marshal(sendParams{
		Message: TextMessage("user", fmt.Sprintf("msg_%d", time.Now().UnixNano()), text),
	})
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "message/send",
		Params:  params,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result Message   `json:"result"`
		Error  *rpcError `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return "", fmt.Errorf("remote agent error (%d): %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result.Text(), nil
}
