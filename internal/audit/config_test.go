package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigMergesFiles(t *testing.T) {
	d := t.TempDir()
	home := filepath.Join(d, "home.yaml")
	proj := filepath.Join(d, "proj.yaml")
	if err := os.WriteFile(home, []byte("audit:\n  backend: file\n  path: /tmp/home.jsonl\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proj, []byte("audit:\n  path: /tmp/project.jsonl\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(home, proj)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Backend != "file" {
		t.Fatalf("backend=%q want file", cfg.Backend)
	}
	if cfg.Path != "/tmp/project.jsonl" {
		t.Fatalf("path=%q want /tmp/project.jsonl", cfg.Path)
	}
}

func TestLoadConfigSplunkNested(t *testing.T) {
	d := t.TempDir()
	home := filepath.Join(d, "home.yaml")
	if err := os.WriteFile(home, []byte("audit:\n  backend: splunk\n  splunk:\n    endpoint: https://splunk.example\n    token: ${SPLUNK_TOKEN}\n    tls_verify: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SPLUNK_TOKEN", "tok-xyz")
	cfg, err := LoadConfig(home)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Backend != "splunk" {
		t.Fatalf("backend=%q want splunk", cfg.Backend)
	}
	if cfg.Endpoint != "https://splunk.example" {
		t.Fatalf("endpoint=%q", cfg.Endpoint)
	}
	if cfg.Token != "tok-xyz" {
		t.Fatalf("token=%q", cfg.Token)
	}
	if cfg.TLSVerify {
		t.Fatal("tls_verify should be false")
	}
}

func TestLoadConfigBatching(t *testing.T) {
	d := t.TempDir()
	home := filepath.Join(d, "home.yaml")
	if err := os.WriteFile(home, []byte("audit:\n  backend: file\n  batch_size: 20\n  flush_interval: 5s\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(home)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BatchSize != 20 {
		t.Fatalf("batch size=%d", cfg.BatchSize)
	}
	if cfg.FlushInterval != 5*time.Second {
		t.Fatalf("flush interval=%v", cfg.FlushInterval)
	}
}

func TestExpandEnvToken(t *testing.T) {
	t.Setenv("AUDIT_TEST_SECRET", "abc123")
	if got := expandEnvToken("${AUDIT_TEST_SECRET}"); got != "abc123" {
		t.Fatalf("got %q", got)
	}
}
