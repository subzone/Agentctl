package controlplane

import (
	"testing"

	"github.com/subzone/Agentctl/internal/entitlement"
)

func TestParseFreemiusWebhook_nestedNumericPlanID(t *testing.T) {
	raw := []byte(`{
		"id": "evt_1",
		"type": "license.created",
		"objects": {
			"license": {"secret_key": "FS-LIVE-001", "plan_id": 55088},
			"user": {"id": "user_9", "email": "buyer@example.com"}
		}
	}`)
	ev, err := parseFreemiusWebhook(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ev.LicenseKey != "FS-LIVE-001" {
		t.Fatalf("license key: %q", ev.LicenseKey)
	}
	if ev.PlanID != "55088" {
		t.Fatalf("plan id: %q", ev.PlanID)
	}
	plan := resolvePlan(ev, map[string]entitlement.Plan{"55088": entitlement.PlanPro})
	if plan != entitlement.PlanPro {
		t.Fatalf("plan: %q", plan)
	}
}

func TestParseFreemiusWebhook_flatSandbox(t *testing.T) {
	raw := []byte(`{
		"event_id": "evt_demo",
		"event": "license.activated",
		"license_key": "FS-CUSTOM-001",
		"plan": "pro",
		"user_id": "user_123"
	}`)
	ev, err := parseFreemiusWebhook(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ev.LicenseKey != "FS-CUSTOM-001" || ev.Plan != "pro" {
		t.Fatalf("unexpected event: %+v", ev)
	}
}
