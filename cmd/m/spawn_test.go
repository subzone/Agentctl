package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/subzone/Agentctl/internal/config"
	"github.com/subzone/Agentctl/internal/engine"
	"github.com/subzone/Agentctl/internal/llm"
)

// runHub is a small helper that drives engine.Run for a synthetic hub agent.
func runHub(t *testing.T, p llm.Provider, _ *config.AgentSpec, rt *agentRuntime, out, status io.Writer) error {
	t.Helper()
	return engine.Run(context.Background(), engine.Config{
		Provider: p,
		Model:    "m",
		System:   "test",
		Tools:    rt.registry,
		Out:      out,
		Status:   status,
	}, "go")
}

func TestPrefixWriterSimple(t *testing.T) {
	var buf bytes.Buffer
	pw := &prefixWriter{w: &buf, prefix: "[a] "}
	pw.Write([]byte("hello\n"))
	if got, want := buf.String(), "[a] hello\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrefixWriterMultiLine(t *testing.T) {
	var buf bytes.Buffer
	pw := &prefixWriter{w: &buf, prefix: "[a] "}
	pw.Write([]byte("line one\nline two\n"))
	want := "[a] line one\n[a] line two\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestPrefixWriterStreamedSubLine(t *testing.T) {
	// Streamed deltas often arrive sub-line — the prefix should appear once
	// per line, not once per Write.
	var buf bytes.Buffer
	pw := &prefixWriter{w: &buf, prefix: "[a] "}
	pw.Write([]byte("hello "))
	pw.Write([]byte("world\nnext"))
	pw.Write([]byte(" line\n"))
	want := "[a] hello world\n[a] next line\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestPrefixWriterTrailingPartial(t *testing.T) {
	// A partial line (no trailing \n) should still get prefixed exactly once.
	var buf bytes.Buffer
	pw := &prefixWriter{w: &buf, prefix: "[a] "}
	pw.Write([]byte("partial"))
	if buf.String() != "[a] partial" {
		t.Errorf("got %q", buf.String())
	}
	pw.Write([]byte(" rest\n"))
	if buf.String() != "[a] partial rest\n" {
		t.Errorf("got %q", buf.String())
	}
}

func TestFindAgentDoc(t *testing.T) {
	docs := []*config.Document{
		{Path: "x.md", Spec: &config.AgentSpec{Meta: config.Meta{Name: "alpha", Type: config.TypeAgent}, Model: "anthropic/x"}},
		{Path: "y.md", Spec: &config.MCPServerSpec{Meta: config.Meta{Name: "alpha", Type: config.TypeMCPServer}}},
	}
	got, err := findAgentDoc(docs, "alpha")
	if err != nil {
		t.Fatalf("findAgentDoc: %v", err)
	}
	if got.Path != "x.md" {
		t.Errorf("found wrong doc: %s", got.Path)
	}

	if _, err := findAgentDoc(docs, "missing"); err == nil {
		t.Error("expected error for missing")
	}
}

