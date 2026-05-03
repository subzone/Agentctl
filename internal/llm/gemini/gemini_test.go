package gemini

import (
	"os"
	"testing"

	"github.com/subzone/Agentctl/internal/llm"
)

func TestRegistrationMissingKey(t *testing.T) {
	os.Unsetenv("GEMINI_API_KEY")
	_, _, err := llm.Resolve("gemini/gemini-2.5-flash")
	if err == nil {
		t.Fatal("expected error when GEMINI_API_KEY is unset")
	}
}

func TestRegistrationSuccess(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-key")
	p, model, err := llm.Resolve("gemini/gemini-2.5-pro")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "gemini-2.5-pro" {
		t.Errorf("model = %q", model)
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
}
