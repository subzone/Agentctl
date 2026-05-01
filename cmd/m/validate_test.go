package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestValidateExamples(t *testing.T) {
	cmd := newValidateCmd()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"../../examples/"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate examples: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(out.String(), "0 issue(s)") {
		t.Errorf("output = %q", out.String())
	}
}

func TestValidateBadFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir+"/bad.md", "no frontmatter here")
	cmd := newValidateCmd()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{dir})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for bad file")
	}
}

func TestChangelogCmd(t *testing.T) {
	cmd := newChangelogCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("changelog: %v", err)
	}
	if !strings.Contains(out.String(), "v0.0.1") {
		t.Errorf("missing v0.0.1 in output: %q", out.String())
	}
}

func TestRootVersionFlag(t *testing.T) {
	root := &cobra.Command{
		Use:     "m",
		Version: "test-version",
	}
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--version: %v", err)
	}
	if !strings.Contains(out.String(), "test-version") {
		t.Errorf("output = %q", out.String())
	}
}

func TestStdinIsPipeWithNonFile(t *testing.T) {
	// Non-*os.File readers are treated as pipe-like.
	r := strings.NewReader("hello")
	if !stdinIsPipe(r) {
		t.Error("non-file reader should be treated as pipe")
	}
}

func TestSystemWithCwd(t *testing.T) {
	result := systemWithCwd("base prompt")
	if !strings.Contains(result, "base prompt") {
		t.Error("missing base prompt")
	}
	if !strings.Contains(result, "Current working directory:") {
		t.Error("missing cwd injection")
	}
}
