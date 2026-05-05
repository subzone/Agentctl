package tools

import (
	"strings"
	"testing"
)

func TestPIIGuardScan(t *testing.T) {
	g := NewPIIGuard(PIIModeRedact)

	tests := []struct {
		input string
		want  string // expected PII type
	}{
		{"email me at john@example.com", "email"},
		{"call 555-123-4567", "phone"},
		{"ssn is 123-45-6789", "ssn"},
		{"card 4111-1111-1111-1111", "credit_card"},
		{"server at 192.168.1.100", "ipv4"},
		{"key is AKIAIOSFODNN7EXAMPLE", "aws_key"},
		{"token sk-abc123def456ghi789jkl012mno", "api_key"},
		{"password: hunter2", "password"},
	}

	for _, tt := range tests {
		findings := g.Scan(tt.input)
		if len(findings) == 0 {
			t.Errorf("Scan(%q): expected %s finding, got none", tt.input, tt.want)
			continue
		}
		found := false
		for _, f := range findings {
			if f.Type == tt.want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Scan(%q): expected %s, got %v", tt.input, tt.want, findings)
		}
	}
}

func TestPIIGuardRedact(t *testing.T) {
	g := NewPIIGuard(PIIModeRedact)

	input := "Contact john@example.com or call 555-123-4567. SSN: 123-45-6789"
	result := g.Redact(input)

	if strings.Contains(result, "john@example.com") {
		t.Error("email not redacted")
	}
	if strings.Contains(result, "555-123-4567") {
		t.Error("phone not redacted")
	}
	if strings.Contains(result, "123-45-6789") {
		t.Error("SSN not redacted")
	}
	if !strings.Contains(result, "<EMAIL>") {
		t.Error("missing <EMAIL> placeholder")
	}
	if !strings.Contains(result, "<PHONE>") {
		t.Error("missing <PHONE> placeholder")
	}
}

func TestPIIGuardOff(t *testing.T) {
	g := NewPIIGuard(PIIModeOff)

	findings := g.Scan("john@example.com")
	if len(findings) != 0 {
		t.Error("expected no findings when mode is off")
	}

	result := g.Redact("john@example.com")
	if result != "john@example.com" {
		t.Error("expected no redaction when mode is off")
	}
}

func TestPIIGuardNil(t *testing.T) {
	g := NewPIIGuard("")
	if g.Mode != PIIModeOff {
		t.Errorf("empty mode should be off, got %s", g.Mode)
	}
}
