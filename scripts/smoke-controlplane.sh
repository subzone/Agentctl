#!/usr/bin/env bash
# Smoke-test the AgentCTL control plane (local sandbox or production health).
#
# Usage:
#   ./scripts/smoke-controlplane.sh              # local dev server on :8090
#   ./scripts/smoke-controlplane.sh --prod       # read-only prod health + metrics
#   ./scripts/smoke-controlplane.sh --url http://host:8090
#
# Local mode starts a temporary server, activates a sandbox license, and checks JWT.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
URL="${AGENTCTL_CONTROL_PLANE_URL:-}"
MODE="local"
PID=""
PORT=""

cleanup() {
  if [[ -n "$PID" ]]; then
    kill "$PID" 2>/dev/null || true
    wait "$PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

while [[ $# -gt 0 ]]; do
  case "$1" in
    --prod) MODE="prod"; URL="https://agentctl-api.myk8s.pp.ua"; shift ;;
    --url) URL="${2:?}"; shift 2 ;;
    -h|--help)
      sed -n '2,8p' "$0"
      exit 0
      ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

if [[ "$MODE" == "local" && -z "$URL" ]]; then
  PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("",0)); print(s.getsockname()[1]); s.close()')"
  URL="http://127.0.0.1:${PORT}"
fi

URL="${URL%/}"

check_health() {
  local body
  body="$(curl -fsS "$URL/health")"
  echo "health: $body"
  echo "$body" | grep -q '"status":"ok"' || { echo "health check failed" >&2; exit 1; }
}

check_metrics() {
  local code
  code="$(curl -s -o /dev/null -w '%{http_code}' "$URL/metrics")"
  echo "metrics: HTTP $code"
  [[ "$code" == "200" ]] || { echo "metrics check failed" >&2; exit 1; }
}

if [[ "$MODE" == "prod" ]]; then
  echo "==> production smoke (read-only): $URL"
  check_health
  check_metrics
  echo "OK — production control plane is reachable"
  exit 0
fi

echo "==> local smoke: $URL"
export AGENTCTL_CP_ENV=dev
export AGENTCTL_CP_ADDR=":${PORT}"
export AGENTCTL_CP_DB_PATH="$(mktemp -t agentctl-cp-XXXXXX.db)"

go run "$ROOT/cmd/controlplane" &
PID=$!

for _ in $(seq 1 30); do
  if curl -fsS "$URL/health" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

check_health
check_metrics

echo "==> activate sandbox license"
resp="$(curl -fsS -X POST "$URL/v1/auth/freemius/activate" \
  -H 'Content-Type: application/json' \
  -d '{"license_key":"FS-PRO-SANDBOX-2026"}')"
echo "$resp" | grep -q '"plan":"pro"' || { echo "activate failed: $resp" >&2; exit 1; }
echo "activate: ok (plan=pro)"

echo "OK — local control plane smoke passed"
