#!/usr/bin/env bash
# Phase 6 acceptance checks: editing TrafficRoute weights re-programs HAProxy
# with no restart, and the observed traffic split follows the new weights.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }
for b in kubectl jq curl; do command -v "$b" >/dev/null || fail "$b not found"; done

echo "==> unit tests (apportionment)"
make -C controller test >/dev/null || fail "controller tests failed"
pass "controller build + unit tests"

echo "==> preconditions: haproxy + controller up, backend scaled"
kubectl -n kubetraffic-data rollout status deploy/kubetraffic-haproxy --timeout=60s >/dev/null || fail "haproxy not ready"
kubectl -n kubetraffic-system rollout status deploy/kubetraffic-controller --timeout=60s >/dev/null || fail "controller not ready"
kubectl -n demo scale deploy/payment-v1 deploy/payment-v2 --replicas=2 >/dev/null
kubectl -n demo rollout status deploy/payment-v2 --timeout=60s >/dev/null

haproxy_pod() { kubectl -n kubetraffic-data get pod -l app.kubernetes.io/component=data-plane -o jsonpath='{.items[0].metadata.name}'; }
POD_BEFORE="$(haproxy_pod)"

kubectl -n kubetraffic-data port-forward svc/kubetraffic-gateway 18080:80 >/tmp/kt-gw6.log 2>&1 &
gw_pf=$!
trap 'kill ${gw_pf} 2>/dev/null || true' EXIT
sleep 3

sample() { # <n> -> "v1 v2 other"
  local n="$1" v1=0 v2=0 other=0 body ver
  for _ in $(seq "$n"); do
    body=$(curl -s -H 'Host: api.example.com' "http://localhost:18080/payment" || true)
    ver=$(echo "$body" | jq -r '.version // "?"' 2>/dev/null)
    case "$ver" in v1) v1=$((v1+1));; v2) v2=$((v2+1));; *) other=$((other+1));; esac
  done
  echo "$v1 $v2 $other"
}

echo "==> apply 90/10"
kubectl apply -f demo/trafficroutes/payment-weighted.yaml >/dev/null
for _ in $(seq 30); do
  [ "$(kubectl -n demo get tr payment-weighted -o jsonpath='{.status.phase}')" = "Ready" ] && break
  sleep 2
done
sleep 3
read -r a1 a2 ao <<<"$(sample 300)"
echo "    90/10 -> v1=$a1 v2=$a2 other=$ao"
[ "$ao" -eq 0 ] || fail "unexpected non-v1/v2 responses"
[ "$a1" -ge 230 ] && [ "$a1" -le 290 ] || fail "90/10: v1=$a1 outside [230,290]"

echo "==> edit to 50/50 (kubectl apply, same object)"
gen_before=$(kubectl -n demo get tr payment-weighted -o jsonpath='{.metadata.generation}')
kubectl apply -f demo/trafficroutes/payment-5050.yaml >/dev/null
gen_after=$(kubectl -n demo get tr payment-weighted -o jsonpath='{.metadata.generation}')
[ "$gen_after" -gt "$gen_before" ] || fail "generation did not advance ($gen_before -> $gen_after)"

# wait for currentWeights to reflect 50/50
for _ in $(seq 30); do
  w=$(kubectl -n demo get tr payment-weighted -o jsonpath='{.status.routes[0].currentWeights[?(@.version=="v2")].weight}')
  [ "$w" = "50" ] && break
  sleep 2
done
[ "$w" = "50" ] || fail "status.currentWeights v2 = '$w', want 50"
pass "status currentWeights updated to 50/50"

sleep 3
read -r b1 b2 bo <<<"$(sample 400)"
echo "    50/50 -> v1=$b1 v2=$b2 other=$bo"
[ "$bo" -eq 0 ] || fail "unexpected non-v1/v2 responses after re-split"
[ "$b1" -ge 150 ] && [ "$b1" -le 250 ] || fail "50/50: v1=$b1 outside [150,250]"
[ "$b2" -ge 150 ] && [ "$b2" -le 250 ] || fail "50/50: v2=$b2 outside [150,250]"
pass "traffic re-split to ~50/50 (v1=$b1, v2=$b2)"

POD_AFTER="$(haproxy_pod)"
[ "$POD_BEFORE" = "$POD_AFTER" ] || fail "HAProxy pod restarted ($POD_BEFORE -> $POD_AFTER); re-split should be a hitless reload"
pass "HAProxy pod unchanged: re-program was a hitless reload"

echo "==> cleanup"
kubectl -n demo delete tr payment-weighted --ignore-not-found >/dev/null

echo
echo "ALL PHASE 6 CHECKS PASSED"
