package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// fakeServer drives the server side of an MCP stdio session over the
// supplied reader/writer. It stops when the reader closes.
type fakeServer struct {
	tools     []Tool
	callError bool
	callOut   string
	callIsErr bool

	gotInitialized bool
	calls          []toolsCallParams

	t *testing.T
}

func newFakeServer(t *testing.T, tools []Tool) *fakeServer {
	return &fakeServer{tools: tools, t: t}
}

func (s *fakeServer) serve(r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		var probe rpcRequest
		if err := json.Unmarshal(line, &probe); err != nil {
			s.t.Errorf("server: bad json: %v line=%s", err, line)
			continue
		}
		switch probe.Method {
		case "initialize":
			s.respond(w, probe.ID, map[string]any{
				"protocolVersion": ProtocolVersion,
				"serverInfo":      map[string]any{"name": "fake", "version": "0"},
				"capabilities":    map[string]any{"tools": map[string]any{}},
			})
		case "notifications/initialized":
			s.gotInitialized = true
		case "tools/list":
			s.respond(w, probe.ID, toolsListResult{Tools: s.tools})
		case "tools/call":
			var params toolsCallParams
			_ = json.Unmarshal(probe.Params, &params)
			s.calls = append(s.calls, params)
			if s.callError {
				s.respondErr(w, probe.ID, -32000, "boom")
				continue
			}
			s.respond(w, probe.ID, toolsCallResult{
				Content: []contentItem{{Type: "text", Text: s.callOut}},
				IsError: s.callIsErr,
			})
		default:
			s.respondErr(w, probe.ID, -32601, "method not found: "+probe.Method)
		}
	}
}

func (s *fakeServer) respond(w io.Writer, id json.RawMessage, result any) {
	r, err := json.Marshal(result)
	if err != nil {
		s.t.Fatalf("server: marshal result: %v", err)
	}
	out, err := json.Marshal(rpcResponse{JSONRPC: JSONRPCVersion, ID: id, Result: r})
	if err != nil {
		s.t.Fatalf("server: marshal response: %v", err)
	}
	_, _ = w.Write(append(out, '\n'))
}

func (s *fakeServer) respondErr(w io.Writer, id json.RawMessage, code int, msg string) {
	out, _ := json.Marshal(rpcResponse{
		JSONRPC: JSONRPCVersion, ID: id,
		Error: &RPCError{Code: code, Message: msg},
	})
	_, _ = w.Write(append(out, '\n'))
}

// pipePair returns connected client/server pipes: clientWrites→serverReads,
// serverWrites→clientReads.
func pipePair() (clientIn io.WriteCloser, clientOut io.ReadCloser, serverIn io.Reader, serverOut io.WriteCloser) {
	cR, cW := io.Pipe() // server reads from cR; client writes to cW
	sR, sW := io.Pipe() // client reads from sR; server writes to sW
	return cW, sR, cR, sW
}

