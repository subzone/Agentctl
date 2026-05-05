package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/subzone/Agentctl/internal/adapters"
	"github.com/subzone/Agentctl/internal/engine"
	"github.com/subzone/Agentctl/internal/llm"
	"github.com/subzone/Agentctl/internal/ports"
	"github.com/subzone/Agentctl/internal/userconfig"
)

// keychainKeyProvider fetches the session encryption key from the OS keychain.
// Falls back to generating and storing a new key on first use.
type keychainKeyProvider struct{}

const sessionKeyService = "session-encryption"

func (k *keychainKeyProvider) GetKey(_ context.Context) ([]byte, error) {
	raw, err := userconfig.GetAPIKey(userconfig.Provider(sessionKeyService))
	if err == nil && raw != "" {
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	// Generate a new key and store it.
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate session key: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	if err := userconfig.SaveAPIKey(userconfig.Provider(sessionKeyService), encoded); err != nil {
		return nil, fmt.Errorf("save session key to keychain: %w", err)
	}
	return key, nil
}

// sessionStore returns the encrypted file store for sessions.
func sessionStore() (*adapters.EncryptedFileStore, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(base, "m", "sessions")
	return adapters.NewEncryptedFileStore(dir, &keychainKeyProvider{})
}

// sessionID generates a human-readable session ID from timestamp + name.
func sessionID(name string) string {
	ts := time.Now().Format("2006-01-02_15-04")
	if name != "" {
		return ts + "_" + name
	}
	return ts
}

// autoSaveID is the fixed session ID used for autosave.
const autoSaveID = "_autosave"
const maxAutoSaves = 10

// autoSaveSnapshot persists a pre-built snapshot. Rotates old autosaves
// so the last N sessions are preserved.
func autoSaveSnapshot(data *ports.SessionData) {
	if len(data.Messages) == 0 {
		return
	}
	store, err := sessionStore()
	if err != nil {
		return
	}
	// Rotate: rename current _autosave to timestamped backup before overwriting.
	ctx := context.Background()
	if existing, lerr := store.LoadSession(ctx, autoSaveID); lerr == nil && len(existing.Messages) > 0 {
		backupID := "_auto_" + time.Now().Format("2006-01-02_15-04-05")
		_ = store.SaveSession(ctx, backupID, existing)
		pruneOldAutoSaves(store)
	}
	_ = store.SaveSession(ctx, autoSaveID, data)
}

// pruneOldAutoSaves keeps only the most recent maxAutoSaves auto-backups.
func pruneOldAutoSaves(store *adapters.EncryptedFileStore) {
	ids, err := store.ListSessions(context.Background())
	if err != nil {
		return
	}
	var autos []string
	for _, id := range ids {
		if strings.HasPrefix(id, "_auto_") {
			autos = append(autos, id)
		}
	}
	if len(autos) <= maxAutoSaves {
		return
	}
	// Sort ascending (oldest first) — filenames are timestamped so lexical sort works.
	sort.Strings(autos)
	for _, id := range autos[:len(autos)-maxAutoSaves] {
		_ = store.DeleteSession(context.Background(), id)
	}
}

// exportSession converts engine state to a persistable SessionData.
func exportSession(sess *engine.Session, provider, model string) *ports.SessionData {
	msgs := sess.Messages()
	usage := sess.Usage()
	out := &ports.SessionData{
		Provider:     provider,
		Model:        model,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		Messages:     make([]ports.Message, len(msgs)),
	}
	for i, m := range msgs {
		pm := ports.Message{Role: string(m.Role)}
		for _, b := range m.Content {
			pb := ports.ContentBlock{Type: string(b.Type), Text: b.Text}
			pb.ToolID = b.ToolID
			pb.ToolName = b.ToolName
			if len(b.ToolInput) > 0 {
				pb.ToolInput = string(b.ToolInput)
			}
			pb.ToolUseID = b.ToolUseID
			pb.Output = b.Output
			pb.IsError = b.IsError
			pm.Content = append(pm.Content, pb)
		}
		out.Messages[i] = pm
	}
	return out
}

// importSession converts stored SessionData back to engine messages.
func importSession(data *ports.SessionData) []llm.Message {
	msgs := make([]llm.Message, len(data.Messages))
	for i, pm := range data.Messages {
		m := llm.Message{Role: llm.Role(pm.Role)}
		for _, pb := range pm.Content {
			b := llm.ContentBlock{
				Type:      llm.BlockType(pb.Type),
				Text:      pb.Text,
				ToolID:    pb.ToolID,
				ToolName:  pb.ToolName,
				ToolUseID: pb.ToolUseID,
				Output:    pb.Output,
				IsError:   pb.IsError,
			}
			if pb.ToolInput != "" {
				b.ToolInput = json.RawMessage(pb.ToolInput)
			}
			m.Content = append(m.Content, b)
		}
		msgs[i] = m
	}
	return msgs
}
