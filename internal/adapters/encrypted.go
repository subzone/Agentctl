package adapters

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/subzone/Agentctl/internal/ports"
)

// EncryptedFileStore persists sessions as encrypted files in a directory.
// Uses AES-256-GCM encryption with a key stored separately.
//
// The encryption format is: nonce (12 bytes) || ciphertext || auth tag (16 bytes)
// This provides both confidentiality and integrity verification.
//
// File extension is ".enc" to distinguish from plaintext session files.
type EncryptedFileStore struct {
	dir         string
	keyProvider KeyProvider
}

// KeyProvider retrieves the encryption key. Implement this to fetch from
// keychain, env var, or wherever you store secrets.
type KeyProvider interface {
	GetKey(ctx context.Context) ([]byte, error)
}

// EnvKeyProvider fetches key from environment variable (for dev/testing).
// This is NOT recommended for production use.
type EnvKeyProvider struct {
	EnvVar string
}

// GetKey retrieves the encryption key from an environment variable.
func (e *EnvKeyProvider) GetKey(_ context.Context) ([]byte, error) {
	key := os.Getenv(e.EnvVar)
	if key == "" {
		return nil, fmt.Errorf("environment variable %s not set", e.EnvVar)
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("decode base64 key: %w", err)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes (256 bits), got %d", len(decoded))
	}
	return decoded, nil
}

// NewEncryptedFileStore creates an EncryptedFileStore. Creates the directory if needed.
// The keyProvider will be used to retrieve the encryption key for each operation.
func NewEncryptedFileStore(dir string, keyProvider KeyProvider) (*EncryptedFileStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	if keyProvider == nil {
		return nil, errors.New("keyProvider is required")
	}
	return &EncryptedFileStore{dir: dir, keyProvider: keyProvider}, nil
}

// SaveSession encrypts and saves session data to a file.
func (e *EncryptedFileStore) SaveSession(_ context.Context, id string, data *ports.SessionData) error {
	if id == "" {
		return errors.New("session id cannot be empty")
	}
	if data == nil {
		return errors.New("session data cannot be nil")
	}

	key, err := e.keyProvider.GetKey(context.Background())
	if err != nil {
		return fmt.Errorf("get encryption key: %w", err)
	}

	// Serialize the session data
	plaintext, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	// Encrypt using AES-256-GCM
	ciphertext, err := encrypt(key, plaintext)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	// Write encrypted data
	if err := os.WriteFile(e.path(id), ciphertext, 0o600); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// LoadSession reads and decrypts session data from a file.
func (e *EncryptedFileStore) LoadSession(_ context.Context, id string) (*ports.SessionData, error) {
	if id == "" {
		return nil, errors.New("session id cannot be empty")
	}

	key, err := e.keyProvider.GetKey(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get encryption key: %w", err)
	}

	// Read encrypted data
	ciphertext, err := os.ReadFile(e.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ports.ErrNotFound{ID: id}
		}
		return nil, fmt.Errorf("read session file: %w", err)
	}

	// Decrypt
	plaintext, err := decrypt(key, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt session: %w", err)
	}

	// Deserialize
	var data ports.SessionData
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &data, nil
}

// ListSessions returns all session IDs (files with .enc extension).
func (e *EncryptedFileStore) ListSessions(_ context.Context) ([]string, error) {
	entries, err := os.ReadDir(e.dir)
	if err != nil {
		return nil, fmt.Errorf("read session dir: %w", err)
	}
	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".enc") {
			ids = append(ids, strings.TrimSuffix(entry.Name(), ".enc"))
		}
	}
	return ids, nil
}

// DeleteSession removes an encrypted session file.
func (e *EncryptedFileStore) DeleteSession(_ context.Context, id string) error {
	if id == "" {
		return errors.New("session id cannot be empty")
	}
	err := os.Remove(e.path(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (e *EncryptedFileStore) path(id string) string {
	// Sanitize: strip path separators and traversal sequences.
	clean := filepath.Base(id)
	if clean == "." || clean == ".." {
		clean = "_invalid_"
	}
	return filepath.Join(e.dir, clean+".enc")
}

// encrypt encrypts plaintext using AES-256-GCM.
// Returns nonce (12 bytes) || ciphertext || auth tag (16 bytes).
func encrypt(key []byte, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	// nonce must be unique per encryption - generate random 12 bytes
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt and append auth tag automatically
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decrypt decrypts ciphertext that was encrypted with encrypt().
// Verifies auth tag - returns error if data was tampered with.
func decrypt(key []byte, ciphertext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short - missing nonce")
	}

	// Split nonce and ciphertext+tag
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt and verify auth tag - will fail if tampered
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt (data may be corrupted or tampered): %w", err)
	}
	return plaintext, nil
}
