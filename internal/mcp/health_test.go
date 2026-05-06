package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestManagerHealthCheck(t *testing.T) {
	// Create a mock transport that responds to ListTools
	mock := &mockTransport{
		tools: []Tool{{Name: "test_tool"}},
	}

	m := &Manager{
		clients: []namedTransport{
			{name: "server1", transport: mock},
		},
	}

	ctx := context.Background()
	results := m.HealthCheck(ctx)

	if len(results) != 1 {
		t.Errorf("HealthCheck returned %d results, want 1", len(results))
	}

	if results["server1"] != nil {
		t.Errorf("server1 should be healthy, got error: %v", results["server1"])
	}
}

func TestManagerHealthCheckTimeout(t *testing.T) {
	// Create a mock transport that blocks forever
	mock := &mockTransport{
		blockForever: true,
	}

	m := &Manager{
		clients: []namedTransport{
			{name: "slow-server", transport: mock},
		},
	}

	ctx := context.Background()
	results := m.HealthCheck(ctx)

	if len(results) != 1 {
		t.Errorf("HealthCheck returned %d results, want 1", len(results))
	}

	if results["slow-server"] == nil {
		t.Error("slow-server should have timeout error, got nil")
	}
}

func TestManagerHealthCheckEmpty(t *testing.T) {
	m := &Manager{clients: nil}

	ctx := context.Background()
	results := m.HealthCheck(ctx)

	if len(results) != 0 {
		t.Errorf("HealthCheck on empty manager returned %d results, want 0", len(results))
	}
}

func TestManagerHealthCheckMultiple(t *testing.T) {
	m := &Manager{
		clients: []namedTransport{
			{name: "healthy1", transport: &mockTransport{tools: []Tool{{Name: "t1"}}}},
			{name: "healthy2", transport: &mockTransport{tools: []Tool{{Name: "t2"}}}},
			{name: "unhealthy", transport: &mockTransport{listError: true}},
		},
	}

	ctx := context.Background()
	results := m.HealthCheck(ctx)

	if len(results) != 3 {
		t.Errorf("HealthCheck returned %d results, want 3", len(results))
	}

	if results["healthy1"] != nil {
		t.Errorf("healthy1 should be healthy: %v", results["healthy1"])
	}
	if results["healthy2"] != nil {
		t.Errorf("healthy2 should be healthy: %v", results["healthy2"])
	}
	if results["unhealthy"] == nil {
		t.Error("unhealthy should have error")
	}
}

// mockTransport implements Transport for testing
type mockTransport struct {
	tools        []Tool
	blockForever bool
	listError    bool
}

func (m *mockTransport) Initialize(ctx context.Context, name, version string) error {
	return nil
}

func (m *mockTransport) ListTools(ctx context.Context) ([]Tool, error) {
	if m.listError {
		return nil, context.DeadlineExceeded
	}
	if m.blockForever {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return nil, nil
		}
	}
	return m.tools, nil
}

func (m *mockTransport) CallTool(ctx context.Context, name string, args json.RawMessage) (string, bool, error) {
	return "ok", false, nil
}

func (m *mockTransport) Close() error {
	return nil
}

func BenchmarkHealthCheck(b *testing.B) {
	m := &Manager{
		clients: []namedTransport{
			{name: "s1", transport: &mockTransport{tools: []Tool{{Name: "t"}}}},
			{name: "s2", transport: &mockTransport{tools: []Tool{{Name: "t"}}}},
			{name: "s3", transport: &mockTransport{tools: []Tool{{Name: "t"}}}},
		},
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.HealthCheck(ctx)
	}
}