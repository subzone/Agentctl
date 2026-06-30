package desktop

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/subzone/Agentctl/internal/adapters"
	"github.com/subzone/Agentctl/internal/ports"
)

type fixedKey struct{ key []byte }

func (f *fixedKey) GetKey(context.Context) ([]byte, error) { return f.key, nil }

func TestSessionPreviewAndSkip(t *testing.T) {
	data := &ports.SessionData{
		Messages: []ports.Message{{
			Role: "user",
			Content: []ports.ContentBlock{{
				Type: "text",
				Text: "hello world from a saved chat",
			}},
		}},
	}
	p := sessionPreview(data)
	if p == "" {
		t.Fatal("expected preview")
	}
	if skipSavedSessionID("_autosave") || skipSavedSessionID("_auto_2026-01-01_12-00-00") {
		// ok
	} else {
		t.Fatal("should skip autosave ids")
	}
	if skipSavedSessionID("s_1234567890123") {
		t.Fatal("should not skip desktop session ids")
	}
}

func TestListSavedSessionsEmpty(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "sessions")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	store, err := adapters.NewEncryptedFileStore(dir, &fixedKey{key: key})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = store.SaveSession(ctx, "s_1000", &ports.SessionData{
		Provider: "groq",
		Model:    "groq/llama",
		Messages: []ports.Message{{
			Role:    "user",
			Content: []ports.ContentBlock{{Type: "text", Text: "test"}},
		}},
	})

	// Override session store path by testing helpers directly via store list
	ids, err := store.ListSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 session, got %d", len(ids))
	}
}
