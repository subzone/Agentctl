// Package mistral registers a "mistral" provider. Mistral exposes an
// OpenAI-compatible API at api.mistral.ai with large free-tier quotas.
package mistral

import (
	"errors"
	"os"

	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/llm/openai"
)

func init() {
	llm.Register("mistral", func() (llm.Provider, error) {
		key := os.Getenv("MISTRAL_API_KEY")
		if key == "" {
			return nil, errors.New("MISTRAL_API_KEY is not set — get one free at console.mistral.ai")
		}
		return openai.New(
			openai.WithAPIKey(key),
			openai.WithBaseURL("https://api.mistral.ai"),
			openai.WithCompat(),
		)
	})
}
