# AgentCTL Enterprise Features — Implementation Plan

## Overview

Five enterprise features to take AgentCTL from a solo-developer tool to an auditable, policy-governed, fleet-manageable platform.

**Current codebase anchors:**
- Engine loop: `internal/engine/engine.go` — `Step()` line 250, `runToolBlock()` line 647
- Tool confirmation callbacks: `ToolConfirm`, `ContinueConfirm`, `ErrorIntervention`
- Secrets: `internal/userconfig/keychain.go` — `GetAPIKey()` / `SetAPIKey()`
- Dangerous patterns (static): `cmd/m/chat.go:46-88`
- Cost tracking: `cmd/m/cost.go`
- Structured logging: `internal/logging/logging.go` (slog JSON to stderr)
- Session export: `cmd/m/session.go:114-141`
- Run command: `cmd/m/run.go`

---

## Delivery Phases

### Phase 1 — v0.2 (Foundation)

**Goal:** Get into automated pipelines and enforce basic guardrails.

#### Feature 5: Headless CI/CD Mode

**Status:** Not started  
**Estimated effort:** 4–6 days

**Gaps vs current state:**
- No structured JSON output mode (only terminal UI)
- No `--timeout` flag
- No differentiated exit codes
- No webhook notifications on completion/failure
- No native GitHub Actions action

**Exit codes to implement:**

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Agent error / LLM failure |
| 2 | Policy violation |
| 3 | Budget exceeded |
| 4 | Timeout |

**Files to create/modify:**

```
cmd/m/run.go                    MODIFY  add --ci, --output, --timeout flags
cmd/m/output/json.go            CREATE  newline-delimited JSON event emitter
cmd/m/output/text.go            CREATE  extract existing terminal renderer
internal/notify/webhook.go      CREATE  POST to Slack/Teams/custom URL
```

**`--ci` flag behavior:** sets `--yes`, `--output json`, `--no-color`, disables TUI spinner, enforces 15-minute default timeout, enables proper exit codes.

**JSON event stream (stdout):**
```json
{"type":"session_start","session_id":"s1234_1","agent":"deploy.md","ts":"2026-05-14T10:00:00Z"}
{"type":"tool_call","tool":"shell","args":{"command":"kubectl apply -f deploy.yaml"},"approved_by":"auto"}
{"type":"tool_result","tool":"shell","exit_code":0,"duration_ms":1240}
{"type":"llm_response","tokens":{"input":1200,"output":340},"cost_usd":0.0032}
{"type":"session_end","outcome":"success","total_cost_usd":0.012,"turns":4}
```

**Notification config:**
```yaml
ci:
  notify:
    webhook: https://hooks.slack.com/services/...
    on: [failure]          # failure | success | always
    include_cost: true
```

**GitHub Actions native action** (`subzone/agentctl-action`):
- Separate repo, thin wrapper
- Downloads binary for runner OS
- Runs `m run --ci --output json`
- Parses JSON stream for GitHub annotations
- Posts cost summary as PR comment

---

#### Feature 4 Tier 1: Inline Policy Rules

**Status:** Not started  
**Estimated effort:** 2–3 days

**Replace:** Static dangerous-pattern list in `cmd/m/chat.go:46-88`  
**With:** Configurable rule engine, enforced **regardless of `--yes`**

**Files to create/modify:**

```
internal/policy/inline.go       CREATE  rule evaluation (regex + path prefix)
internal/policy/engine.go       CREATE  policy check interface, called from engine callback
cmd/m/chat.go                   MODIFY  remove static list, wire policy engine
internal/engine/engine.go       MODIFY  call policy engine in runToolBlock() before confirm
```

**Rule schema in `~/.config/m/config.yaml` or `.m/config.yaml`:**
```yaml
policy:
  rules:
    - name: no-prod-kubectl-delete
      tool: shell
      deny_pattern: "kubectl delete.*production"
      message: "Deleting from production is forbidden"

    - name: restrict-write-paths
      tool: fs_write
      allow_path_prefix: ["./", "/tmp/"]
      message: "Writes outside the repo directory are forbidden"

    - name: no-pipe-to-shell
      tool: shell
      deny_pattern: "curl.*\\|.*sh|wget.*\\|.*sh"
      message: "Pipe-to-shell downloads are forbidden"
```

**Policy violation behavior:**
- Hard deny — cannot be overridden by `--yes` or user confirmation
- Prints rule name + message to stderr
- Writes audit event (if audit configured)
- Returns exit code 2

---

### Phase 2 — v0.3 (Audit + Secrets)

#### Feature 1: Centralized Audit Log

**Status:** Not started  
**Estimated effort:** 6–8 days

**Hook points:**
- `internal/engine/engine.go:runToolBlock()` — before/after every tool call
- `internal/llm/` provider calls — request/response metadata (not content)
- Session start/end in `cmd/m/run.go` and `cmd/m/chat.go`

**Files to create:**

