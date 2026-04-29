package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Client is a JSON-RPC peer talking to an MCP server over a pair of
// io streams. Stdio framing is one JSON object per line.
//
// Construction: NewProcessClient (spawns a server) or NewPipeClient
// (tests / custom transports). Initialize must run before tools/list or
// tools/call.
type Client struct {
	in  io.WriteCloser
	out io.ReadCloser
	cmd *exec.Cmd // nil for pipe-based clients

	writeMu sync.Mutex // serializes writes onto in

	nextID     atomic.Int64
	pendingMu  sync.Mutex
	pending    map[int64]chan *rpcResponse
	closed     chan struct{}
	closedOnce sync.Once
	readErr    error
}

// NewProcessClient launches the given command with the supplied env and
// starts the read loop. Caller must call Close to terminate the subprocess.
//
// env entries are appended to the parent environment so the child still
// sees PATH etc.; pass only the variables you want to add or override.
func NewProcessClient(ctx context.Context, command []string, env map[string]string) (*Client, error) {
	if len(command) == 0 {
		return nil, errors.New("mcp: command is empty")
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Env = append(os.Environ(), formatEnv(env)...)
	cmd.Stderr = os.Stderr // surface server logs to the user

	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp: start %v: %w", command, err)
	}
	return startClient(in, out, cmd), nil
}

// NewPipeClient wires a Client to the given streams. Used by tests.
func NewPipeClient(in io.WriteCloser, out io.ReadCloser) *Client {
	return startClient(in, out, nil)
}

func startClient(in io.WriteCloser, out io.ReadCloser, cmd *exec.Cmd) *Client {
	c := &Client{
		in:      in,
		out:     out,
		cmd:     cmd,
		pending: make(map[int64]chan *rpcResponse),
		closed:  make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// readLoop dispatches inbound responses to waiters. Notifications and
// server-initiated requests are ignored — this client is a tools-only
// consumer for now.
func (c *Client) readLoop() {
	defer c.markClosed(nil)

	scanner := bufio.NewScanner(c.out)
	scanner.Buffer(make([]byte, 0, 64*1024), 8<<20)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var resp rpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			// Could be a notification or a server→client request. Drop.
			continue
		}
		if len(resp.ID) == 0 || string(resp.ID) == "null" {
			continue // notification, no id
		}
		var id int64
		if err := json.Unmarshal(resp.ID, &id); err != nil {
			continue // non-integer ids aren't ours
		}
		c.pendingMu.Lock()
		ch, ok := c.pending[id]
		if ok {
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()
		if ok {
			ch <- &resp
		}
	}
	c.readErr = scanner.Err()
}

func (c *Client) markClosed(err error) {
	c.closedOnce.Do(func() {
		if err != nil && c.readErr == nil {
			c.readErr = err
		}
		close(c.closed)
		// Wake any waiters so they don't hang.
		c.pendingMu.Lock()
		for id, ch := range c.pending {
			delete(c.pending, id)
			close(ch)
		}
		c.pendingMu.Unlock()
	})
}

// call sends a request and waits for the matched response. Cancellation
// via ctx unblocks the waiter but does not abort the request server-side.
func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)

	idRaw, err := json.Marshal(id)
	if err != nil {
		return nil, err
	}
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
	body = append(body, '\n')

	ch := make(chan *rpcResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	if err := c.writeRaw(body); err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, err
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, c.closedErr()
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("mcp %s: %s (code %d)", method, resp.Error.Message, resp.Error.Code)
		}
		return resp.Result, nil
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-c.closed:
		return nil, c.closedErr()
	}
}

// notify sends a one-way JSON-RPC notification.
func (c *Client) notify(method string, params any) error {
	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		paramsRaw = b
	}
	body, err := json.Marshal(rpcNotification{
		JSONRPC: JSONRPCVersion, Method: method, Params: paramsRaw,
	})
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return c.writeRaw(body)
}

func (c *Client) writeRaw(b []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := c.in.Write(b); err != nil {
		return fmt.Errorf("mcp: write: %w", err)
	}
	return nil
}

func (c *Client) closedErr() error {
	if c.readErr != nil {
		return fmt.Errorf("mcp: connection closed: %w", c.readErr)
	}
	return errors.New("mcp: connection closed")
}

// Initialize performs the MCP handshake: sends initialize, then the
// notifications/initialized notification on success.
func (c *Client) Initialize(ctx context.Context, clientName, clientVersion string) error {
	params := map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": clientName, "version": clientVersion},
	}
	if _, err := c.call(ctx, "initialize", params); err != nil {
		return err
	}
	return c.notify("notifications/initialized", nil)
}

// ListTools returns the server's currently published tool listing.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	raw, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}
	var result toolsListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("mcp: parse tools/list: %w", err)
	}
	return result.Tools, nil
}

// CallTool invokes name on the server with the given JSON-encoded args.
// Returns the concatenated text content, the isError flag from the
// server, and a transport-level error if any.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (string, bool, error) {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	raw, err := c.call(ctx, "tools/call", toolsCallParams{Name: name, Arguments: args})
	if err != nil {
		return "", false, err
	}
	var result toolsCallResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", false, fmt.Errorf("mcp: parse tools/call: %w", err)
	}
	var out strings.Builder
	for _, item := range result.Content {
		if item.Type == "text" {
			out.WriteString(item.Text)
		}
	}
	return out.String(), result.IsError, nil
}

// Close closes the writer side, waits briefly for the subprocess (if any)
// to exit, and force-kills it on timeout.
func (c *Client) Close() error {
	_ = c.in.Close()
	if c.cmd == nil {
		_ = c.out.Close()
		c.markClosed(nil)
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- c.cmd.Wait() }()
	select {
	case err := <-done:
		c.markClosed(err)
		return err
	case <-time.After(2 * time.Second):
		_ = c.cmd.Process.Kill()
		<-done
		c.markClosed(errors.New("mcp: server did not exit cleanly; killed"))
		return c.readErr
	}
}

func formatEnv(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}
