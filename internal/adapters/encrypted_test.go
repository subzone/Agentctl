package adapters

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/subzone/Agentctl/internal/ports"
)

// fixedKeyProvider provides a fixed key for testing.
// In production, use keychain or env var provider.
type fixedKeyProvider struct {
	key []byte
}

func (f *fixedKeyProvider) GetKey(_ context.Context) ([]byte, error) {
	return f.key, nil
}

func TestEncryptDecrypt(t *testing.T) {
	// Generate a test key
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)

	ciphertext, err := encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	// Verify ciphertext is different from plaintext
	if string(plaintext) == string(ciphertext) {
		t.Error("ciphertext should differ from plaintext")
	}

	// Decrypt and verify
	decrypted, err := decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %s, want %s", decrypted, plaintext)
	}
}

func TestEncryptTamperDetection(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("important data")

	ciphertext, err := encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	// Tamper with the ciphertext
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err = decrypt(key, ciphertext)
	if err == nil {
		t.Error("decrypt should fail when data is tampered")
	}
}

func TestEncryptWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
		key2[i] = byte(i + 1)
	}

	plaintext := []byte("test data")

	ciphertext, err := encrypt(key1, plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	_, err = decrypt(key2, ciphertext)
	if err == nil {
		t.Error("decrypt should fail with wrong key")
	}
}

func TestEncryptedFileStore(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	store, err := NewEncryptedFileStore(tmpDir, &fixedKeyProvider{key: key})
	if err != nil {
		t.Fatalf("NewEncryptedFileStore failed: %v", err)
	}

	ctx := context.Background()
	sessionID := "test-session-123"
	sessionData := &ports.SessionData{
		Provider:     "openai",
		Model:        "gpt-4",
		InputTokens:  100,
		OutputTokens: 50,
		Messages: []ports.Message{
			{
				Role: "user",
				Content: []ports.ContentBlock{
					{Type: "text", Text: "Hello, how are you?"},
				},
			},
			{
				Role: "assistant",
				Content: []ports.ContentBlock{
					{Type: "text", Text: "I'm doing well, thank you!"},
				},
			},
		},
	}

	// Test SaveSession
	err = store.SaveSession(ctx, sessionID, sessionData)
	if err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// Verify file exists and has .enc extension
	filePath := filepath.Join(tmpDir, sessionID+".enc")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("encrypted session file should exist")
	}

	// Test LoadSession
	loaded, err := store.LoadSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	if loaded.Provider != sessionData.Provider {
		t.Errorf("provider = %s, want %s", loaded.Provider, sessionData.Provider)
	}
	if loaded.Model != sessionData.Model {
		t.Errorf("model = %s, want %s", loaded.Model, sessionData.Model)
	}
	if len(loaded.Messages) != len(sessionData.Messages) {
		t.Errorf("messages count = %d, want %d", len(loaded.Messages), len(sessionData.Messages))
	}

	// Test ListSessions
	sessions, err := store.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 1 || sessions[0] != sessionID {
		t.Errorf("ListSessions = %v, want [%s]", sessions, sessionID)
	}

	// Test DeleteSession
	err = store.DeleteSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("session file should be deleted")
	}

	// Test LoadSession on deleted session
	_, err = store.LoadSession(ctx, sessionID)
	if err == nil {
		t.Error("LoadSession should fail for deleted session")
	}
}

func TestEncryptedFileStoreNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	key := make([]byte, 32)
	store, _ := NewEncryptedFileStore(tmpDir, &fixedKeyProvider{key: key})

	_, err := store.LoadSession(context.Background(), "nonexistent")
	if err == nil {
		t.Error("LoadSession should fail for nonexistent session")
	}
	if _, ok := err.(*ports.ErrNotFound); !ok {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestEnvKeyProvider(t *testing.T) {
	// Set environment variable with valid base64 key
	validKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	t.Setenv("TEST_ENCRYPTION_KEY", validKey)

	provider := &EnvKeyProvider{EnvVar: "TEST_ENCRYPTION_KEY"}
	key, err := provider.GetKey(context.Background())
	if err != nil {
		t.Fatalf("GetKey failed: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("key length = %d, want 32", len(key))
	}

	// Test with missing env var
	provider = &EnvKeyProvider{EnvVar: "NONEXISTENT_VAR_12345"}
	_, err = provider.GetKey(context.Background())
	if err == nil {
		t.Error("GetKey should fail when env var not set")
	}

	// Test with invalid base64
	t.Setenv("TEST_ENCRYPTION_KEY", "not-valid-base64!!!")
	provider = &EnvKeyProvider{EnvVar: "TEST_ENCRYPTION_KEY"}
	_, err = provider.GetKey(context.Background())
	if err == nil {
		t.Error("GetKey should fail with invalid base64")
	}

	// Test with wrong key size (16 bytes instead of 32)
	shortKey := base64.StdEncoding.EncodeToString(make([]byte, 16))
	t.Setenv("TEST_ENCRYPTION_KEY", shortKey)
	provider = &EnvKeyProvider{EnvVar: "TEST_ENCRYPTION_KEY"}
	_, err = provider.GetKey(context.Background())
	if err == nil {
		t.Error("GetKey should fail with wrong key size")
	}
}

func TestEncryptedFileStoreEmptyID(t *testing.T) {
	tmpDir := t.TempDir()
	key := make([]byte, 32)
	store, _ := NewEncryptedFileStore(tmpDir, &fixedKeyProvider{key: key})

	ctx := context.Background()
	sessionData := &ports.SessionData{Provider: "test", Model: "test"}

	// Empty ID should fail
	err := store.SaveSession(ctx, "", sessionData)
	if err == nil {
		t.Error("SaveSession should fail with empty ID")
	}

	_, err = store.LoadSession(ctx, "")
	if err == nil {
		t.Error("LoadSession should fail with empty ID")
	}

	err = store.DeleteSession(ctx, "")
	if err == nil {
		t.Error("DeleteSession should fail with empty ID")
	}
}
