// Package litellm registers a "litellm" provider that wraps the openai
// provider with a custom base URL and API key. LiteLLM exposes an
// OpenAI-compatible /v1/chat/completions endpoint, so the openai client
// works unchanged once the URL and key are pointed at it.
package litellm

import (
	"errors"
	"os"

	"github.com/subzone/m/internal/llm"
	"github.com/subzone/m/internal/llm/openai"
)

func init() {
	llm.Register("litellm", func() (llm.Provider, error) {
		baseURL := os.Getenv("LITELLM_BASE_URL")
		if baseURL == "" {
			return nil, errors.New("LITELLM_BASE_URL is not set")
		}
		apiKey := os.Getenv("LITELLM_API_KEY")
		if apiKey == "" {
			return nil, errors.New("LITELLM_API_KEY is not set")
		}
		return openai.New(openai.WithAPIKey(apiKey), openai.WithBaseURL(baseURL))
	})
}
