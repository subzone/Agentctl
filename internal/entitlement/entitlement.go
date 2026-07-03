// Package entitlement manages local plan state and package access gates.
// Production builds will verify signed JWTs from the control plane; until
// then, dev license keys activate plans offline.
package entitlement

import (
	"fmt"
	"strings"
	"time"
)

// Plan identifies the subscription tier.
type Plan string

const (
	PlanFree       Plan = "free"
	PlanPro        Plan = "pro"
	PlanTeam       Plan = "team"
	PlanEnterprise Plan = "enterprise"
)

// State is persisted under ~/.config/m/entitlement.json.
type State struct {
	Plan         Plan     `json:"plan"`
	Packages     []string `json:"packages,omitempty"`
	Entitlements []string `json:"entitlements,omitempty"`
	Subject      string   `json:"subject,omitempty"`
	LicenseHint  string   `json:"licenseHint,omitempty"`
	ActivatedAt  int64    `json:"activatedAt,omitempty"`
	ExpiresAt    int64    `json:"expiresAt,omitempty"`
	Source       string   `json:"source,omitempty"` // dev-key | control-plane
	Token        string   `json:"token,omitempty"`  // signed JWT from control plane
}

// Default returns the free-tier state.
func Default() State {
	return State{Plan: PlanFree, Entitlements: []string{"free"}}
}

// EffectiveEntitlements returns entitlement keys granted by the current plan.
func (s State) EffectiveEntitlements() []string {
	seen := map[string]bool{}
	var out []string
	add := func(keys ...string) {
		for _, k := range keys {
			k = strings.TrimSpace(strings.ToLower(k))
			if k == "" || seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, k)
		}
	}
	add(s.Entitlements...)
	add(planEntitlements(s.Plan)...)
	add(s.Packages...) // installed package names also grant access to their content
	return out
}

func planEntitlements(p Plan) []string {
	switch p {
	case PlanPro:
		return []string{"free", "pro"}
	case PlanTeam:
		return []string{"free", "pro", "team"}
	case PlanEnterprise:
		return []string{"free", "pro", "team", "enterprise"}
	default:
		return []string{"free"}
	}
}

// HasEntitlement reports whether the user can use a required entitlement key.
func (s State) HasEntitlement(required string) bool {
	required = strings.TrimSpace(strings.ToLower(required))
	if required == "" {
		return true
	}
	for _, e := range s.EffectiveEntitlements() {
		if e == required {
			return true
		}
	}
	return false
}

// HasAllEntitlements returns missing keys (empty slice = ok).
func (s State) HasAllEntitlements(required []string) []string {
	var missing []string
	for _, r := range required {
		r = strings.TrimSpace(strings.ToLower(r))
		if r == "" {
			continue
		}
		if !s.HasEntitlement(r) {
			missing = append(missing, r)
		}
	}
	return missing
}

// IsExpired reports whether the license has passed ExpiresAt.
func (s State) IsExpired() bool {
	if s.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() > s.ExpiresAt
}

// ActivateDevKey maps offline dev keys to a plan (until control plane ships).
func ActivateDevKey(key string) (State, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return State{}, fmt.Errorf("license key required")
	}
	upper := strings.ToUpper(key)
	now := time.Now().Unix()
	exp := time.Now().AddDate(1, 0, 0).Unix()

	switch upper {
	case "AGENTCTL-PRO-DEV-2026", "AGENTCTL-PRO-TRIAL":
		return State{
			Plan:         PlanPro,
			Entitlements: []string{"pro"},
			LicenseHint:  maskKey(key),
			ActivatedAt:  now,
			ExpiresAt:    exp,
			Source:       "dev-key",
		}, nil
	case "AGENTCTL-TEAM-DEV-2026":
		return State{
			Plan:         PlanTeam,
			Entitlements: []string{"pro", "team"},
			LicenseHint:  maskKey(key),
			ActivatedAt:  now,
			ExpiresAt:    exp,
			Source:       "dev-key",
		}, nil
	case "AGENTCTL-ENTERPRISE-DEV-2026":
		return State{
			Plan:         PlanEnterprise,
			Entitlements: []string{"pro", "team", "enterprise"},
			LicenseHint:  maskKey(key),
			ActivatedAt:  now,
			ExpiresAt:    exp,
			Source:       "dev-key",
		}, nil
	default:
		return State{}, fmt.Errorf("invalid or unknown license key (use AGENTCTL-PRO-DEV-2026 for local testing)")
	}
}

func maskKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "…" + key[len(key)-4:]
}
