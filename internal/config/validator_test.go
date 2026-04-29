package config

import (
	"strings"
	"testing"
)

func TestValidateAgentRequiresModel(t *testing.T) {
	doc := &Document{
		Path: "x.md",
		Spec: &AgentSpec{Meta: Meta{Name: "ok", Type: TypeAgent}},
	}
	issues := Validate(doc)
	if len(issues) == 0 {
		t.Fatal("expected issues for missing model")
	}
	found := false
	for _, i := range issues {
		if i.Field == "model" {
			found = true
		}
	}
	if !found {
		t.Errorf("no model issue in %v", issues)
	}
}

func TestValidateAgentValid(t *testing.T) {
	doc := &Document{
		Path: "x.md",
		Spec: &AgentSpec{
			Meta:  Meta{Name: "ok", Type: TypeAgent},
			Model: "anthropic/claude-sonnet-4-6",
		},
	}
	if issues := Validate(doc); len(issues) != 0 {
		t.Errorf("unexpected issues: %v", issues)
	}
}

func TestValidateNameFormat(t *testing.T) {
	doc := &Document{
		Path: "x.md",
		Spec: &AgentSpec{
			Meta:  Meta{Name: "Bad Name!", Type: TypeAgent},
			Model: "anthropic/claude-sonnet-4-6",
		},
	}
	issues := Validate(doc)
	if len(issues) == 0 {
		t.Fatal("expected name format issue")
	}
}

func TestValidateDocsCrossRef(t *testing.T) {
	docs := []*Document{
		{
			Path: "agent.md",
			Spec: &AgentSpec{
				Meta:   Meta{Name: "main", Type: TypeAgent},
				Model:  "anthropic/claude-sonnet-4-6",
				Skills: []string{"missing-skill"},
				MCP:    []string{"missing-mcp"},
			},
		},
	}
	issues := ValidateDocs(docs)
	if len(issues) < 2 {
		t.Fatalf("expected at least 2 cross-ref issues, got %v", issues)
	}
	combined := ""
	for _, i := range issues {
		combined += i.Error() + "\n"
	}
	if !strings.Contains(combined, "missing-skill") || !strings.Contains(combined, "missing-mcp") {
		t.Errorf("issues missing expected refs: %s", combined)
	}
}

func TestValidateDocsDuplicate(t *testing.T) {
	docs := []*Document{
		{Path: "a.md", Spec: &AgentSpec{Meta: Meta{Name: "dup", Type: TypeAgent}, Model: "anthropic/m"}},
		{Path: "b.md", Spec: &AgentSpec{Meta: Meta{Name: "dup", Type: TypeAgent}, Model: "anthropic/m"}},
	}
	issues := ValidateDocs(docs)
	dupFound := false
	for _, i := range issues {
		if strings.Contains(i.Message, "duplicate") {
			dupFound = true
		}
	}
	if !dupFound {
		t.Errorf("expected duplicate issue, got %v", issues)
	}
}
