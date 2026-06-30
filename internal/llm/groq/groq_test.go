package groq

import (
	"os"
	"testing"

	"github.com/subzone/Agentctl/internal/llm"
)

func TestRegister(t *testing.T) {
	os.Setenv("GROQ_API_KEY", "test-key")
	defer os.Unsetenv("GROQ_API_KEY")

	p, model, err := llm.Resolve("groq/llama-3.3-70b-versatile")
	if err != nil {
		t.Fatal(err)
	}
	if model != "llama-3.3-70b-versatile" {
		t.Fatalf("got model %q", model)
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
}
