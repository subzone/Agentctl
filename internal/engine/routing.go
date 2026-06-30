package engine

import (
	"strings"

	"github.com/subzone/Agentctl/internal/config"
)

// RouteResult holds the classification outcome for a single user turn.
type RouteResult struct {
	Category  string
	Model     string
	Fallback  []string
	System    string // empty = keep default
	MaxTokens int    // 0 = keep default
}

// Classify picks the best expert for the given input using rule-based
// heuristics. No LLM call — just keyword and length checks.
// Returns nil if no routing config or no match (use default model).
func Classify(input string, routing *config.RoutingConfig) *RouteResult {
	if routing == nil || len(routing.Experts) == 0 {
		return nil
	}

	lower := strings.ToLower(input)
	inputLen := len(input)

	// Score each expert. Highest score wins.
	type scored struct {
		score int
		idx   int
	}
	var best scored
	best.idx = -1

	for i, e := range routing.Experts {
		score := 0

		// Keyword match: +10 per keyword found
		for _, kw := range e.Keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				score += 10
			}
		}

		// Length-based routing
		if e.MinLength > 0 && inputLen >= e.MinLength {
			score += 5
		}
		if e.MaxLength > 0 && inputLen <= e.MaxLength {
			score += 3
		}
		// Penalize if input exceeds max_length for this expert
		if e.MaxLength > 0 && inputLen > e.MaxLength {
			score -= 20
		}

		if score > best.score {
			best.score = score
			best.idx = i
		}
	}

	// If no expert scored positive, fall back to first expert (default category)
	if best.idx < 0 {
		best.idx = 0
	}

	e := routing.Experts[best.idx]
	r := &RouteResult{
		Category: e.Category,
		Model:    e.Model,
		Fallback: e.Fallback,
		System:   e.System,
	}
	if e.MaxTokens != nil {
		r.MaxTokens = *e.MaxTokens
	}
	return r
}