func TestClientHandshakeAndListTools(t *testing.T) {
	clientIn, clientOut, serverIn, serverOut := pipePair()
	srv := newFakeServer(t, []Tool{
		{Name: "create_issue", Description: "open an issue", InputSchema: json.RawMessage(`{"type":"object"}`)},
		{Name: "list_repos", InputSchema: json.RawMessage(`{"type":"object"}`)},
	})
	go srv.serve(serverIn, serverOut)

	c := NewPipeClient(clientIn, clientOut)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := c.Initialize(ctx, "agent", "test"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	tools, err := c.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 2 || tools[0].Name != "create_issue" || tools[1].Name != "list_repos" {
		t.Errorf("tools = %+v", tools)
	}

	_ = c.Close()
	// Drain server.
	time.Sleep(20 * time.Millisecond)
	if !srv.gotInitialized {
		t.Error("server never received notifications/initialized")
	}
}

func TestClientCallToolText(t *testing.T) {
	clientIn, clientOut, serverIn, serverOut := pipePair()
	srv := newFakeServer(t, nil)
	srv.callOut = "result-body"
	go srv.serve(serverIn, serverOut)

	c := NewPipeClient(clientIn, clientOut)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Initialize(ctx, "agent", "test"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	out, isErr, err := c.CallTool(ctx, "do", json.RawMessage(`{"x":1}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if isErr {
		t.Error("isError = true")
	}
	if out != "result-body" {
		t.Errorf("out = %q", out)
	}
	if len(srv.calls) != 1 || srv.calls[0].Name != "do" {
		t.Errorf("server calls = %+v", srv.calls)
	}
	if string(srv.calls[0].Arguments) != `{"x":1}` {
		t.Errorf("server saw args = %s", srv.calls[0].Arguments)
	}
	_ = c.Close()
}

func TestClientCallToolErrorResponse(t *testing.T) {
	clientIn, clientOut, serverIn, serverOut := pipePair()
	srv := newFakeServer(t, nil)
	srv.callError = true
	go srv.serve(serverIn, serverOut)

	c := NewPipeClient(clientIn, clientOut)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.Initialize(ctx, "agent", "test")
	_, _, err := c.CallTool(ctx, "do", nil)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("got %v", err)
	}
	_ = c.Close()
}

func TestClientCallToolIsErrorResult(t *testing.T) {
	clientIn, clientOut, serverIn, serverOut := pipePair()
	srv := newFakeServer(t, nil)
	srv.callOut = "tool-said-no"
	srv.callIsErr = true
	go srv.serve(serverIn, serverOut)

	c := NewPipeClient(clientIn, clientOut)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.Initialize(ctx, "agent", "test")
	out, isErr, err := c.CallTool(ctx, "do", nil)
	if err != nil {
		t.Fatalf("CallTool returned err for isError=true result: %v", err)
	}
	if !isErr {
		t.Error("expected isError=true")
	}
	if out != "tool-said-no" {
		t.Errorf("out = %q", out)
	}
	_ = c.Close()
}

func TestClientConcurrentCalls(t *testing.T) {
	// Server intentionally responds in reverse order to ensure id correlation
	// is the only thing the client relies on.
	cR, cW := io.Pipe()
	sR, sW := io.Pipe()

	go func() {
		defer sW.Close()
		scanner := bufio.NewScanner(cR)
		var pending []rpcRequest
		for scanner.Scan() {
			var r rpcRequest
			if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
				continue
			}
			if r.Method == "notifications/initialized" {
				continue
			}
			pending = append(pending, r)
			if r.Method == "initialize" {
				out, _ := json.Marshal(rpcResponse{
					JSONRPC: JSONRPCVersion, ID: r.ID,
					Result: json.RawMessage(`{}`),
				})
				sW.Write(append(out, '\n'))
				pending = pending[:0]
				continue
			}
			if len(pending) == 2 {
				// Reply in reverse order.
				for i := len(pending) - 1; i >= 0; i-- {
					p := pending[i]
					body, _ := json.Marshal(toolsCallResult{
						Content: []contentItem{{Type: "text", Text: fmt.Sprintf("got id %s", p.ID)}},
					})
					out, _ := json.Marshal(rpcResponse{
						JSONRPC: JSONRPCVersion, ID: p.ID, Result: body,
					})
					sW.Write(append(out, '\n'))
				}
				pending = pending[:0]
			}
		}
	}()

	c := NewPipeClient(cW, sR)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Initialize(ctx, "a", "0"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	type result struct {
		out string
		err error
	}
	results := make(chan result, 2)
	go func() {
		out, _, err := c.CallTool(ctx, "a", nil)
		results <- result{out, err}
	}()
	go func() {
		out, _, err := c.CallTool(ctx, "b", nil)
		results <- result{out, err}
	}()
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil {
			t.Errorf("call %d err: %v", i, r.err)
		}
		if !strings.HasPrefix(r.out, "got id") {
			t.Errorf("call %d out = %q", i, r.out)
		}
	}
	_ = c.Close()
}

func TestClientContextCancel(t *testing.T) {
	// Server never replies to tools/call.
	cR, cW := io.Pipe()
	sR, sW := io.Pipe()
	go func() {
		scanner := bufio.NewScanner(cR)
		for scanner.Scan() {
			var r rpcRequest
			if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
				continue
			}
			if r.Method == "initialize" {
				out, _ := json.Marshal(rpcResponse{JSONRPC: JSONRPCVersion, ID: r.ID, Result: json.RawMessage(`{}`)})
				sW.Write(append(out, '\n'))
			}
			// drop everything else
		}
	}()

	c := NewPipeClient(cW, sR)
	ctx0, cancel0 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel0()
	if err := c.Initialize(ctx0, "a", "0"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _, err := c.CallTool(ctx, "do", nil)
	if err == nil {
		t.Fatal("expected ctx.Err()")
	}
	_ = c.Close()
}

// TestProcessClientSpawnsBinary checks the exec.Cmd-based path. We re-exec
// this test binary in "fixture" mode (controlled by TEST_MCP_FIXTURE=1) and
// have it act as a minimal MCP server.
func TestProcessClientSpawnsBinary(t *testing.T) {
	exe, err := osExecutable()
	if err != nil {
		t.Skipf("executable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := NewProcessClient(ctx, []string{exe, "-test.run=TestHelperMCPFixture"}, map[string]string{"TEST_MCP_FIXTURE": "1"})
	if err != nil {
		t.Fatalf("NewProcessClient: %v", err)
	}
	defer c.Close()

	if err := c.Initialize(ctx, "agent", "test"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	tools, err := c.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Errorf("tools = %+v", tools)
	}
	out, isErr, err := c.CallTool(ctx, "echo", json.RawMessage(`{"text":"hi"}`))
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if isErr {
		t.Error("isError = true")
	}
	if out != "echoed: hi" {
		t.Errorf("out = %q", out)
	}
}
