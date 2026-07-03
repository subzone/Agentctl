# AgentCTL Control Plane (sandbox)

Phase 1 hosted entitlement API for Freemius license sync and signed JWT entitlements.

## Run locally

```bash
# Terminal 1 — API on :8090
go run ./cmd/controlplane
# or
m controlplane serve

# Terminal 2 — activate sandbox license
export AGENTCTL_CONTROL_PLANE_URL=http://localhost:8090
m license activate FS-PRO-SANDBOX-2026
m license status
m packages install pro-dev
```

## Sandbox licenses

| Key | Plan |
|-----|------|
| `FS-PRO-SANDBOX-2026` | pro |
| `FS-TEAM-SANDBOX-2026` | team |

## Freemius webhook (sandbox)

```bash
curl -X POST http://localhost:8090/v1/webhooks/freemius \
  -H "Content-Type: application/json" \
  -H "X-Agentctl-Webhook-Secret: dev-webhook-secret" \
  -d '{
    "event_id": "evt_demo_1",
    "event": "license.activated",
    "license_key": "FS-CUSTOM-001",
    "plan": "pro",
    "user_id": "user_123"
  }'
```

Then activate with `m license activate FS-CUSTOM-001`.

## Environment

| Variable | Default | Purpose |
|----------|---------|---------|
| `AGENTCTL_CP_ADDR` | `:8090` | Listen address |
| `AGENTCTL_FREEMIUS_WEBHOOK_SECRET` | `dev-webhook-secret` | Webhook auth |
| `AGENTCTL_CP_SIGNING_KEY` | dev Ed25519 key | JWT signing (hex) |
| `AGENTCTL_CONTROL_PLANE_URL` | _(client)_ | Client API base URL |

## API contract

OpenAPI 3.1 spec: [`openapi.yaml`](openapi.yaml)

Production URL (planned): `https://api.agentctl.dev`
