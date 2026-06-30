package desktop

import (
	"strings"
	"testing"
)

func TestComposeParseToolFormRoundTrip(t *testing.T) {
	a := &App{}
	tpl := a.NewToolTemplate()
	form, err := a.ParseToolForm(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if form.Name != "my_tool" {
		t.Fatalf("name: %q", form.Name)
	}
	out, err := a.ComposeToolForm(form)
	if err != nil {
		t.Fatal(err)
	}
	form2, err := a.ParseToolForm(out)
	if err != nil {
		t.Fatal(err)
	}
	if form2.Name != form.Name || form2.Description != form.Description {
		t.Fatalf("round trip mismatch: %+v vs %+v", form, form2)
	}
	if len(form2.Command) == 0 {
		t.Fatal("expected command")
	}
}

func TestComposeSkillForm(t *testing.T) {
	a := &App{}
	out, err := a.ComposeSkillForm(SkillForm{
		Name:        "go_style",
		Description: "Go coding conventions",
		Body:        "Prefer idiomatic Go. Use errors.Is.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "type: skill") {
		t.Fatal("missing type")
	}
	form, err := a.ParseSkillForm(out)
	if err != nil {
		t.Fatal(err)
	}
	if form.Name != "go_style" || form.Body == "" {
		t.Fatalf("parse: %+v", form)
	}
}
