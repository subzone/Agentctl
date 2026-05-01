package alibaba

import (
	"os"
	"testing"

	"github.com/subzone/m/internal/llm"
)

func TestRegistrationMissingKey(t *testing.T) {
	os.Unsetenv("DASHSCOPE_API_KEY")
	_, _, err := llm.Resolve("alibaba/qwen-plus")
	if err == nil {
		t.Fatal("expected error when DASHSCOPE_API_KEY is unset")
	}
}

func TestRegistrationSuccess(t *testing.T) {
	t.Setenv("DASHSCOPE_API_KEY", "test-key")
	p, model, err := llm.Resolve("alibaba/qwen-plus")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "qwen-plus" {
		t.Errorf("model = %q", model)
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
}
