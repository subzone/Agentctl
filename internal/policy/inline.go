package policy

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Rule defines one inline policy rule.
type Rule struct {
	Name            string   `yaml:"name"`
	Tool            string   `yaml:"tool,omitempty"`
	DenyPattern     string   `yaml:"deny_pattern,omitempty"`
	AllowPathPrefix []string `yaml:"allow_path_prefix,omitempty"`
	Message         string   `yaml:"message,omitempty"`
}

type compiledRule struct {
	rule Rule
	re   *regexp.Regexp
}

// InlineEngine evaluates static rules loaded from config files.
type InlineEngine struct {
	rules []compiledRule
}

func NewInlineEngine(rules []Rule) (*InlineEngine, error) {
	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		cr := compiledRule{rule: r}
		if r.DenyPattern != "" {
			re, err := regexp.Compile(r.DenyPattern)
			if err != nil {
				name := r.Name
				if name == "" {
					name = "<unnamed>"
				}
				return nil, fmt.Errorf("policy rule %s has invalid deny_pattern: %w", name, err)
			}
			cr.re = re
		}
		compiled = append(compiled, cr)
	}
	return &InlineEngine{rules: compiled}, nil
}

// Check returns an empty string if allowed or a deny reason if blocked.
func (e *InlineEngine) Check(toolName string, input json.RawMessage) string {
	if e == nil {
		return ""
	}
	payload := string(input)
	path := inputPath(input)
	for _, r := range e.rules {
		if r.rule.Tool != "" && r.rule.Tool != toolName {
			continue
		}
		if r.re != nil && r.re.MatchString(payload) {
			return ruleMessage(r.rule)
		}
		if len(r.rule.AllowPathPrefix) > 0 {
			if !hasAllowedPathPrefix(path, r.rule.AllowPathPrefix) {
				return ruleMessage(r.rule)
			}
		}
	}
	return ""
}

func ruleMessage(r Rule) string {
	if r.Message != "" {
		return r.Message
	}
	if r.Name != "" {
		return r.Name
	}
	return "blocked by policy"
}

func inputPath(input json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(input, &m); err != nil {
		return ""
	}
	for _, key := range []string{"path", "file", "filepath", "target", "dir"} {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func hasAllowedPathPrefix(path string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	if path == "" {
		return false
	}
	cleanPath := filepath.Clean(path)
	for _, p := range prefixes {
		if p == "" {
			continue
		}
		cleanPrefix := filepath.Clean(p)
		if strings.HasPrefix(cleanPath, cleanPrefix) {
			return true
		}
	}
	return false
}
