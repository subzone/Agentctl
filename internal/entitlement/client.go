package entitlement

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultControlPlaneURL = "http://localhost:8090"

// ControlPlaneURL returns AGENTCTL_CONTROL_PLANE_URL or the local dev default when unset.
func ControlPlaneURL() string {
	if u := strings.TrimSpace(os.Getenv("AGENTCTL_CONTROL_PLANE_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	return ""
}

type activateRequest struct {
	LicenseKey string `json:"license_key"`
	MachineID  string `json:"machine_id,omitempty"`
}

type entitlementResponse struct {
	Token        string   `json:"token"`
	Plan         string   `json:"plan"`
	Entitlements []string `json:"entitlements"`
	Packages     []string `json:"packages"`
	Subject      string   `json:"subject"`
	ExpiresAt    string   `json:"expires_at"`
}

// Activate tries offline dev keys first, then the control plane when configured.
func Activate(key string) (State, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return State{}, fmt.Errorf("license key required")
	}
	if strings.HasPrefix(strings.ToUpper(key), "AGENTCTL-") {
		if s, err := ActivateDevKey(key); err == nil {
			return s, nil
		}
	}
	base := ControlPlaneURL()
	if base == "" {
		return ActivateDevKey(key)
	}
	return activateRemote(base, key)
}

// Refresh exchanges a cached JWT for a new one from the control plane.
func Refresh() (State, error) {
	base := ControlPlaneURL()
	if base == "" {
		return State{}, fmt.Errorf("set AGENTCTL_CONTROL_PLANE_URL to refresh entitlements")
	}
	s, err := Load()
	if err != nil {
		return State{}, err
	}
	if strings.TrimSpace(s.Token) == "" {
		return State{}, fmt.Errorf("no control-plane token cached — run: m license activate <key>")
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/auth/refresh", nil)
	if err != nil {
		return State{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return State{}, fmt.Errorf("control plane refresh: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return State{}, readAPIError(resp)
	}
	return decodeEntitlementResponse(resp.Body)
}

func activateRemote(base, key string) (State, error) {
	body, err := json.Marshal(activateRequest{LicenseKey: key})
	if err != nil {
		return State{}, err
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/auth/freemius/activate", bytes.NewReader(body))
	if err != nil {
		return State{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return State{}, fmt.Errorf("control plane activate: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return State{}, readAPIError(resp)
	}
	return decodeEntitlementResponse(resp.Body)
}

func decodeEntitlementResponse(r io.Reader) (State, error) {
	var out entitlementResponse
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return State{}, err
	}
	if strings.TrimSpace(out.Token) == "" {
		return State{}, fmt.Errorf("control plane returned empty token")
	}
	s, err := ApplyToken(out.Token)
	if err != nil {
		return State{}, err
	}
	if out.Subject != "" {
		s.Subject = out.Subject
	}
	if out.Plan != "" {
		s.Plan = Plan(strings.ToLower(out.Plan))
	}
	if len(out.Entitlements) > 0 {
		s.Entitlements = out.Entitlements
	}
	if len(out.Packages) > 0 {
		s.Packages = out.Packages
	}
	if s.LicenseHint == "" {
		s.LicenseHint = "control-plane"
	}
	return s, nil
}

func readAPIError(resp *http.Response) error {
	var payload struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	if payload.Error != "" {
		return fmt.Errorf("control plane: %s", payload.Error)
	}
	return fmt.Errorf("control plane: HTTP %d", resp.StatusCode)
}

// LocalControlPlaneURL is the default URL used by `m controlplane serve`.
func LocalControlPlaneURL() string {
	return defaultControlPlaneURL
}