func TestProjectRootWithSiblingSubdirs(t *testing.T) {
	dir := t.TempDir()
	mustMkdirAll(t, dir+"/agents")
	mustMkdirAll(t, dir+"/mcp")
	mustWrite(t, dir+"/agents/foo.md", "stub")

	got, err := projectRoot(dir + "/agents/foo.md")
	if err != nil {
		t.Fatalf("projectRoot: %v", err)
	}
	if want := mustAbs(t, dir); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProjectRootFlat(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir+"/foo.md", "stub")
	got, err := projectRoot(dir + "/foo.md")
	if err != nil {
		t.Fatalf("projectRoot: %v", err)
	}
	// No sibling subdirs in dir's parent → use the agent's own dir.
	if want := mustAbs(t, dir); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// scriptedProvider replays a pre-built sequence of event-streams, one per
// turn. Mirrors the helper in the engine tests but kept local so this
// package needn't import engine_test.
type scriptedProvider struct {
	turns [][]llm.Event
}

func (s *scriptedProvider) Stream(ctx context.Context, req llm.Request) (<-chan llm.Event, error) {
	if len(s.turns) == 0 {
		return nil, errors.New("scriptedProvider: out of turns")
	}
	turn := s.turns[0]
	s.turns = s.turns[1:]
	ch := make(chan llm.Event, len(turn))
	for _, ev := range turn {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func TestSpawnerRunsSubagentEndToEnd(t *testing.T) {
	// Subagent doc; no MCP, no tools, no further subagents.
	subDoc := &config.Document{
		Path: "p.md",
		Body: "you are a planner",
		Spec: &config.AgentSpec{
			Meta:  config.Meta{Name: "planner", Type: config.TypeAgent},
			Model: "fake/p",
		},
	}

	// Stash a fake provider that the subagent will use.
	subProvider := &scriptedProvider{turns: [][]llm.Event{{
		{Kind: llm.EventText, Text: "1) a\n2) b"},
		{Kind: llm.EventDone, StopReason: "end_turn"},
	}}}
	llm.Register("fake", func() (llm.Provider, error) { return subProvider, nil })

	var out, status bytes.Buffer
	sp := &spawner{
		docs:       []*config.Document{subDoc},
		out:        &out,
		status:     &status,
		spawnDepth: 1,
	}

	got, err := sp.spawn(context.Background(), "planner", "plan it")
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if got != "1) a\n2) b" {
		t.Errorf("returned text = %q", got)
	}
	// Subagent text should be prefixed in the user-facing stream.
	if !strings.Contains(out.String(), "[planner] 1) a") {
		t.Errorf("out missing prefixed text: %q", out.String())
	}
	// Status should bracket the call.
	s := status.String()
	if !strings.Contains(s, "→ delegate planner: plan it") {
		t.Errorf("status missing call open: %q", s)
	}
	if !strings.Contains(s, "← delegate planner") {
		t.Errorf("status missing call close: %q", s)
	}
}

// TestHubDelegatesToSubagent exercises the full hub → engine → delegate
// tool → spawner → subagent engine → result-as-tool-result-back-to-hub
// flow. The hub's first turn calls delegate; its second turn produces
// final text using the subagent's reply.
func TestHubDelegatesToSubagent(t *testing.T) {
	// Subagent: a single text turn.
	subDoc := &config.Document{
		Path: "sub.md",
		Body: "you are a planner",
		Spec: &config.AgentSpec{
			Meta:  config.Meta{Name: "planner", Type: config.TypeAgent},
			Model: "subprov/m",
		},
	}
	subProvider := &scriptedProvider{turns: [][]llm.Event{{
		{Kind: llm.EventText, Text: "first do A then B"},
		{Kind: llm.EventDone, StopReason: "end_turn"},
	}}}
	llm.Register("subprov", func() (llm.Provider, error) { return subProvider, nil })

	// Hub: turn 1 fires the delegate tool; turn 2 produces final text.
	hubSpec := &config.AgentSpec{
		Meta:      config.Meta{Name: "hub", Type: config.TypeAgent},
		Model:     "hubprov/m",
		Subagents: []string{"planner"},
	}
	hubProvider := &scriptedProvider{turns: [][]llm.Event{
		{
			{Kind: llm.EventToolCall, ToolID: "tu_1", ToolName: "delegate", ToolInput: json.RawMessage(`{"name":"planner","task":"plan it"}`)},
			{Kind: llm.EventDone, StopReason: "tool_use"},
		},
		{
			{Kind: llm.EventText, Text: "Planner said: first do A then B"},
			{Kind: llm.EventDone, StopReason: "end_turn"},
		},
	}}

	var out, status bytes.Buffer
	hubSpawner := &spawner{
		docs:       []*config.Document{subDoc},
		out:        &out,
		status:     &status,
		spawnDepth: 1,
	}
	rt, err := buildAgentRuntime(context.Background(), hubSpec, []*config.Document{subDoc}, hubSpawner, &status)
	if err != nil {
		t.Fatalf("buildAgentRuntime: %v", err)
	}
	defer rt.close()
	if _, ok := rt.registry.Get("delegate"); !ok {
		t.Fatal("hub registry missing delegate tool")
	}

	// Drive the hub through engine.Run by faking the hub provider directly.
	// (Importing engine here is fine; the file already references llm types.)
	if err := runHub(t, hubProvider, hubSpec, rt, &out, &status); err != nil {
		t.Fatalf("runHub: %v", err)
	}

	hubText := out.String()
	if !strings.Contains(hubText, "Planner said: first do A then B") {
		t.Errorf("hub output missing final text: %q", hubText)
	}
	// Subagent output streamed with prefix.
	if !strings.Contains(hubText, "[planner] first do A then B") {
		t.Errorf("hub output missing prefixed subagent text: %q", hubText)
	}
	// Status records the delegation envelope.
	s := status.String()
	if !strings.Contains(s, "→ delegate planner") || !strings.Contains(s, "← delegate planner") {
		t.Errorf("status missing delegate envelope: %q", s)
	}
}

func TestSpawnerDepthLimit(t *testing.T) {
	sp := &spawner{spawnDepth: MaxSubagentDepth + 1}
	_, err := sp.spawn(context.Background(), "anything", "go")
	if err == nil || !strings.Contains(err.Error(), "max subagent depth") {
		t.Errorf("got %v", err)
	}
}

func TestSpawnerUnknownSubagent(t *testing.T) {
	sp := &spawner{spawnDepth: 1, docs: nil, out: io.Discard, status: io.Discard}
	_, err := sp.spawn(context.Background(), "ghost", "go")
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Errorf("got %v", err)
	}
}

func TestBuildAgentRuntimeAutoIncludesDelegate(t *testing.T) {
	spec := &config.AgentSpec{
		Meta:      config.Meta{Name: "hub", Type: config.TypeAgent},
		Model:     "anthropic/x",
		Tools:     []string{"shell"},
		Subagents: []string{"planner"},
	}
	subDoc := &config.Document{
		Path: "p.md",
		Spec: &config.AgentSpec{
			Meta:  config.Meta{Name: "planner", Type: config.TypeAgent},
			Model: "anthropic/y",
		},
	}
	sp := &spawner{spawnDepth: 1, docs: []*config.Document{subDoc}}

	rt, err := buildAgentRuntime(context.Background(), spec, []*config.Document{subDoc}, sp, io.Discard)
	if err != nil {
		t.Fatalf("buildAgentRuntime: %v", err)
	}
	defer rt.close()

	if _, ok := rt.registry.Get("delegate"); !ok {
		t.Error("delegate should be auto-included when subagents present")
	}
	if _, ok := rt.registry.Get("shell"); !ok {
		t.Error("explicit allowlist tool missing")
	}
}

func TestBuildAgentRuntimeNoSubagentsNoDelegate(t *testing.T) {
	spec := &config.AgentSpec{
		Meta:  config.Meta{Name: "hub", Type: config.TypeAgent},
		Model: "anthropic/x",
		Tools: []string{"shell"},
	}
	rt, err := buildAgentRuntime(context.Background(), spec, nil, &spawner{spawnDepth: 1}, io.Discard)
	if err != nil {
		t.Fatalf("buildAgentRuntime: %v", err)
	}
	defer rt.close()
	if _, ok := rt.registry.Get("delegate"); ok {
		t.Error("delegate should not be present when subagents is empty")
	}
}

func TestBuildAgentRuntimeOmitsDelegateBeyondDepth(t *testing.T) {
	spec := &config.AgentSpec{
		Meta:      config.Meta{Name: "deep", Type: config.TypeAgent},
		Model:     "anthropic/x",
		Subagents: []string{"sub"},
	}
	// Beyond the cap: the next spawn would create depth MaxSubagentDepth+1.
	sp := &spawner{spawnDepth: MaxSubagentDepth + 1}
	rt, err := buildAgentRuntime(context.Background(), spec, nil, sp, io.Discard)
	if err != nil {
		t.Fatalf("buildAgentRuntime: %v", err)
	}
	defer rt.close()
	if _, ok := rt.registry.Get("delegate"); ok {
		t.Error("delegate should be omitted once depth cap is reached")
	}
}

// Encode an InputSchema to make sure the auto-included delegate's enum
// reflects the parent's subagents list.
func TestBuildAgentRuntimeDelegateEnumMatchesSubagents(t *testing.T) {
	spec := &config.AgentSpec{
		Meta:      config.Meta{Name: "hub", Type: config.TypeAgent},
		Model:     "anthropic/x",
		Subagents: []string{"alpha", "beta"},
	}
	subDocs := []*config.Document{
		{Spec: &config.AgentSpec{Meta: config.Meta{Name: "alpha", Type: config.TypeAgent}, Model: "x/y"}},
		{Spec: &config.AgentSpec{Meta: config.Meta{Name: "beta", Type: config.TypeAgent}, Model: "x/y"}},
	}
	sp := &spawner{spawnDepth: 1, docs: subDocs}
	rt, err := buildAgentRuntime(context.Background(), spec, subDocs, sp, io.Discard)
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer rt.close()
	d, ok := rt.registry.Get("delegate")
	if !ok {
		t.Fatal("missing delegate")
	}
	var schema map[string]any
	if err := json.Unmarshal(d.InputSchema(), &schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	props := schema["properties"].(map[string]any)
	name := props["name"].(map[string]any)
	enum, _ := name["enum"].([]any)
	if len(enum) != 2 || enum[0] != "alpha" || enum[1] != "beta" {
		t.Errorf("enum = %+v", name["enum"])
	}
}
