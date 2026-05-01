package litellm

import (
	"os"
	"testing"

	"github.com/subzone/m/internal/llm"
)

func TestRegistration(t *testing.T) {
	// The init() in this package registers "litellm". Verify Resolve
	// finds it and that it fails cleanly when env vars are missing.
	os.Unsetenv("LITELLM_BASE_URL")
	os.Unsetenv("LITELLM_API_KEY")

	_, _, err := llm.Resolve("litellm/some-model")
	if err == nil {
		t.Fatal("expected error when LITELLM_BASE_URL is unset")
	}
}

func TestRegistrationWithBaseURLOnly(t *testing.T) {
	t.Setenv("LITELLM_BASE_URL", "http://localhost:4000")
	os.Unsetenv("LITELLM_API_KEY")

	_, _, err := llm.Resolve("litellm/some-model")
	if err == nil {
		t.Fatal("expected error when LITELLM_API_KEY is unset")
	}
}

func TestRegistrationSuccess(t *testing.T) {
	t.Setenv("LITELLM_BASE_URL", "http://localhost:4000")
	t.Setenv("LITELLM_API_KEY", "test-key")

	p, model, err := llm.Resolve("litellm/my-model")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "my-model" {
		t.Errorf("model = %q, want %q", model, "my-model")
	}
	if p == nil {
		t.Fatal("provider is nil")
	}
}
