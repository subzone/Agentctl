package main

import (
	"bytes"
	"testing"
)

func TestVersionLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"0.0.1", "0.0.2", true},
		{"0.0.2", "0.0.1", false},
		{"0.0.1", "0.0.1", false},
		{"0.1.0", "0.2.0", true},
		{"1.0.0", "0.9.9", false},
		{"0.0.9", "0.0.10", true},
		{"0.0.1-rc1", "0.0.1", false}, // same after stripping pre-release
	}
	for _, tt := range tests {
		got := versionLess(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("versionLess(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in   string
		want [3]int
		err  bool
	}{
		{"0.0.1", [3]int{0, 0, 1}, false},
		{"v1.2.3", [3]int{1, 2, 3}, false},
		{"0.0.1-rc1", [3]int{0, 0, 1}, false},
		{"1.0.0+build42", [3]int{1, 0, 0}, false},
		{"bad", [3]int{}, true},
		{"1.2", [3]int{}, true},
	}
	for _, tt := range tests {
		got, err := parseVersion(tt.in)
		if (err != nil) != tt.err {
			t.Errorf("parseVersion(%q) err = %v, wantErr = %v", tt.in, err, tt.err)
			continue
		}
		if err == nil && got != tt.want {
			t.Errorf("parseVersion(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestPrintReleases(t *testing.T) {
	rs := []release{
		{Version: "0.0.2", Date: "2026-05-01", Highlights: []string{"feature A"}},
		{Version: "0.0.1", Date: "2026-04-30", Highlights: []string{"initial"}},
	}
	var buf bytes.Buffer
	printReleases(&buf, rs, "What's new:\n")
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("v0.0.2")) {
		t.Errorf("missing v0.0.2 in output: %q", out)
	}
	if !bytes.Contains([]byte(out), []byte("feature A")) {
		t.Errorf("missing highlight in output: %q", out)
	}
	if !bytes.Contains([]byte(out), []byte("What's new:")) {
		t.Errorf("missing header in output: %q", out)
	}
}

func TestPrintReleasesNoHeader(t *testing.T) {
	rs := []release{{Version: "1.0.0", Date: "2026-01-01", Highlights: []string{"big"}}}
	var buf bytes.Buffer
	printReleases(&buf, rs, "")
	if bytes.Contains(buf.Bytes(), []byte("What's new")) {
		t.Error("should not have header when empty string passed")
	}
}

func TestVersionLessMalformed(t *testing.T) {
	// Malformed versions fall back to string comparison.
	if !versionLess("abc", "def") {
		t.Error("expected string fallback: abc < def")
	}
	if versionLess("def", "abc") {
		t.Error("expected string fallback: def > abc")
	}
}
