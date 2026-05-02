// Package gemini registers a "gemini" provider that wraps the openai
// adapter against Google's OpenAI-compatible endpoint at
// generativelanguage.googleapis.com. Supports Gemini 2.5 Pro/Flash,
// tool calling, streaming, and structured output.
package gemini

import (
	"errors"
	"os"

	"github.com/subzone/m/internal/llm"
	"github.com/subzone/m/internal/llm/openai"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"

func init() {
	llm.Register("gemini", func() (llm.Provider, error) {
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil, errors.New("GEMINI_API_KEY is not set")
		}
		baseURL := os.Getenv("GEMINI_BASE_URL")
		if baseURL == "" {
			baseURL = defaultBaseURL
		}
		return openai.New(openai.WithAPIKey(apiKey), openai.WithBaseURL(baseURL), openai.WithCompat(), openai.WithChatPath("/chat/completions"))
	})
}
