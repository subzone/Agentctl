package tools

import (
	"regexp"
	"strings"
)

// PIIMode controls how PII is handled.
type PIIMode string

const (
	PIIModeOff    PIIMode = "off"
	PIIModeRedact PIIMode = "redact"
	PIIModeWarn   PIIMode = "warn"
)

// PIIGuard scans text for personally identifiable information and
// redacts or flags it based on the configured mode.
type PIIGuard struct {
	Mode     PIIMode
	patterns []piiPattern
}

type piiPattern struct {
	name    string
	re      *regexp.Regexp
	replace string
}

// NewPIIGuard creates a guard with the given mode.
func NewPIIGuard(mode PIIMode) *PIIGuard {
	if mode == PIIModeOff || mode == "" {
		return &PIIGuard{Mode: PIIModeOff}
	}
	return &PIIGuard{
		Mode: mode,
		patterns: []piiPattern{
			{name: "email", re: regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`), replace: "<EMAIL>"},
			{name: "phone", re: regexp.MustCompile(`(?:\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`), replace: "<PHONE>"},
			{name: "ssn", re: regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`), replace: "<SSN>"},
			{name: "credit_card", re: regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`), replace: "<CREDIT_CARD>"},
			{name: "ipv4", re: regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`), replace: "<IP_ADDRESS>"},
			{name: "aws_key", re: regexp.MustCompile(`(?:AKIA|ASIA)[A-Z0-9]{16}`), replace: "<AWS_KEY>"},
			{name: "api_key", re: regexp.MustCompile(`(?:sk-[a-zA-Z0-9]{20,}|ghp_[a-zA-Z0-9]{36}|gho_[a-zA-Z0-9]{36})`), replace: "<API_KEY>"},
			{name: "jwt", re: regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`), replace: "<JWT_TOKEN>"},
			{name: "private_key", re: regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA )?PRIVATE KEY-----`), replace: "<PRIVATE_KEY>"},
			{name: "password", re: regexp.MustCompile(`(?i)(?:password|passwd|pwd)\s*[:=]\s*\S+`), replace: "<PASSWORD_REDACTED>"},
		},
	}
}

// PIIFinding represents a detected PII instance.
type PIIFinding struct {
	Type  string
	Match string
}

// Scan checks text for PII and returns findings.
func (g *PIIGuard) Scan(text string) []PIIFinding {
	if g.Mode == PIIModeOff || len(g.patterns) == 0 {
		return nil
	}
	var findings []PIIFinding
	for _, p := range g.patterns {
		matches := p.re.FindAllString(text, 5)
		for _, m := range matches {
			findings = append(findings, PIIFinding{Type: p.name, Match: m})
		}
	}
	return findings
}

// Redact replaces all PII in text with placeholder tokens.
func (g *PIIGuard) Redact(text string) string {
	if g.Mode == PIIModeOff || len(g.patterns) == 0 {
		return text
	}
	for _, p := range g.patterns {
		text = p.re.ReplaceAllString(text, p.replace)
	}
	return text
}

// Summary returns a human-readable summary of findings.
func (g *PIIGuard) Summary(findings []PIIFinding) string {
	if len(findings) == 0 {
		return ""
	}
	counts := make(map[string]int)
	for _, f := range findings {
		counts[f.Type]++
	}
	var parts []string
	for t, c := range counts {
		if c == 1 {
			parts = append(parts, t)
		} else {
			parts = append(parts, t+"(x"+strings.Repeat("", 0)+string(rune('0'+c))+")")
		}
	}
	return "⚠️  PII detected: " + strings.Join(parts, ", ")
}
