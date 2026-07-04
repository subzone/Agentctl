# AgentCTL Control Plane

Phase 1 hosted entitlement API for Freemius license sync and signed JWT entitlements.

## Production

Deployed to `https://agentctl-api.myk8s.pp.ua` via ArgoCD — see
`helm/controlplane/` in this repo for the chart and
`argocd-app-of-apps/applications/agentctl-controlplane-app.yaml` +
`argocd-app-of-apps/apps/istio-config/agentctl-vs.yaml` +
`argocd-app-of-apps/apps/external-secrets-config/agentctl-controlplane-env.yaml`
in the cluster GitOps repo for the deployment.

The client (`m license activate <key>`) talks to this URL by default —
no env var needed. Override with `AGENTCTL_CONTROL_PLANE_URL` (or set it to
`-` to force offline/dev-key-only mode).

Required production secrets (populated in AWS Parameter Store, synced via
ExternalSecrets — see the `agentctl-controlplane-env.yaml` ExternalSecret):

| Parameter Store key | Env var | Purpose |
|---|---|---|
| `/agentctl-controlplane/FREEMIUS_WEBHOOK_SECRET` | `AGENTCTL_FREEMIUS_WEBHOOK_SECRET` | Freemius product's webhook "Secret Key" — verifies the `x-signature` header |
| `/agentctl-controlplane/CP_SIGNING_KEY` | `AGENTCTL_CP_SIGNING_KEY` | Ed25519 private key seed (hex) that signs entitlement JWTs — **must** be the private half of `entitlement.ProdPublicKeyHex` in `internal/entitlement/jwt.go`, or every client-side verification fails |
| `/agentctl-controlplane/GHCR_DOCKERCONFIG` | — | Pull secret for the `ghcr.io/subzone/agentctl-controlplane` image |

`AGENTCTL_CP_ENV=production` is set in `helm/controlplane/values.yaml` and
makes the server refuse to boot without the two secrets above, and skip
seeding the sandbox test licenses.

**Webhook auth**: the AgentCTL product on Freemius is a non-WordPress
("SaaS"/"Other") product, so Freemius doesn't hand out a per-webhook signing
secret the way their WordPress SDK does. Per their own docs' recommendation
for this case, the webhook URL configured in the Freemius dashboard must
include a caller-supplied secret as a query parameter:

```
https://agentctl-api.myk8s.pp.ua/v1/webhooks/freemius?token=<AGENTCTL_FREEMIUS_WEBHOOK_SECRET value>
```

Generate the token yourself (`openssl rand -hex 32`), put it in Parameter
Store, and paste the URL with `?token=...` into the Freemius webhook field.
The server checks this first (`authenticateWebhook` in
`internal/controlplane/server.go`); it also opportunistically checks an
`x-signature` HMAC header if one is ever present, but that isn't the primary
mechanism for this product type.

**Already configured** on the live Freemius listener (`apps/33496/webhooks/listeners`):
URL with a real generated token, 11 event types selected. Real event names
confirmed against the dashboard's live catalog (not guessed) — no generic
`payment.failed`/`license.revoked`/`subscription.renewed`:

- Grant/refresh entitlement: `license.activated`, `license.created`, `subscription.created`
- Revoke entitlement: `license.cancelled`, `license.deactivated`, `license.deleted`,
  `license.expired`, `subscription.cancelled`, `subscription.renewal.failed.last`
- Acknowledged, no-op: `license.activations.synced`, `license.blacklisted_site.deleted`

Freemius plan IDs for this product (store 17705, app 33496): Free = `55087`
(no mapping needed), Pro = `55088` @ $19/mo → `AGENTCTL_FREEMIUS_PLAN_MAP={"55088":"pro"}`
(already set in `helm/controlplane/values.yaml`).

Checkout links: production `https://checkout.freemius.com/app/33496/plan/55088/`,
sandbox `https://checkout.freemius.com/app/33496/plan/55088/?sandbox=true`.

**Not yet verified**: the exact JSON field names inside the real webhook
payload (`objects.license.*`) — Freemius's own docs confirm the `objects:
{ license }` nesting and HMAC-SHA256 signing, but no live payload has been
captured yet. Do this once the control plane is actually deployed (a
sandbox purchase right now would just fail to connect — nothing is running
at `agentctl-api.myk8s.pp.ua` yet): trigger a sandbox purchase, then check
`internal/controlplane/webhook.go`'s parsing against what actually arrives.

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
| `AGENTCTL_CP_ENV` | `dev` | `production` disables sandbox seeding and requires the two secrets below |
| `AGENTCTL_CP_ADDR` | `:8090` | Listen address |
| `AGENTCTL_CP_DB_PATH` | `controlplane.db` | SQLite file path (mount a volume in production) |
| `AGENTCTL_FREEMIUS_WEBHOOK_SECRET` | `dev-webhook-secret` (dev only) | HMAC key for the `x-signature` webhook header; required in production |
| `AGENTCTL_CP_SIGNING_KEY` | dev Ed25519 key (dev only) | JWT signing key seed (hex); required in production |
| `AGENTCTL_FREEMIUS_PLAN_MAP` | `{}` | JSON map of Freemius plan ID → our plan name, e.g. `{"12345":"pro","12346":"team"}` |
| `AGENTCTL_CONTROL_PLANE_URL` | _(client)_ | Client API base URL — defaults to production; set to `-` for offline/dev-key-only |

## API contract

OpenAPI 3.1 spec: [`openapi.yaml`](openapi.yaml)

Production URL: `https://agentctl-api.myk8s.pp.ua`
