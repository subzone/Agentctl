package audit

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/subzone/Agentctl/internal/ports"
)

type splunkSink struct {
	endpoint string
	token    string
	client   *http.Client
}

func NewSplunkSink(endpoint string, token string, tlsVerify bool) (ports.AuditSink, error) {
	endpoint = strings.TrimSpace(endpoint)
	token = strings.TrimSpace(token)
	if endpoint == "" {
		return nil, fmt.Errorf("splunk endpoint is required")
	}
	if token == "" {
		return nil, fmt.Errorf("splunk token is required")
	}
	if !strings.Contains(endpoint, "/services/collector") {
		endpoint = strings.TrimRight(endpoint, "/") + "/services/collector"
	}

	tr := &http.Transport{}
	if !tlsVerify {
		// #nosec G402 -- Optional opt-out for self-signed/local Splunk endpoints via config.
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &splunkSink{
		endpoint: endpoint,
		token:    token,
		client: &http.Client{
			Timeout:   8 * time.Second,
			Transport: tr,
		},
	}, nil
}

func (s *splunkSink) Emit(ctx context.Context, event ports.AuditEvent) error {
	if event.TS.IsZero() {
		event.TS = time.Now().UTC()
	}
	payload := map[string]any{
		"time":       float64(event.TS.UnixNano()) / 1e9,
		"host":       hostName(),
		"source":     "agentctl",
		"sourcetype": "agentctl:audit",
		"event":      event,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Splunk "+s.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("splunk hec status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func (s *splunkSink) Flush(_ context.Context) error { return nil }
func (s *splunkSink) Close() error                  { return nil }

func hostName() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}
