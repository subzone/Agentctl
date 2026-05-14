package audit

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/subzone/Agentctl/internal/ports"
)

type fileSink struct {
	mu        sync.Mutex
	f         *os.File
	w         *bufio.Writer
	hmacKey   []byte
	closeOnce sync.Once
}

func NewFileSink(path string, hmacSecret string) (ports.AuditSink, error) {
	if path == "" {
		return nil, fmt.Errorf("audit file path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &fileSink{
		f:       f,
		w:       bufio.NewWriterSize(f, 16*1024),
		hmacKey: []byte(hmacSecret),
	}, nil
}

func (s *fileSink) Emit(_ context.Context, event ports.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if event.EventID == "" {
		event.EventID = randomID()
	}
	if event.TS.IsZero() {
		event.TS = time.Now().UTC()
	}

	unsigned := event
	unsigned.HMAC = ""
	rawUnsigned, err := json.Marshal(unsigned)
	if err != nil {
		return err
	}
	if len(s.hmacKey) > 0 {
		h := hmac.New(sha256.New, s.hmacKey)
		h.Write(rawUnsigned)
		event.HMAC = "sha256:" + hex.EncodeToString(h.Sum(nil))
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := s.w.Write(raw); err != nil {
		return err
	}
	if err := s.w.WriteByte('\n'); err != nil {
		return err
	}
	return s.w.Flush()
}

func (s *fileSink) Flush(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Flush()
}

func (s *fileSink) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.w.Flush(); err != nil {
			closeErr = err
		}
		if err := s.f.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