```
internal/ports/audit.go         CREATE  AuditSink interface
internal/audit/noop.go          CREATE  default no-op sink
internal/audit/file.go          CREATE  append-only JSONL + HMAC per line
internal/audit/splunk.go        CREATE  Splunk HEC HTTP POST
internal/audit/datadog.go       CREATE  Datadog Logs API
internal/audit/syslog.go        CREATE  RFC 5424 (covers ELK via fluentd/logstash)
internal/audit/batch.go         CREATE  async batching + flush logic
```

**AuditSink interface:**
```go
type AuditSink interface {
    Emit(ctx context.Context, event AuditEvent) error
    Flush(ctx context.Context) error
    Close() error
}
```

**Audit event shape:**
```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440000",
  "session_id": "s1234567_1",
  "ts": "2026-05-14T10:23:01Z",
  "user": "milenk@company.com",
  "hostname": "dev-box-01",
  "project": "api-service",
  "tool": "shell",
  "args": {"command": "kubectl apply -f deployment.yaml"},
  "outcome": "success",
  "exit_code": 0,
  "duration_ms": 1240,
  "approved_by": "user",
  "policy_checked": true,
  "hmac": "sha256:abc123..."
}
```

HMAC per event (shared key from `audit.hmac_secret`) prevents local tampering before events reach the centralized system.

**Config:**
```yaml
audit:
  backend: splunk              # splunk | datadog | syslog | file | none
  endpoint: https://splunk.corp.com:8088
  token: ${SPLUNK_HEC_TOKEN}
  hmac_secret: ${AUDIT_HMAC_SECRET}
  tls_verify: true
  batch_size: 50
  flush_interval: 3s
  include_llm_metadata: true   # log token counts, model, cost — NOT prompt content
```

**Compliance note:** Never log prompt/response content by default. Only log metadata (tool name, args, outcome, token counts). Add explicit opt-in flag if content logging is needed.

---

#### Feature 2: Fleet-wide Secrets Management

**Status:** Not started  
**Estimated effort:** 5–7 days (initial backends), +1 sprint for OIDC

**Hook points:** `internal/userconfig/keychain.go` — extend the existing interface.

**Files to create:**

```
internal/secrets/vault.go       CREATE  HashiCorp Vault (AppRole + K8s service account auth)
internal/secrets/aws.go         CREATE  AWS Secrets Manager (aws-sdk-go-v2)
internal/secrets/azure.go       CREATE  Azure Key Vault (azidentity SDK)
internal/secrets/cache.go       CREATE  TTL wrapper — avoid per-call vault round trips
```

**Config:**
```yaml
secrets:
  backend: vault               # vault | aws | azure | keychain (default)
  vault:
    addr: https://vault.corp.com
    auth: kubernetes           # kubernetes | approle | oidc
    role: agentctl-dev
    path: secret/data/agentctl/api-keys
    ttl: 1h
  aws:
    region: us-east-1
    secret_name: agentctl/api-keys
    role_arn: arn:aws:iam::123456789:role/agentctl
  azure:
    vault_url: https://corp-vault.vault.azure.net
    secret_name: agentctl-api-keys
```

**Auth priority order:**
1. Environment variables (`ANTHROPIC_API_KEY`, etc.) — always checked first for CI compatibility
2. Configured secrets backend (Vault / AWS / Azure)
3. OS keychain (macOS Keychain, libsecret, Windows registry)

**OIDC/SAML identity (separate sub-task):**
- Browser-based device-code flow for interactive use
- K8s service account token for cluster workloads
- AWS IAM roles for EC2/ECS/Lambda
- Azure Managed Identity for Azure workloads

**Build tags:** Use `//go:build vault` / `//go:build aws` etc. to keep slim builds. Default binary has no cloud SDK dependencies.

---

### Phase 3 — v0.4 (FinOps + OPA + GitHub Action)

#### Feature 3: FinOps & Cost Governance

**Status:** Not started  
**Estimated effort:** 4–5 days

