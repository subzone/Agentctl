---
title: DevOps Patterns
layout: default
nav_order: 8
---

# DevOps patterns

`m` doesn't ship dedicated Kubernetes, Terraform, or Helm tools — and
that's by design. This page explains the philosophy, shows how to use
the existing tools for DevOps work, and points to the MCP path for
richer integrations.

## Why shell, not native tools

The 7 builtin tools (`shell`, `fs_read`, `fs_write`, `fs_list`, `git`,
`test_run`, `delegate`) are generic primitives. Through `shell`, an
agent already has full access to every CLI on the system:

```bash
kubectl apply -f deployment.yaml
terraform plan -out=tfplan
helm upgrade --install myapp ./chart
argocd app sync myapp
```

A dedicated `kubernetes` builtin would just wrap what `shell` already
does — while adding maintenance burden (client-go version pinning,
API compatibility across K8s releases) and bloating the binary.

The tradeoffs:

| Approach | Pros | Cons |
|---|---|---|
| Shell-based | Zero deps, works with any CLI version, binary stays small | LLM parses raw text output |
| Native tool | Structured JSON output, safety guardrails | Version coupling, binary bloat, maintenance |
| MCP server | Structured + decoupled, community-maintained | Extra process, setup overhead |

For most DevOps workflows, shell-based agents work well — the LLM is
good at parsing kubectl/terraform/helm output.

## DevOps agent examples

The repo ships three purpose-built DevOps agents in
[`examples/agents/`][examples]:

### k8s-debug — Kubernetes troubleshooting

```bash
m chat examples/agents/k8s-debug.md
```

```
» pods are crash-looping in the payments namespace
→ shell kubectl get pods -n payments
→ shell kubectl describe pod payments-api-7f8b9c-x2k4j -n payments
→ shell kubectl logs payments-api-7f8b9c-x2k4j --previous
The pod is OOMKilled — it's using 512Mi but the limit is 256Mi.
→ fs_read k8s/payments/deployment.yaml
→ fs_write k8s/payments/deployment.yaml (patch: resources.limits.memory → 512Mi)
```

The agent follows a systematic triage: events → describe → logs →
root cause → fix. It reads local manifests with `fs_read` and patches
them with `fs_write`.

### terraform-plan — Infrastructure review

```bash
m chat examples/agents/terraform-plan.md
```

```
» review the plan for the new RDS instance
→ shell terraform plan -out=tfplan
→ shell terraform show -json tfplan
3 resources will be created. I see two issues:
1. The security group allows 0.0.0.0/0 on port 5432 — restrict to VPC CIDR.
2. No lifecycle block — changing engine_version will destroy and recreate.
→ fs_write infra/rds.tf (patch: add cidr_blocks, lifecycle block)
→ shell terraform validate
```

The agent runs `terraform plan`, analyzes the output, flags security
and operational risks, and patches the `.tf` files directly.

### helm-deploy — Chart management

```bash
m chat examples/agents/helm-deploy.md
```

```
» lint and review the api-gateway chart before we deploy
→ shell helm lint ./charts/api-gateway
→ fs_read charts/api-gateway/values.yaml
→ shell helm template api-gateway ./charts/api-gateway --debug
Issues found:
1. No resource limits in values.yaml — pods can consume unbounded memory.
2. No PodDisruptionBudget — rolling updates could take down all replicas.
3. Ingress missing TLS configuration.
→ fs_write charts/api-gateway/values.yaml (patch: add resources, pdb)
→ fs_write charts/api-gateway/templates/pdb.yaml (create)
→ shell helm lint ./charts/api-gateway
```

The agent lints, renders templates locally, reviews values for
production readiness, and creates missing resources.

## Combining agents with delegate

For complex infrastructure tasks, use the `orchestrator` agent or
build a custom hub that delegates to DevOps specialists:

```markdown
---
name: infra-hub
type: agent
model: anthropic/claude-sonnet-4-6
tools:
  - fs_read
  - fs_list
subagents:
  - k8s-debug
  - terraform-plan
  - helm-deploy
---
You are an infrastructure orchestrator. Route tasks to the right
specialist:
- Kubernetes issues → k8s-debug
- Terraform changes → terraform-plan
- Helm charts → helm-deploy
```

Multiple delegations in the same turn run in parallel — the hub can
ask `terraform-plan` to review infra while `helm-deploy` lints the
chart simultaneously.

## MCP path for richer integrations

When you need structured output parsing or safety guardrails beyond
what shell provides, use MCP servers. `m` already supports MCP via
stdio JSON-RPC.

Example: a `kubectl-mcp` server that returns pod status as structured
JSON instead of raw text:

```markdown
---
name: k8s-agent
type: agent
model: anthropic/claude-sonnet-4-6
tools:
  - fs_read
  - fs_write
mcp:
  - kubectl
---
```

With a companion MCP server definition:

```markdown
---
name: kubectl
type: mcp_server
command: kubectl-mcp-server
args: ["--context", "production"]
---
```

The community is building MCP servers for kubectl, terraform, and
other DevOps tools. As they mature, you can plug them into any agent
without changing `m`'s core.

## When to use what

| Scenario | Approach |
|---|---|
| Ad-hoc debugging, one-off commands | Shell-based agent (`k8s-debug`, `devops`) |
| Plan review, manifest editing | Shell + fs agents (`terraform-plan`, `helm-deploy`) |
| CI/CD automation, structured responses | MCP server or spoke agent with `response_schema` |
| Multi-tool orchestration | Hub agent with `delegate` to specialists |
| Safety-critical operations | MCP server with built-in guardrails |

## Creating your own DevOps agent

Start from any of the examples and customize:

```yaml
---
name: my-infra
type: agent
model: anthropic/claude-sonnet-4-6
tools:
  - shell
  - fs_read
  - fs_write
  - fs_list
  - git
  - test_run
temperature: 0.2
---
Your system prompt here. Define:
1. What tools/CLIs the agent should use (kubectl, terraform, ansible, etc.)
2. What safety checks to perform before destructive operations
3. What patterns to look for when reviewing infrastructure code
4. How to report findings (structured JSON via response_schema, or prose)
```

Low temperature (0.2) is recommended for infrastructure work — you
want deterministic, precise output.

## Next steps

- **[Custom agents](agents.html)** — full agent authoring guide
- **[Architecture](architecture.html)** — how tools and MCP work
- **[Providers](providers.html)** — pick the right model for DevOps

[examples]: https://github.com/subzone/m/tree/main/examples/agents
