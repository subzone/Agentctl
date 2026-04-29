package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestToolAdapterNamespacing(t *testing.T) {
	cases := []struct {
		prefix   string
		upstream string
		want     string
	}{
		{"github", "create_issue", "github__create_issue"},
		{"", "tool_a", "tool_a"},
		{"weird.name", "x", "weird_name__x"},
		{"jira", "issue.search", "jira__issue_search"},
	}
	for _, c := range cases {
		a := NewToolAdapter(nil, Tool{Name: c.upstream}, c.prefix)
		if got := a.Name(); got != c.want {
			t.Errorf("prefix=%q upstream=%q got %q want %q", c.prefix, c.upstream, got, c.want)
		}
	}
}

func TestToolAdapterDefaultsInputSchema(t *testing.T) {
	a := NewToolAdapter(nil, Tool{Name: "x"}, "")
	if string(a.InputSchema()) != `{"type":"object"}` {
		t.Errorf("default schema = %s", a.InputSchema())
	}
	a2 := NewToolAdapter(nil, Tool{Name: "x", InputSchema: json.RawMessage(`{"foo":1}`)}, "")
	if string(a2.InputSchema()) != `{"foo":1}` {
		t.Errorf("schema passthrough failed")
	}
}

func TestToolAdapterRunSuccessAndErrorPaths(t *testing.T) {
	clientIn, clientOut, serverIn, serverOut := pipePair()
	srv := newFakeServer(t, nil)
	srv.callOut = "ok"
	go srv.serve(serverIn, serverOut)
	c := NewPipeClient(clientIn, clientOut)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.Initialize(ctx, "agent", "test")

	a := NewToolAdapter(c, Tool{Name: "do"}, "ns")
	out, err := a.Run(ctx, json.RawMessage(`{"k":1}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "ok" {
		t.Errorf("out = %q", out)
	}
	if a.Name() != "ns__do" {
		t.Errorf("name = %q", a.Name())
	}
	_ = c.Close()
}

func TestToolAdapterRunIsErrorBecomesGoError(t *testing.T) {
	clientIn, clientOut, serverIn, serverOut := pipePair()
	srv := newFakeServer(t, nil)
	srv.callOut = "tool failed"
	srv.callIsErr = true
	go srv.serve(serverIn, serverOut)
	c := NewPipeClient(clientIn, clientOut)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.Initialize(ctx, "agent", "test")

	a := NewToolAdapter(c, Tool{Name: "do"}, "ns")
	out, err := a.Run(ctx, nil)
	if err == nil {
		t.Fatal("expected error from isError=true result")
	}
	if !strings.Contains(out, "tool failed") {
		t.Errorf("out should preserve detail, got %q", out)
	}
	_ = c.Close()
}
