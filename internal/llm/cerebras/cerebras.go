// Package cerebras registers a "cerebras" provider. Cerebras exposes an
// OpenAI-compatible API at api.cerebras.ai with blazing fast inference.
package cerebras

import (
	"errors"
	"os"

	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/llm/openai"
)

func init() {
	llm.Register("cerebras", func() (llm.Provider, error) {
		key := os.Getenv("CEREBRAS_API_KEY")
		if key == "" {
			return nil, errors.New("CEREBRAS_API_KEY is not set — get one free at cloud.cerebras.ai")
		}
		return openai.New(
			openai.WithAPIKey(key),
			openai.WithBaseURL("https://api.cerebras.ai"),
			openai.WithCompat(),
		)
	})
}
