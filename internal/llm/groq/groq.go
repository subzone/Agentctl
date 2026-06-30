// Package groq registers a "groq" provider. Groq exposes an OpenAI-compatible
// API at api.groq.com, so the openai client works with WithBaseURL + WithCompat.
package groq

import (
	"errors"
	"os"

	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/llm/openai"
)

func init() {
	llm.Register("groq", func() (llm.Provider, error) {
		key := os.Getenv("GROQ_API_KEY")
		if key == "" {
			return nil, errors.New("GROQ_API_KEY is not set — get one free at console.groq.com")
		}
		return openai.New(
			openai.WithAPIKey(key),
			openai.WithBaseURL("https://api.groq.com/openai"),
			openai.WithCompat(),
		)
	})
}
