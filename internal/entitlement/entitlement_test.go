package entitlement_test

import (
	"testing"

	"github.com/subzone/Agentctl/internal/entitlement"
)

func TestActivateDevKeyPro(t *testing.T) {
	s, err := entitlement.ActivateDevKey("AGENTCTL-PRO-DEV-2026")
	if err != nil {
		t.Fatal(err)
	}
	if s.Plan != entitlement.PlanPro {
		t.Fatalf("plan: %s", s.Plan)
	}
	if !s.HasEntitlement("pro") {
		t.Fatal("expected pro entitlement")
	}
	if s.HasEntitlement("team") {
		t.Fatal("pro should not include team")
	}
}

func TestHasAllEntitlements(t *testing.T) {
	s := entitlement.Default()
	missing := s.HasAllEntitlements([]string{"pro"})
	if len(missing) != 1 || missing[0] != "pro" {
		t.Fatalf("missing: %v", missing)
	}
}
