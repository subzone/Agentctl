package controlplane_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/subzone/Agentctl/internal/controlplane"
	"github.com/subzone/Agentctl/internal/entitlement"
)

func TestActivateSandboxLicense(t *testing.T) {
	cfg, err := controlplane.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	srv, err := controlplane.NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"license_key": "FS-PRO-SANDBOX-2026"})
	resp, err := http.Post(ts.URL+"/v1/auth/freemius/activate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var out struct {
		Token string `json:"token"`
		Plan  string `json:"plan"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Plan != "pro" || out.Token == "" {
		t.Fatalf("unexpected response: %+v", out)
	}
	state, err := entitlement.ApplyToken(out.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !state.HasEntitlement("pro") {
		t.Fatal("expected pro from sandbox license")
	}
}

func TestFreemiusWebhookCreatesLicense(t *testing.T) {
	cfg, err := controlplane.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	srv, err := controlplane.NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ev := map[string]string{
		"event_id":    "evt_1",
		"event":       "license.activated",
		"license_key": "FS-CUSTOM-TEST",
		"plan":        "team",
		"user_id":     "user_42",
	}
	raw, _ := json.Marshal(ev)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/webhooks/freemius", bytes.NewReader(raw))
	req.Header.Set("X-Agentctl-Webhook-Secret", cfg.WebhookSecret)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook status %d", resp.StatusCode)
	}

	body, _ := json.Marshal(map[string]string{"license_key": "FS-CUSTOM-TEST"})
	resp2, err := http.Post(ts.URL+"/v1/auth/freemius/activate", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("activate status %d", resp2.StatusCode)
	}
}
