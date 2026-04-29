package mcp

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/milenkom81/m/internal/config"
)

func TestResolveAndMissing(t *testing.T) {
	docs := []*config.Document{
		{Path: "a.md", Spec: &config.MCPServerSpec{Meta: config.Meta{Name: "github", Type: config.TypeMCPServer}}},
		{Path: "b.md", Spec: &config.MCPServerSpec{Meta: config.Meta{Name: "jira", Type: config.TypeMCPServer}}},
		{Path: "c.md", Spec: &config.AgentSpec{Meta: config.Meta{Name: "ignored", Type: config.TypeAgent}}},
	}
	specs, missing := Resolve(docs, []string{"github", "slack", "jira"})
	if len(specs) != 2 || specs[0].Name != "github" || specs[1].Name != "jira" {
		t.Errorf("specs = %+v", specs)
	}
	if len(missing) != 1 || missing[0] != "slack" {
		t.Errorf("missing = %v", missing)
	}
}

func TestIsAllowedFor(t *testing.T) {
	open := &config.MCPServerSpec{}
	if !isAllowedFor(open, "anything") {
		t.Error("empty allowed_agents should permit anyone")
	}
	if !isAllowedFor(open, "") {
		t.Error("empty allowed_agents + empty agent should permit")
	}
	scoped := &config.MCPServerSpec{AllowedAgents: []string{"coder"}}
	if !isAllowedFor(scoped, "coder") {
		t.Error("listed agent should be allowed")
	}
	if isAllowedFor(scoped, "intruder") {
		t.Error("unlisted agent should be denied")
	}
	// Empty agent name bypasses (e.g. ad-hoc invocations) — documented behavior.
	if !isAllowedFor(scoped, "") {
		t.Error("empty agent name should pass through (bypass)")
	}
}

// TestManagerOpenSkipsUnsupportedTransport spawns no clients but checks
// that an HTTP/SSE-only server is logged-and-skipped rather than fatal.
func TestManagerOpenSkipsUnsupportedTransport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	specs := []*config.MCPServerSpec{{
		Meta:      config.Meta{Name: "remote", Type: config.TypeMCPServer},
		Transport: config.TransportHTTP,
		URL:       "https://example.invalid",
	}}
	var buf bytes.Buffer
	mgr, err := Open(ctx, specs, "anyone", &buf)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer mgr.Close()
	if got := mgr.Registry().All(); len(got) != 0 {
		t.Errorf("expected no tools, got %d", len(got))
	}
	if !bytes.Contains(buf.Bytes(), []byte("transport \"http\" not yet supported")) {
		t.Errorf("missing skip log; got: %s", buf.String())
	}
}

// TestManagerOpenSkipsDisallowedAgent verifies allowed_agents enforcement.
func TestManagerOpenSkipsDisallowedAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	specs := []*config.MCPServerSpec{{
		Meta:          config.Meta{Name: "github", Type: config.TypeMCPServer},
		Transport:     config.TransportStdio,
		Command:       []string{"/bin/false"}, // never gets invoked
		AllowedAgents: []string{"coder"},
	}}
	var buf bytes.Buffer
	mgr, err := Open(ctx, specs, "intruder", &buf)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer mgr.Close()
	if got := mgr.Registry().All(); len(got) != 0 {
		t.Errorf("expected no tools (disallowed agent), got %d", len(got))
	}
	if !bytes.Contains(buf.Bytes(), []byte("allowed_agents")) {
		t.Errorf("missing skip log; got: %s", buf.String())
	}
}

// TestManagerOpenViaFixture spawns the fixture binary and verifies the
// manager exposes its tools under the namespaced name.
func TestManagerOpenViaFixture(t *testing.T) {
	exe, err := osExecutable()
	if err != nil {
		t.Skipf("executable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	specs := []*config.MCPServerSpec{{
		Meta:       config.Meta{Name: "fix", Type: config.TypeMCPServer},
		Transport:  config.TransportStdio,
		Command:    []string{exe, "-test.run=TestHelperMCPFixture"},
		Env:        map[string]string{"TEST_MCP_FIXTURE": "1"},
		ToolPrefix: "fix",
	}}
	mgr, err := Open(ctx, specs, "anyone", nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer mgr.Close()

	reg := mgr.Registry()
	if got, ok := reg.Get("fix__echo"); !ok || got == nil {
		t.Fatalf("registry missing fix__echo; have %v", reg.All())
	}
	echo, _ := reg.Get("fix__echo")
	out, err := echo.Run(ctx, []byte(`{"text":"world"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "echoed: world" {
		t.Errorf("out = %q", out)
	}
}
