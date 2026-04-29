package mcp

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

// osExecutable returns the path to the running test binary so we can re-exec
// ourselves as an MCP server fixture. Wrapped behind a function so a future
// caller can stub it out if needed.
func osExecutable() (string, error) {
	return os.Executable()
}

// TestHelperMCPFixture is a re-entry point: when TEST_MCP_FIXTURE=1, the
// process behaves as a minimal MCP server speaking on stdin/stdout instead
// of running tests. The supervising test in client_test.go exec's this
// binary with that env var set to exercise the spawn path end-to-end.
func TestHelperMCPFixture(t *testing.T) {
	if os.Getenv("TEST_MCP_FIXTURE") != "1" {
		t.Skip("not the MCP server fixture (set TEST_MCP_FIXTURE=1)")
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	respond := func(id json.RawMessage, result any) {
		body, _ := json.Marshal(result)
		out, _ := json.Marshal(rpcResponse{JSONRPC: JSONRPCVersion, ID: id, Result: body})
		os.Stdout.Write(append(out, '\n'))
	}

	for scanner.Scan() {
		var r rpcRequest
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue
		}
		switch r.Method {
		case "initialize":
			respond(r.ID, map[string]any{
				"protocolVersion": ProtocolVersion,
				"serverInfo":      map[string]any{"name": "fixture", "version": "0"},
				"capabilities":    map[string]any{"tools": map[string]any{}},
			})
		case "notifications/initialized":
			// no response
		case "tools/list":
			respond(r.ID, toolsListResult{Tools: []Tool{
				{Name: "echo", Description: "echo input", InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`)},
			}})
		case "tools/call":
			var p toolsCallParams
			_ = json.Unmarshal(r.Params, &p)
			var args struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(p.Arguments, &args)
			respond(r.ID, toolsCallResult{
				Content: []contentItem{{Type: "text", Text: "echoed: " + args.Text}},
			})
		}
	}
}
