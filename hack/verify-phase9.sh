#!/usr/bin/env bash
# Phase 9 acceptance checks: Redis-backed rate limiting and circuit-breaker state
# are genuinely shared across control-plane replicas (not per-pod), survive a
# full replica restart, and fail open (never 500) when Redis itself is down.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"
NS=kubetraffic-system

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }
for b in kubectl jq curl; do command -v "$b" >/dev/null || fail "$b not found"; done

JAVA21="$(/usr/libexec/java_home -v 21 2>/dev/null || true)"

echo "==> unit tests"
( cd control-plane && JAVA_HOME="${JAVA21}" mvn -B -q test ) || fail "control-plane unit tests failed"
pass "control-plane unit tests"

echo "==> deploy Redis + control plane"
make control-plane-image >/dev/null || fail "image build failed"
kubectl apply -k deployments/control-plane >/dev/null
kubectl -n "${NS}" rollout status deploy/kubetraffic-redis --timeout=120s >/dev/null || fail "redis not ready"
kubectl -n "${NS}" rollout restart deploy/kubetraffic-control-plane >/dev/null
kubectl -n "${NS}" rollout status deploy/kubetraffic-control-plane --timeout=180s >/dev/null || fail "control-plane not ready"
pass "redis (1/1) and control-plane (2/2) deployed"

pods=($(kubectl -n "${NS}" get pods -l app.kubernetes.io/component=control-plane -o jsonpath='{.items[*].metadata.name}'))
[ "${#pods[@]}" -ge 2 ] || fail "expected 2 control-plane pods, found ${#pods[@]}"
pod_a="${pods[0]}"; pod_b="${pods[1]}"

pf() { kubectl -n "${NS}" port-forward "$1" "$2":8080 >/tmp/kt-p9-"$2".log 2>&1 & echo $!; }

echo "==> rate limit counter is shared across two DIFFERENT pods, not per-replica"
pf_a=$(pf "pod/${pod_a}" 19101); sleep 3
key="verify-$$"
for i in 1 2 3; do
  curl -s -o /dev/null -XPOST "http://localhost:19101/api/v1/ratelimit/${key}/check?limit=5&windowSeconds=120"
done
kill "${pf_a}" 2>/dev/null || true

pf_b=$(pf "pod/${pod_b}" 19102); sleep 3
resp=$(curl -s -XPOST "http://localhost:19102/api/v1/ratelimit/${key}/check?limit=5&windowSeconds=120")
count=$(echo "${resp}" | jq -r '.count')
kill "${pf_b}" 2>/dev/null || true
[ "${count}" = "4" ] || fail "expected pod B to continue the shared count at 4, got ${count} (${resp})"
pass "counter continued from pod A's 3 to pod B's 4 -> Redis-shared, not per-replica"

echo "==> rate limit denies once the shared limit is exceeded"
pf_a=$(pf "pod/${pod_a}" 19103); sleep 3
curl -s -o /dev/null -XPOST "http://localhost:19103/api/v1/ratelimit/${key}/check?limit=5&windowSeconds=120"
code=$(curl -s -o /dev/null -w '%{http_code}' -XPOST "http://localhost:19103/api/v1/ratelimit/${key}/check?limit=5&windowSeconds=120")
kill "${pf_a}" 2>/dev/null || true
[ "${code}" = "429" ] || fail "6th request over a limit of 5 returned ${code}, want 429"
pass "6th request over the limit -> 429"

echo "==> circuit breaker opens at threshold and state survives a full pod restart"
cb_key="verify-cb-$$"
pf_a=$(pf svc/kubetraffic-control-plane 19104); sleep 3
for _ in 1 2 3 4 5; do
  curl -s -o /dev/null -XPOST "http://localhost:19104/api/v1/circuit-breaker/${cb_key}/failure?threshold=5&recoverySeconds=5"
done
state=$(curl -s "http://localhost:19104/api/v1/circuit-breaker/${cb_key}?recoverySeconds=5" | jq -r '.state')
kill "${pf_a}" 2>/dev/null || true
[ "${state}" = "OPEN" ] || fail "circuit state = ${state}, want OPEN after 5 failures at threshold 5"
pass "circuit OPEN after 5 consecutive failures"

kubectl -n "${NS}" rollout restart deploy/kubetraffic-control-plane >/dev/null
kubectl -n "${NS}" rollout status deploy/kubetraffic-control-plane --timeout=180s >/dev/null
sleep 6 # let the 5s recoverySeconds elapse too, so we also see the HALF_OPEN transition

pf_a=$(pf svc/kubetraffic-control-plane 19105); sleep 3
after=$(curl -s "http://localhost:19105/api/v1/circuit-breaker/${cb_key}?recoverySeconds=5")
kill "${pf_a}" 2>/dev/null || true
after_state=$(echo "${after}" | jq -r '.state')
[ "${after_state}" = "HALF_OPEN" ] || fail "state after restart+recovery = ${after_state}, want HALF_OPEN (proves Redis-backed persistence + recovery logic)"
pass "circuit state survived a full control-plane pod restart and auto-recovered to HALF_OPEN"

echo "==> Redis outage: both endpoints fail OPEN (200, never 500)"
kubectl -n "${NS}" scale deploy/kubetraffic-redis --replicas=0 >/dev/null
sleep 3
pf_a=$(pf svc/kubetraffic-control-plane 19106); sleep 3
rl_code=$(curl -s -o /dev/null -w '%{http_code}' -XPOST "http://localhost:19106/api/v1/ratelimit/outage-test/check?limit=5&windowSeconds=60")
cb_code=$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:19106/api/v1/circuit-breaker/outage-test?recoverySeconds=5")
kill "${pf_a}" 2>/dev/null || true
[ "${rl_code}" = "200" ] || fail "rate limit check during Redis outage returned ${rl_code}, want 200 (fail-open)"
[ "${cb_code}" = "200" ] || fail "circuit breaker check during Redis outage returned ${cb_code}, want 200 (fail-open)"
fallback_logged=$(kubectl -n "${NS}" logs -l app.kubernetes.io/component=control-plane --since=2m 2>/dev/null | grep -c "falling back to a local" || true)
[ "${fallback_logged}" -ge 1 ] || fail "no 'falling back to a local' log line found"
pass "rate-limit and circuit-breaker both fail open during a Redis outage (${fallback_logged} fallback log lines)"

kubectl -n "${NS}" scale deploy/kubetraffic-redis --replicas=1 >/dev/null
kubectl -n "${NS}" rollout status deploy/kubetraffic-redis --timeout=120s >/dev/null
pass "Redis restored"

echo
echo "ALL PHASE 9 CHECKS PASSED"
