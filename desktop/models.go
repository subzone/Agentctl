package desktop

// ModelOption is one selectable model for a provider.
type ModelOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// KnownModels returns suggested models per provider for the settings picker.
func (a *App) KnownModels() map[string][]ModelOption {
	return map[string][]ModelOption{
		"groq": {
			{ID: "llama-3.3-70b-versatile", Label: "Llama 3.3 70B"},
			{ID: "llama-3.1-8b-instant", Label: "Llama 3.1 8B (fast)"},
		},
		"gemini": {
			{ID: "gemini-2.5-flash", Label: "Gemini 2.5 Flash"},
			{ID: "gemini-2.0-flash", Label: "Gemini 2.0 Flash"},
		},
		"cerebras": {
			{ID: "gpt-oss-120b", Label: "GPT OSS 120B"},
		},
		"mistral": {
			{ID: "mistral-large-latest", Label: "Mistral Large"},
			{ID: "codestral-latest", Label: "Codestral"},
		},
		"anthropic": {
			{ID: "claude-sonnet-4-20250514", Label: "Claude Sonnet 4"},
			{ID: "claude-3-5-haiku-20241022", Label: "Claude 3.5 Haiku"},
		},
		"openai": {
			{ID: "gpt-4o", Label: "GPT-4o"},
			{ID: "gpt-4o-mini", Label: "GPT-4o mini"},
		},
		"ollama": {
			{ID: "qwen3-coder", Label: "Qwen3 Coder (local)"},
			{ID: "llama3.3", Label: "Llama 3.3 (local)"},
		},
	}
}
