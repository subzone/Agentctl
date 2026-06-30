package cerebras

import (
	"os"
	"testing"

	"github.com/subzone/Agentctl/internal/llm"
)

func TestRegister(t *testing.T) {
	os.Setenv("CEREBRAS_API_KEY", "test-key")
	defer os.Unsetenv("CEREBRAS_API_KEY")

	p, model, err := llm.Resolve("cerebras/llama-3.3-70b")
	if err != nil {
		t.Fatal(err)
	}
	if model != "llama-3.3-70b" {
		t.Fatalf("got model %q", model)
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
}
