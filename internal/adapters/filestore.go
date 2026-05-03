package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/subzone/Agentctl/internal/ports"
)

// FileStore persists sessions as JSON files in a directory.
type FileStore struct {
	dir string
}

// NewFileStore creates a FileStore. Creates the directory if needed.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	return &FileStore{dir: dir}, nil
}

func (f *FileStore) SaveSession(_ context.Context, id string, data *ports.SessionData) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f.path(id), b, 0o600)
}

func (f *FileStore) LoadSession(_ context.Context, id string) (*ports.SessionData, error) {
	b, err := os.ReadFile(f.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ports.ErrNotFound{ID: id}
		}
		return nil, err
	}
	var data ports.SessionData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (f *FileStore) ListSessions(_ context.Context) ([]string, error) {
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			ids = append(ids, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return ids, nil
}

func (f *FileStore) DeleteSession(_ context.Context, id string) error {
	err := os.Remove(f.path(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (f *FileStore) path(id string) string {
	return filepath.Join(f.dir, id+".json")
}
