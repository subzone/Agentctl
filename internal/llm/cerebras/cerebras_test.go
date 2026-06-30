package cerebras

import (
	"os"
	"testing"

	"github.com/subzone/Agentctl/internal/llm"
)

func TestRegister(t *testing.T) {
	os.Setenv("CEREBRAS_API_KEY", "test-key")
	defer os.Unsetenv("CEREBRAS_API_KEY")

	p, model, err := llm.Resolve("cerebras/gpt-oss-120b")
	if err != nil {
		t.Fatal(err)
	}
	if model != "gpt-oss-120b" {
		t.Fatalf("got model %q", model)
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
}
