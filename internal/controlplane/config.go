package controlplane

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/subzone/Agentctl/internal/entitlement"
)

// Config configures the control plane HTTP server.
type Config struct {
	Addr          string
	Env           string // "dev" (default) or "production"
	DBPath        string
	WebhookSecret string
	SigningKey    ed25519.PrivateKey
	FreemiusPlans map[string]entitlement.Plan // Freemius plan ID -> our plan name
	Version       string
}

// IsProduction reports whether the server is running in production mode,
// which disables sandbox license seeding and requires an explicit signing key.
func (c Config) IsProduction() bool {
	return strings.EqualFold(strings.TrimSpace(c.Env), "production")
}

// LoadConfig reads server config from environment.
func LoadConfig() (Config, error) {
	env := strings.TrimSpace(os.Getenv("AGENTCTL_CP_ENV"))
	if env == "" {
		env = "dev"
	}
	addr := strings.TrimSpace(os.Getenv("AGENTCTL_CP_ADDR"))
	if addr == "" {
		addr = ":8090"
	}
	dbPath := strings.TrimSpace(os.Getenv("AGENTCTL_CP_DB_PATH"))
	if dbPath == "" {
		dbPath = "controlplane.db"
	}
	secret := strings.TrimSpace(os.Getenv("AGENTCTL_FREEMIUS_WEBHOOK_SECRET"))
	if secret == "" {
		if strings.EqualFold(env, "production") {
			return Config{}, fmt.Errorf("AGENTCTL_FREEMIUS_WEBHOOK_SECRET is required when AGENTCTL_CP_ENV=production")
		}
		secret = "dev-webhook-secret" // #nosec G101 -- sandbox default; overridden in production
	}
	priv, err := loadSigningKey(env)
	if err != nil {
		return Config{}, err
	}
	plans, err := loadFreemiusPlanMap()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Addr:          addr,
		Env:           env,
		DBPath:        dbPath,
		WebhookSecret: secret,
		SigningKey:    priv,
		FreemiusPlans: plans,
		Version:       "0.1.0",
	}, nil
}

func loadSigningKey(env string) (ed25519.PrivateKey, error) {
	hexKey := strings.TrimSpace(os.Getenv("AGENTCTL_CP_SIGNING_KEY"))
	if hexKey == "" {
		if strings.EqualFold(env, "production") {
			return nil, fmt.Errorf("AGENTCTL_CP_SIGNING_KEY is required when AGENTCTL_CP_ENV=production")
		}
		log.Printf("AGENTCTL_CP_SIGNING_KEY not set — signing entitlements with the public dev key (sandbox only)")
		return entitlement.DevPrivateKey()
	}
	return decodePrivateKeyHex(hexKey)
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

// loadFreemiusPlanMap reads AGENTCTL_FREEMIUS_PLAN_MAP, a JSON object mapping
// Freemius plan IDs (as they appear in webhook payloads) to our plan names,
// e.g. {"12345":"pro","12346":"team"}. Populate this once the real Freemius
// product/plans exist and their numeric IDs are known.
func loadFreemiusPlanMap() (map[string]entitlement.Plan, error) {
	raw := strings.TrimSpace(os.Getenv("AGENTCTL_FREEMIUS_PLAN_MAP"))
	out := map[string]entitlement.Plan{}
	if raw == "" {
		return out, nil
	}
	var flat map[string]string
	if err := json.Unmarshal([]byte(raw), &flat); err != nil {
		return nil, fmt.Errorf("invalid AGENTCTL_FREEMIUS_PLAN_MAP: %w", err)
	}
	for k, v := range flat {
		out[k] = entitlement.Plan(strings.ToLower(strings.TrimSpace(v)))
	}
	return out, nil
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
