# AgentCTL → $1M ARR Plan

Status: **Active** · Owner: product · Updated: 2026-06-30

Target: **$1M annual recurring revenue** via Pro / Team / Enterprise tiers, not more free features.

Math: ~4,200 × $20/mo Pro · ~840 × $99/mo Team · or fewer Enterprise deals.

---

## North star

**Product today:** local agent control plane (CLI + desktop) — **v0.8.3** shipped.

**Business gap:** billing path is wired but **first paid conversion not yet proven** end-to-end.

**Strategy:** hosted control plane + Freemius billing + curated packages + CI/enterprise wedge.

**Control plane:** `https://agentctl-api.myk8s.pp.ua` — **live** (`/health` + `/metrics` returning 200 as of 2026-07-05).

Detailed control-plane architecture: [`CONTROL_PLANE_SPEC.md`](CONTROL_PLANE_SPEC.md)  
Enterprise feature depth: [`ENTERPRISE_PLAN.md`](ENTERPRISE_PLAN.md)

---

## Phase 1 — Revenue foundation (Months 1–2)

Goal: first paid SKUs and client-side entitlement gating.

| # | Deliverable | Status |
|---|-------------|--------|
| 1.1 | `internal/entitlement` — plan, packages, local store | ✅ Done |
| 1.2 | `m license status` / `m license activate` | ✅ Done |
| 1.3 | Package lock — `m packages` shows LOCKED, install refuses | ✅ Done |
| 1.4 | `m packages install <name>` — copy bundle to `~/.config/m/` | ✅ Done |
| 1.5 | Desktop: plan badge + activate license in Settings | ✅ Done |
| 1.6 | Curated Pro packages: `pro-dev`, `pro-security` | ✅ Done |
| 1.7 | Control plane API contract (OpenAPI draft) | ✅ Done |
| 1.8 | Freemius sandbox + webhook receiver | ✅ Done |
| 1.9 | Signed JWT entitlements from server | ✅ Done |
| 1.10 | Pricing page + checkout links | 🔧 Almost done — page shipped; E2E purchase smoke pending |

**1.10 breakdown (2026-07-04):**

| # | Item | Status |
|---|------|--------|
| 1.10a | Control plane persists licenses (SQLite, survives restarts) | ✅ Done |
| 1.10b | Production Ed25519 signing key + client trust update | ✅ Done (code) — private key must be set as `AGENTCTL_CP_SIGNING_KEY` in the cluster secret store |
| 1.10c | Client defaults to production control plane URL (no env var needed) | ✅ Done |
| 1.10d | Freemius webhook signature verification (`x-signature` HMAC) | ✅ Done (code) — unverified against a live Freemius payload, test before go-live |
| 1.10e | GHCR image + Helm chart + ArgoCD deployment to `agentctl-api.myk8s.pp.ua` | ✅ Live — `/health` OK; redeploy with `pullPolicy: Always` on `:latest` |
| 1.10f | Freemius seller account: product, Pro plan, checkout link | ✅ Done — product `AgentCtl` (store 17705, app 33496); Free=`55087`, Pro=`55088` @ $19/mo monthly; checkout links: prod `https://checkout.freemius.com/app/33496/plan/55088/`, sandbox `?sandbox=true` |
| 1.10g | Populate real secrets in AWS Parameter Store (`/agentctl-controlplane/*`) | ✅ Done (assumed — production boots with `AGENTCTL_CP_ENV=production`) |
| 1.10h | Public pricing page linking to Freemius checkout | ✅ Done — `docs/pricing.html` (v0.8.1) |
| 1.10i | End-to-end smoke test with a real (or Freemius sandbox) purchase | 🔧 Runbook in `docs/E2E_BILLING.md` — execute sandbox checkout + verify webhook |

**Freemius webhook listener** (confirmed live in dashboard, `apps/33496/webhooks/listeners`): URL `https://agentctl-api.myk8s.pp.ua/v1/webhooks/freemius?token=<secret>`, 11 event types selected — `license.{activated,created,cancelled,deactivated,deleted,expired,activations.synced,blacklisted_site.deleted}`, `subscription.{created,cancelled,renewal.failed.last}`. Real event names confirmed against the live dashboard catalog and differ from the original guess (no generic `payment.failed`/`license.revoked` — see `internal/controlplane/webhook.go`).

**Exit criteria:** User can activate Pro, install a locked package, see plan in desktop. Dev keys work offline; production uses control plane.

---

## Phase 2 — Team wedge (Months 2–3)

Goal: first B2B revenue from pipelines and org governance.

| # | Deliverable | Status |
|---|-------------|--------|
| 2.1 | GitHub Action (`subzone/agentctl-action`) | ⬜ |
| 2.2 | Org audit export (desktop Security → JSON/CSV upload) | ⬜ |
| 2.3 | Team plan: shared agent registry sync | ⬜ |
| 2.4 | Fleet secrets (Vault/AWS) adapter | ⬜ |
| 2.5 | FinOps budgets in CI (`hard-stop` on limit) | ⬜ |

**Exit criteria:** Team can run `m run --ci` in GitHub Actions with policy + cost gate; 10 paying teams.

---

## Phase 3 — Moat & retention (Months 3–4)

| # | Deliverable | Status |
|---|-------------|--------|
| 3.1 | MCP marketplace (curated + one-click install) | ⬜ |
| 3.2 | Cloud sync of agents/skills (Pro) | ⬜ |
| 3.3 | Knowledge graph hosted connector | ⬜ |
| 3.4 | Vertical agent packs (SRE, compliance, QA) | ⬜ |
| 3.5 | Token broker / MoE proxy (Pro API included) | ⬜ |

---

## Phase 4 — Growth (ongoing)

| # | Deliverable | Status |
|---|-------------|--------|
| 4.1 | Homebrew cask reliable | ⬜ |
| 4.2 | Docs SEO / comparison pages | ⬜ |
| 4.3 | Demo content (fix failing test in 60s) | ⬜ |
| 4.4 | Reseller / agency package flows | ⬜ |

---

## Pricing sketch (validate with users)

| Plan | Price | Includes |
|------|-------|----------|
| **Free** | $0 | MoE with own keys, built-in agents, local extensions |
| **Pro** | $19–29/mo | Pro packages, cloud sync, priority routing, brokered API quota |
| **Team** | $99/mo + seats | Shared registry, audit, GitHub Action, 5 seats |
| **Enterprise** | Custom | SSO, RBAC, fleet policy, Vault secrets, SLA |

---

## Current sprint

**Phase 1.1–1.9 shipped** (entitlement store, CLI + desktop gating, control plane, JWT entitlements).

**Current sprint:** finish **1.10i** (run `docs/E2E_BILLING.md`), then **Phase 2.1** — GitHub Action.

---

## How to update this doc

When a row ships, change ⬜ → ✅ and link the PR/commit.  
When a phase completes, add the exit date and revenue metric.
