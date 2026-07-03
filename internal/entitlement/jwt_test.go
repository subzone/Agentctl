package entitlement_test

import (
	"testing"
	"time"

	"github.com/subzone/Agentctl/internal/entitlement"
)

func TestJWTRoundTrip(t *testing.T) {
	priv, err := entitlement.DevPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub, err := entitlement.DevPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	claims := entitlement.Claims{
		Plan:         entitlement.PlanPro,
		Entitlements: []string{"pro"},
		Subject:      "test_user",
		Ver:          1,
	}
	token, err := entitlement.SignClaims(claims, priv, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := entitlement.ParseToken(token, pub)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Plan != entitlement.PlanPro {
		t.Fatalf("plan: %s", parsed.Plan)
	}
	state, err := entitlement.ApplyToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if !state.HasEntitlement("pro") {
		t.Fatal("expected pro entitlement from JWT")
	}
	if state.Source != "control-plane" {
		t.Fatalf("source: %s", state.Source)
	}
}
