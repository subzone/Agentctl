package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/subzone/Agentctl/internal/config"
	"github.com/subzone/Agentctl/internal/mcp"
	"github.com/subzone/Agentctl/internal/tools"
)

// TestResult is the outcome of running a custom shell tool from the test bench.
type TestResult struct {
	OK         bool   `json:"ok"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs"`
}

// MCPTestResult reports whether an MCP server starts and which tools it exposes.
type MCPTestResult struct {
	OK    bool     `json:"ok"`
	Tools []string `json:"tools"`
	Log   string   `json:"log"`
	Error string   `json:"error,omitempty"`
}

// TestTool executes a user-authored shell tool with the given JSON stdin input.
// name must match a saved tool in ~/.config/m/tools.
func (a *App) TestTool(name, inputJSON string) TestResult {
	start := time.Now()
	name = strings.TrimSpace(name)
	if name == "" {
		return TestResult{Error: "tool name is required"}
	}
	p := filepath.Join(userToolsDir(), name+".md")
	doc, err := config.ParseFile(p)
	if err != nil {
		return TestResult{Error: fmt.Sprintf("load: %v", err)}
	}
	spec, ok := doc.Spec.(*config.ToolSpec)
	if !ok {
		return TestResult{Error: "not a tool document"}
	}
	if spec.Runtime != config.RuntimeShell {
		return TestResult{Error: "only shell tools can be tested from the bench"}
	}
	desc := spec.Description
	if body := strings.TrimSpace(doc.Body); body != "" {
		if desc != "" {
			desc += "\n\n"
		}
		desc += body
	}
	ct := tools.NewCommandTool(
		spec.Name, desc, spec.Command, spec.Env,
		spec.ParametersJSON(), time.Duration(spec.TimeoutSec)*time.Second,
	)
	var input json.RawMessage
	if strings.TrimSpace(inputJSON) == "" {
		input = json.RawMessage(`{}`)
	} else if !json.Valid([]byte(inputJSON)) {
		return TestResult{Error: "input is not valid JSON"}
	} else {
		input = json.RawMessage(inputJSON)
	}
	ctx, cancel := context.WithTimeout(a.ctx, 90*time.Second)
	defer cancel()
	out, err := ct.Run(ctx, input)
	dur := time.Since(start).Milliseconds()
	if err != nil {
		return TestResult{OK: false, Output: out, Error: err.Error(), DurationMs: dur}
	}
	return TestResult{OK: true, Output: out, DurationMs: dur}
}

// TestMCP tries to start a configured MCP server and list its tools.
func (a *App) TestMCP(name string) MCPTestResult {
	name = strings.TrimSpace(name)
	if name == "" {
		return MCPTestResult{Error: "server name is required"}
	}
	p := filepath.Join(userMCPDir(), name+".md")
	doc, err := config.ParseFile(p)
	if err != nil {
		return MCPTestResult{Error: fmt.Sprintf("load: %v", err)}
	}
	spec, ok := doc.Spec.(*config.MCPServerSpec)
	if !ok {
		return MCPTestResult{Error: "not an mcp_server document"}
	}
	var logBuf bytes.Buffer
	ctx, cancel := context.WithTimeout(a.ctx, 45*time.Second)
	defer cancel()
	mgr, err := mcp.Open(ctx, []*config.MCPServerSpec{spec}, "test-bench", &logBuf)
	if mgr != nil {
		defer mgr.Close()
	}
	if err != nil {
		return MCPTestResult{Error: err.Error(), Log: logBuf.String()}
	}
	reg := mgr.Registry()
	var names []string
	for _, t := range reg.All() {
		names = append(names, t.Name())
	}
	if len(names) == 0 {
		return MCPTestResult{
			OK:    false,
			Log:   logBuf.String(),
			Error: "server started but exposed no tools",
		}
	}
	return MCPTestResult{OK: true, Tools: names, Log: logBuf.String()}
}
