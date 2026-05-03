package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDelegateRunSuccess(t *testing.T) {
	var got struct {
		name, task, model string
	}
	spawn := func(_ context.Context, name, task, model string) (string, error) {
		got.name, got.task, got.model = name, task, model
		return "subagent reply", nil
	}
	d := NewDelegate(spawn, []string{"planner", "researcher"})
	out, err := d.Run(context.Background(), json.RawMessage(`{"name":"planner","task":"plan it"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "subagent reply" {
		t.Errorf("out = %q", out)
	}
	if got.name != "planner" || got.task != "plan it" || got.model != "" {
		t.Errorf("captured = %+v", got)
	}
}

func TestDelegateRejectsNotInList(t *testing.T) {
	d := NewDelegate(func(context.Context, string, string, string) (string, error) {
		t.Fatal("spawn should not be called")
		return "", nil
	}, []string{"planner"})
	_, err := d.Run(context.Background(), json.RawMessage(`{"name":"intruder","task":"x"}`))
	if err == nil || !strings.Contains(err.Error(), "intruder") {
		t.Errorf("got %v", err)
	}
}

func TestDelegateRequiresFields(t *testing.T) {
	d := NewDelegate(func(context.Context, string, string, string) (string, error) { return "", nil }, []string{"planner"})
	cases := []string{
		`{"task":"x"}`,
		`{"name":"planner"}`,
		`{}`,
		``,
	}
	for _, in := range cases {
		_, err := d.Run(context.Background(), json.RawMessage(in))
		if err == nil {
			t.Errorf("Run(%q) should have errored", in)
		}
	}
}

func TestDelegatePropagatesSpawnError(t *testing.T) {
	want := errors.New("upstream broke")
	d := NewDelegate(func(context.Context, string, string, string) (string, error) {
		return "", want
	}, []string{"planner"})
	_, err := d.Run(context.Background(), json.RawMessage(`{"name":"planner","task":"go"}`))
	if !errors.Is(err, want) {
		t.Errorf("got %v, want wraps %v", err, want)
	}
}

func TestDelegateInputSchemaIncludesEnum(t *testing.T) {
	d := NewDelegate(nil, []string{"alpha", "beta"})
	var schema map[string]any
	if err := json.Unmarshal(d.InputSchema(), &schema); err != nil {
		t.Fatalf("schema unmarshal: %v", err)
	}
	props, _ := schema["properties"].(map[string]any)
	name, _ := props["name"].(map[string]any)
	enum, _ := name["enum"].([]any)
	if len(enum) != 2 || enum[0] != "alpha" || enum[1] != "beta" {
		t.Errorf("enum = %+v", name["enum"])
	}
	required, _ := schema["required"].([]any)
	if len(required) != 2 {
		t.Errorf("required = %+v", required)
	}
}

func TestDelegateDescriptionListsAvailable(t *testing.T) {
	d := NewDelegate(nil, []string{"planner", "researcher"})
	desc := d.Description()
	if !strings.Contains(desc, "planner") || !strings.Contains(desc, "researcher") {
		t.Errorf("description missing names: %q", desc)
	}
}

func TestDelegateNoSpawnSurfacesError(t *testing.T) {
	d := NewDelegate(nil, []string{"planner"})
	_, err := d.Run(context.Background(), json.RawMessage(`{"name":"planner","task":"go"}`))
	if err == nil || !strings.Contains(err.Error(), "no spawn function") {
		t.Errorf("got %v", err)
	}
}

func TestDelegateModelOverride(t *testing.T) {
	var got struct {
		name, task, model string
	}
	spawn := func(_ context.Context, name, task, model string) (string, error) {
		got.name, got.task, got.model = name, task, model
		return "subagent reply", nil
	}
	d := NewDelegate(spawn, []string{"planner", "researcher"})
	out, err := d.Run(context.Background(), json.RawMessage(`{"name":"planner","task":"plan it","model":"anthropic/claude-3-5-haiku"}`))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "subagent reply" {
		t.Errorf("out = %q", out)
	}
	if got.name != "planner" || got.task != "plan it" || got.model != "anthropic/claude-3-5-haiku" {
		t.Errorf("captured = %+v", got)
	}
}
