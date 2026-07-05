package controlplane

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/subzone/Agentctl/internal/entitlement"
)

// verifyFreemiusSignature checks the `x-signature` header Freemius sends on
// every webhook request: hex(HMAC-SHA256(rawBody, webhookSecret)).
//
// NOTE: verified against Freemius's own docs ("the SDK automatically verifies
// the authenticity of incoming webhook requests using the signature sent in
// the x-signature header"), but the exact byte encoding was not confirmed
// against a live payload. Before relying on this in production, send a real
// test event from the Freemius dashboard and diff its x-signature header
// against this computation — adjust here if it doesn't match.
func verifyFreemiusSignature(rawBody []byte, signatureHeader, secret string) bool {
	sig := strings.TrimSpace(signatureHeader)
	if sig == "" || secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

// FreemiusWebhookEvent is the normalized shape we act on, after extracting
// it from either Freemius's real nested payload or the flat sandbox shape
// used for local testing (see controlplane/README.md).
type FreemiusWebhookEvent struct {
	EventID    string
	Event      string
	LicenseKey string
	PlanID     string
	Plan       string
	UserID     string
	Email      string
}

// freemiusNestedPayload models Freemius's documented webhook shape:
// a top-level event id/type plus an "objects" map of related entities.
// Field names are best-effort — confirm against a real dashboard test event.
type freemiusNestedPayload struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Objects struct {
		License struct {
			SecretKey string     `json:"secret_key"`
			PlanID    flexString `json:"plan_id"`
		} `json:"license"`
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	} `json:"objects"`
}

// freemiusFlatPayload is the flat sandbox shape (see controlplane/README.md
// curl example) — kept for local testing without a live Freemius account.
type freemiusFlatPayload struct {
	EventID    string `json:"event_id,omitempty"`
	Event      string `json:"event"`
	LicenseKey string `json:"license_key"`
	Plan       string `json:"plan"`
	UserID     string `json:"user_id"`
	Email      string `json:"email"`
}

// parseFreemiusWebhook accepts either payload shape and normalizes it.
func parseFreemiusWebhook(raw []byte) (FreemiusWebhookEvent, error) {
	var nested freemiusNestedPayload
	if err := json.Unmarshal(raw, &nested); err == nil && nested.Type != "" && nested.Objects.License.SecretKey != "" {
		return FreemiusWebhookEvent{
			EventID:    nested.ID,
			Event:      nested.Type,
			LicenseKey: nested.Objects.License.SecretKey,
			PlanID:     nested.Objects.License.PlanID.String(),
			UserID:     nested.Objects.User.ID,
			Email:      nested.Objects.User.Email,
		}, nil
	}
	var flat freemiusFlatPayload
	if err := json.Unmarshal(raw, &flat); err != nil {
		return FreemiusWebhookEvent{}, fmt.Errorf("invalid JSON")
	}
	if flat.LicenseKey == "" {
		return FreemiusWebhookEvent{}, fmt.Errorf("license_key required")
	}
	return FreemiusWebhookEvent{
		EventID:    flat.EventID,
		Event:      flat.Event,
		LicenseKey: flat.LicenseKey,
		Plan:       flat.Plan,
		UserID:     flat.UserID,
		Email:      flat.Email,
	}, nil
}

// ApplyFreemiusEvent updates license state from a webhook (idempotent).
func ApplyFreemiusEvent(store *Store, ev FreemiusWebhookEvent, planMap map[string]entitlement.Plan) error {
	key := strings.TrimSpace(ev.LicenseKey)
	if key == "" {
		return fmt.Errorf("license_key required")
	}
	id := strings.TrimSpace(ev.EventID)
	if id == "" {
		id = ev.Event + ":" + key
	}
	seen, err := store.MarkWebhookSeen(id)
	if err != nil {
		return err
	}
	if !seen {
		return nil
	}
	// Event names and the "objects.license" nesting confirmed against the
	// real event catalog and JS SDK docs for the AgentCTL product
	// (dashboard.freemius.com/.../webhooks/listeners) — not the generic
	// "subscription.cancelled"/"payment.failed" names originally guessed.
	switch strings.ToLower(strings.TrimSpace(ev.Event)) {
	case "license.cancelled", "license.deactivated", "license.deleted", "license.expired",
		"subscription.cancelled", "subscription.renewal.failed.last":
		return store.RevokeLicense(key)
	case "license.activated", "license.created", "subscription.created":
		plan := resolvePlan(ev, planMap)
		subject := strings.TrimSpace(ev.UserID)
		if subject == "" {
			subject = strings.TrimSpace(ev.Email)
		}
		if subject == "" {
			subject = "user_" + key
		}
		return store.UpsertLicense(LicenseRecord{
			LicenseKey:   key,
			Plan:         plan,
			Subject:      subject,
			Entitlements: entitlement.State{Plan: plan}.EffectiveEntitlements(),
		})
	case "license.activations.synced", "license.blacklisted_site.deleted":
		// Selected on the webhook listener but not entitlement-changing —
		// acknowledge without acting so Freemius doesn't see a failed delivery.
		return nil
	default:
		return fmt.Errorf("unknown event: %s", ev.Event)
	}
}

func resolvePlan(ev FreemiusWebhookEvent, planMap map[string]entitlement.Plan) entitlement.Plan {
	if ev.PlanID != "" {
		if p, ok := planMap[ev.PlanID]; ok {
			return p
		}
	}
	if ev.Plan != "" {
		return entitlement.Plan(strings.ToLower(strings.TrimSpace(ev.Plan)))
	}
	return entitlement.PlanPro
}
