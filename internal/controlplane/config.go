package controlplane

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/subzone/Agentctl/internal/entitlement"
)

// Config configures the control plane HTTP server.
type Config struct {
	Addr          string
	WebhookSecret string
	SigningKey    ed25519.PrivateKey
	Version       string
}

// LoadConfig reads server config from environment.
func LoadConfig() (Config, error) {
	addr := strings.TrimSpace(os.Getenv("AGENTCTL_CP_ADDR"))
	if addr == "" {
		addr = ":8090"
	}
	secret := strings.TrimSpace(os.Getenv("AGENTCTL_FREEMIUS_WEBHOOK_SECRET"))
	if secret == "" {
		secret = "dev-webhook-secret" // #nosec G101 -- sandbox default; override in production
	}
	priv, err := loadSigningKey()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Addr:          addr,
		WebhookSecret: secret,
		SigningKey:    priv,
		Version:       "0.1.0",
	}, nil
}

func loadSigningKey() (ed25519.PrivateKey, error) {
	if hexKey := strings.TrimSpace(os.Getenv("AGENTCTL_CP_SIGNING_KEY")); hexKey != "" {
		return decodePrivateKeyHex(hexKey)
	}
	return entitlement.DevPrivateKey()
}

func decodePrivateKeyHex(hexKey string) (ed25519.PrivateKey, error) {
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, err
	}
	switch len(b) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(b), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(b), nil
	default:
		return nil, fmt.Errorf("invalid signing key length")
	}
}

// IssueToken signs an entitlement JWT for a license record.
func IssueToken(rec *LicenseRecord, key ed25519.PrivateKey) (string, entitlement.State, error) {
	if rec == nil {
		return "", entitlement.State{}, fmt.Errorf("nil license")
	}
	claims := entitlement.Claims{
		Plan:         rec.Plan,
		Entitlements: append([]string(nil), rec.Entitlements...),
		Ver:          rec.Version,
		Subject:      rec.Subject,
	}
	token, err := entitlement.SignClaims(claims, key, 30*24*time.Hour)
	if err != nil {
		return "", entitlement.State{}, err
	}
	state, err := entitlement.ApplyToken(token)
	if err != nil {
		return "", entitlement.State{}, err
	}
	state.LicenseHint = maskLicense(rec.LicenseKey)
	return token, state, nil
}

func maskLicense(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "…" + key[len(key)-4:]
}

// FreemiusWebhookEvent is the sandbox webhook payload shape.
type FreemiusWebhookEvent struct {
	EventID    string `json:"event_id,omitempty"`
	Event      string `json:"event"`
	LicenseKey string `json:"license_key"`
	Plan       string `json:"plan"`
	UserID     string `json:"user_id"`
	Email      string `json:"email"`
}

// ApplyFreemiusEvent updates license state from a webhook (idempotent).
func ApplyFreemiusEvent(store *Store, ev FreemiusWebhookEvent) error {
	key := strings.TrimSpace(ev.LicenseKey)
	if key == "" {
		return fmt.Errorf("license_key required")
	}
	id := strings.TrimSpace(ev.EventID)
	if id == "" {
		id = ev.Event + ":" + key
	}
	if !store.MarkWebhookSeen(id) {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(ev.Event)) {
	case "license.revoked", "subscription.cancelled", "payment.failed":
		store.RevokeLicense(key)
		return nil
	case "license.activated", "subscription.created", "subscription.renewed":
		plan := entitlement.Plan(strings.ToLower(strings.TrimSpace(ev.Plan)))
		if plan == "" {
			plan = entitlement.PlanPro
		}
		subject := strings.TrimSpace(ev.UserID)
		if subject == "" {
			subject = strings.TrimSpace(ev.Email)
		}
		if subject == "" {
			subject = "user_" + key
		}
		store.UpsertLicense(LicenseRecord{
			LicenseKey:   key,
			Plan:         plan,
			Subject:      subject,
			Entitlements: entitlement.State{Plan: plan}.EffectiveEntitlements(),
		})
		return nil
	default:
		return fmt.Errorf("unknown event: %s", ev.Event)
	}
}

// PackageCatalog returns curated package metadata for GET /v1/packages.
func PackageCatalog() []PackageOffer {
	return []PackageOffer{
		{Name: "pro-dev", Description: "Coder, reviewer, and dev workflow agents", Entitlements: []string{"pro"}},
		{Name: "pro-security", Description: "Security review and hardening pack", Entitlements: []string{"pro"}},
	}
}

// PackageOffer is a catalog entry returned by the API.
type PackageOffer struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Entitlements []string `json:"entitlements"`
}

// EncodeJSON writes v as JSON to w.
func EncodeJSON(w interface{ Write([]byte) (int, error) }, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
