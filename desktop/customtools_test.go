package desktop

import (
	"strings"
	"testing"
)

func TestSaveToolLifecycle(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	a := NewApp()
	tpl := a.NewToolTemplate()

	// New tool saves fine.
	if err := a.SaveTool("", tpl); err != nil {
		t.Fatalf("save new my_tool: %v", err)
	}

	// Saving another *new* tool with the same name must NOT clobber it.
	if err := a.SaveTool("", tpl); err == nil {
		t.Fatal("expected overwrite guard error for duplicate new tool")
	} else if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Editing the same tool (oldName == name) is allowed.
	edited := strings.Replace(tpl, "timeout_sec: 30", "timeout_sec: 45", 1)
	if err := a.SaveTool("my_tool", edited); err != nil {
		t.Fatalf("edit my_tool in place: %v", err)
	}

	// A second, differently named tool accumulates.
	second := strings.Replace(tpl, "name: my_tool", "name: weather", 1)
	if err := a.SaveTool("", second); err != nil {
		t.Fatalf("save weather: %v", err)
	}
	if got := len(a.ListTools()); got != 2 {
		t.Fatalf("expected 2 tools, got %d", got)
	}

	// Rename my_tool -> my_tool_2: old file is removed, count stays 2.
	renamed := strings.Replace(tpl, "name: my_tool", "name: my_tool_2", 1)
	if err := a.SaveTool("my_tool", renamed); err != nil {
		t.Fatalf("rename my_tool: %v", err)
	}
	names := map[string]bool{}
	for _, td := range a.ListTools() {
		names[td.Name] = true
	}
	if names["my_tool"] || !names["my_tool_2"] || !names["weather"] {
		t.Fatalf("unexpected tools after rename: %v", names)
	}
}

func TestSaveToolFriendlyNameError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	a := NewApp()
	bad := strings.Replace(a.NewToolTemplate(), "name: my_tool", "name: My Tool", 1)
	err := a.SaveTool("", bad)
	if err == nil || !strings.Contains(err.Error(), "lowercase") {
		t.Fatalf("expected friendly name error, got %v", err)
	}
}
