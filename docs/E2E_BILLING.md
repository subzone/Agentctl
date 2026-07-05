# End-to-end billing smoke test (Phase 1.10i)

Proves: Freemius checkout → webhook → control plane license store → `m license activate` → package install.

## Prerequisites

- Control plane reachable: `./scripts/smoke-controlplane.sh --prod`
- Freemius product **AgentCtl** (app `33496`, Pro plan `55088`)
- Webhook listener URL includes `?token=<AGENTCTL_FREEMIUS_WEBHOOK_SECRET>`

## 1. Sandbox purchase

Open (sandbox — no real charge):

```
https://checkout.freemius.com/app/33496/plan/55088/?sandbox=true
```

Complete checkout with a test card. Note the **license key** from the Freemius email or dashboard.

## 2. Verify webhook delivery

Check control plane logs (Kubernetes):

```bash
kubectl logs -l app.kubernetes.io/name=agentctl-controlplane -n <namespace> --tail=100
```

Look for `webhook_freemius` handling `license.created` or `license.activated` without 4xx/5xx.

Prometheus (if Grafana is wired):

- `agentctl_webhook_events_total{outcome="ok"}`
- `agentctl_license_activations_total{plan="pro"}`

## 3. Activate on CLI

Production control plane is the default (no env var needed):

```bash
m license activate <LICENSE_KEY>
m license status
m packages list          # pro-dev / pro-security should be installable
m packages install pro-dev
```

Offline override for local sandbox only:

```bash
export AGENTCTL_CONTROL_PLANE_URL=http://localhost:8090
m license activate FS-PRO-SANDBOX-2026
```

## 4. Activate in desktop

1. Settings → License → paste license key → Activate
2. Home should show **Pro** plan badge
3. Settings → Packages → install `pro-dev`

## 5. If webhook parsing fails

Capture the raw JSON from server logs, then compare field names against
`internal/controlplane/webhook.go` (`freemiusNestedPayload`). Adjust parsing
and redeploy the control plane image.

Common fixes:

- `secret_key` vs `license_key` under `objects.license`
- `plan_id` as string vs number (JSON unmarshals numbers into string fields as empty — may need `json.Number` or flexible type)

## 6. Pass criteria

- [ ] Sandbox purchase completes
- [ ] Webhook returns 200 and license row exists in SQLite
- [ ] `m license activate` returns a signed JWT with `plan: pro`
- [ ] `m packages install pro-dev` succeeds
- [ ] Desktop shows Pro and can install the package
