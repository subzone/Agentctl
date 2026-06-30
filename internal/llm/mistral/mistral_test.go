package mistral

import (
	"os"
	"testing"

	"github.com/subzone/Agentctl/internal/llm"
)

func TestRegister(t *testing.T) {
	os.Setenv("MISTRAL_API_KEY", "test-key")
	defer os.Unsetenv("MISTRAL_API_KEY")

	p, model, err := llm.Resolve("mistral/mistral-large-latest")
	if err != nil {
		t.Fatal(err)
	}
	if model != "mistral-large-latest" {
		t.Fatalf("got model %q", model)
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
}