**Note:** If enterprises route through [LiteLLM proxy](https://docs.litellm.ai/docs/proxy/), they get budget limits, team allocation, and dashboards today — `provider: litellm` + `base_url` already supported. This feature covers orgs that connect directly to provider APIs.

**Hook points:**
- Before LLM call: check budget balance
- After LLM call: record spend, report upstream
- `cmd/m/cost.go`: cost calculation already exists — extend to push

**Files to create:**

```
internal/finops/budget.go       CREATE  pre-call check, post-call deduction
internal/finops/reporter.go     CREATE  async POST to cost reporting endpoint
internal/finops/tags.go         CREATE  inject team/project tags from env/git/config
```

**Config:**
```yaml
finops:
  daily_limit_usd: 50.0
  monthly_limit_usd: 500.0
  on_limit: hard-stop          # warn | hard-stop
  report_to: https://cost-api.corp.com/v1/usage
  tags:
    team: platform
    project: "${GIT_REPO}"     # resolved via git remote at runtime
    env: "${DEPLOY_ENV}"
    cost_center: "CC-1234"
```

**Budget counter storage:** Start with a local file (`~/.config/m/budget.json`, rotated daily/monthly). Remote API optional for multi-machine aggregation.

---

#### Feature 4 Tier 2: OPA Integration

**Status:** Not started  
**Estimated effort:** 3–4 days (after Tier 1)

**Adds:** Remote OPA server or embedded OPA WASM evaluation on top of inline rules.

**Files to create:**

```
internal/policy/opa.go          CREATE  OPA REST client (/v1/data/agentctl/allow)
internal/policy/bundle.go       CREATE  OPA policy bundle loader (local path or HTTP)
```

**OPA query input:**
```json
{
  "tool": "shell",
  "command": "kubectl delete pod/api -n production",
  "user": "milenk",
  "hostname": "dev-box-01",
  "project": "api-service",
  "session_id": "s1234_1",
  "environment": "production"
}
```

**OPA returns:**
```json
{"result": {"allow": false, "reason": "production-namespace-protection", "rule": "no-prod-kubectl-delete"}}
```

**Config:**
```yaml
policy:
  engine: opa                  # inline | opa
  opa:
    endpoint: https://opa.corp.com
    policy_path: agentctl/allow
    bundle_path: /etc/agentctl/policy.tar.gz   # optional local bundle
    fail_open: false           # deny if OPA unreachable
    timeout: 2s
```

---

#### Feature 5 Addendum: GitHub Actions Native Action

**Repo:** `subzone/agentctl-action` (separate repo)  
**Estimated effort:** 2–3 days

```yaml
# .github/workflows/agentic-deploy.yml
- uses: subzone/agentctl-action@v1
  with:
    agent: .m/deploy-agent.md
    task: "Deploy PR #${{ github.event.number }} to staging"
    yes: true
    timeout: 15m
    output: json
    policy: .m/policy.yaml
  env:
    ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

**Action behavior:**
- Downloads correct binary for runner OS/arch
- Runs `m run --ci --output json --timeout $timeout`
- Parses JSON event stream
- Emits GitHub annotations for warnings/errors
- Posts cost summary as step summary (`$GITHUB_STEP_SUMMARY`)
- Returns appropriate exit code

---

## Architecture: New Ports / Interfaces

All five features share a common pattern — add a port interface, wire via dependency injection into the engine, provide pluggable implementations.

```
internal/ports/
  ├── audit.go          AuditSink interface
  ├── policy.go         PolicyEngine interface
  └── secrets.go        SecretStore interface (extends existing keychain)

internal/finops/
  └── finops.go         BudgetEnforcer interface
```

Engine wiring in `internal/engine/engine.go`:
```go
type Engine struct {
    // existing fields ...
    Audit   ports.AuditSink       // nil = noop
    Policy  ports.PolicyEngine    // nil = allow-all
    Secrets ports.SecretStore     // nil = env/keychain
    FinOps  finops.BudgetEnforcer // nil = no limits
}
```

Each feature degrades gracefully to no-op when not configured — existing behavior unchanged for solo users.

---

## Config Schema (full enterprise example)

```yaml
# ~/.config/m/config.yaml
provider: anthropic
model: claude-sonnet-4-6

secrets:
  backend: vault
  vault:
    addr: https://vault.corp.com
    auth: kubernetes
    role: agentctl-dev
    path: secret/data/agentctl/api-keys

audit:
  backend: splunk
  endpoint: https://splunk.corp.com:8088
  token: ${SPLUNK_HEC_TOKEN}
  hmac_secret: ${AUDIT_HMAC_SECRET}
  batch_size: 50
  flush_interval: 3s

policy:
  engine: opa
  opa:
    endpoint: https://opa.corp.com
    policy_path: agentctl/allow
    fail_open: false
  rules:
    - name: no-prod-delete
      tool: shell
      deny_pattern: "kubectl delete.*production"
      message: "Production deletes are forbidden"

finops:
  daily_limit_usd: 50.0
  monthly_limit_usd: 500.0
  on_limit: hard-stop
  report_to: https://cost-api.corp.com/v1/usage
  tags:
    team: platform
    project: "${GIT_REPO}"

ci:
  notify:
    webhook: https://hooks.slack.com/services/...
    on: [failure]
    include_cost: true
```

---

## Summary Table

| Feature | Phase | Effort | Key Files | Exit Code |
|---------|-------|--------|-----------|-----------|
| 5a: CI/CD headless mode + JSON output | v0.2 | 4–6 days | `cmd/m/run.go`, `cmd/m/output/json.go` | 0/1/4 |
| 4a: Inline policy rules | v0.2 | 2–3 days | `internal/policy/inline.go` | 2 |
| 1: Audit log (file + cloud sinks) | v0.3 | 6–8 days | `internal/audit/`, `internal/ports/audit.go` | — |
| 2: Vault / AWS / Azure secrets | v0.3 | 5–7 days | `internal/secrets/` | — |
| 3: FinOps budget enforcement | v0.4 | 4–5 days | `internal/finops/` | 3 |
| 4b: OPA integration | v0.4 | 3–4 days | `internal/policy/opa.go` | 2 |
| 5b: GitHub Actions native action | v0.4 | 2–3 days | separate repo `agentctl-action` | — |

**Total estimate:** ~8–10 weeks of focused development across 3 releases.
