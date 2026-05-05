// Package alibaba registers an "alibaba" provider that wraps the openai
// adapter against Alibaba Cloud's DashScope OpenAI-compatible endpoint.
// Supports Qwen models with tool calling, streaming, and structured output.
package alibaba

import (
	"errors"
	"os"

	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/llm/openai"
)

const defaultBaseURL = "https://dashscope-intl.aliyuncs.com/compatible-mode"

func init() {
	llm.Register("alibaba", func() (llm.Provider, error) {
		apiKey := os.Getenv("DASHSCOPE_API_KEY")
		if apiKey == "" {
			return nil, errors.New("alibaba API key not found — run `m config` to set it up, or export DASHSCOPE_API_KEY")
		}
		baseURL := os.Getenv("DASHSCOPE_BASE_URL")
		if baseURL == "" {
			baseURL = defaultBaseURL
		}
		return openai.New(openai.WithAPIKey(apiKey), openai.WithBaseURL(baseURL), openai.WithCompat())
	})
}
