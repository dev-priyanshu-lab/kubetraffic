#!/usr/bin/env bash
# Phase 4 acceptance checks: the controller resolves backend Services +
# EndpointSlices into per-version ready endpoints in status, and reacts to scale.
set -euo pipefail

NS_SYS=kubetraffic-system
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }

command -v kubectl >/dev/null || fail "kubectl not found"
command -v jq >/dev/null || fail "jq not found"

tr_json() { kubectl -n demo get tr payment-weighted -o json; }
tr_field() { tr_json | jq -r "$1"; }

wait_for() { # <jq filter> <expected> <desc>
  local filter="$1" want="$2" desc="$3" got=""
  for _ in $(seq 40); do
    got="$(tr_field "${filter}" 2>/dev/null || true)"
    [ "${got}" = "${want}" ] && { pass "${desc} (${got})"; return; }
    sleep 1
  done
  fail "${desc}: got '${got}', want '${want}'"
}

echo "==> build + unit tests"
make -C controller test >/dev/null || fail "controller tests failed"
pass "controller build + unit tests"

echo "==> rebuild image, load into kind, redeploy"
make -C controller kind-load deploy >/dev/null || fail "deploy failed"
kubectl -n "${NS_SYS}" rollout status deploy/kubetraffic-controller --timeout=150s >/dev/null || fail "rollout not ready"
pass "controller redeployed"

echo "==> ensure demo backend is at full scale"
kubectl -n demo scale deploy/payment-v1 deploy/payment-v2 --replicas=2 >/dev/null
kubectl -n demo rollout status deploy/payment-v1 --timeout=60s >/dev/null
kubectl -n demo rollout status deploy/payment-v2 --timeout=60s >/dev/null

echo "==> apply TrafficRoute and check resolution"
kubectl apply -f demo/trafficroutes/payment-weighted.yaml >/dev/null
wait_for '.status.phase' 'Pending' 'phase'
wait_for '.status.conditions[]|select(.type=="Resolved").status' 'True' 'Resolved condition'

v1_ready=$(tr_field '.status.routes[0].resolvedEndpoints[]|select(.version=="v1").ready')
v2_ready=$(tr_field '.status.routes[0].resolvedEndpoints[]|select(.version=="v2").ready')
[ "${v1_ready}" = "2" ] || fail "v1 ready = '${v1_ready}', want 2"
[ "${v2_ready}" = "2" ] || fail "v2 ready = '${v2_ready}', want 2"
pass "resolvedEndpoints: v1=${v1_ready}/2, v2=${v2_ready}/2"

echo "==> cross-check against EndpointSlices"
slice_ready=$(kubectl -n demo get endpointslices -l kubernetes.io/service-name=payment -o json \
  | jq '[.items[].endpoints[]|select(.conditions.ready==true)]|length')
status_ready=$(tr_field '[.status.routes[0].resolvedEndpoints[].ready]|add')
[ "${slice_ready}" = "${status_ready}" ] \
  || fail "status total ready (${status_ready}) != EndpointSlice ready (${slice_ready})"
pass "status ready total (${status_ready}) matches EndpointSlices (${slice_ready})"

echo "==> weights mirror the spec"
w1=$(tr_field '.status.routes[0].currentWeights[]|select(.version=="v1").weight')
w2=$(tr_field '.status.routes[0].currentWeights[]|select(.version=="v2").weight')
[ "${w1}" = "90" ] && [ "${w2}" = "10" ] || fail "currentWeights v1=${w1} v2=${w2}, want 90/10"
pass "currentWeights: v1=${w1}, v2=${w2}"

echo "==> scale payment-v2 to 0 -> Degraded"
kubectl -n demo scale deploy/payment-v2 --replicas=0 >/dev/null
wait_for '.status.phase' 'Degraded' 'phase after scale-to-zero'
wait_for '.status.routes[0].resolvedEndpoints[]|select(.version=="v2").ready' '0' 'v2 ready after scale-to-zero'

echo "==> scale payment-v2 back to 2 -> recovers"
kubectl -n demo scale deploy/payment-v2 --replicas=2 >/dev/null
kubectl -n demo rollout status deploy/payment-v2 --timeout=60s >/dev/null
wait_for '.status.phase' 'Pending' 'phase after recovery'
wait_for '.status.routes[0].resolvedEndpoints[]|select(.version=="v2").ready' '2' 'v2 ready after recovery'

echo "==> cleanup"
kubectl -n demo delete tr payment-weighted --ignore-not-found >/dev/null

echo
echo "ALL PHASE 4 CHECKS PASSED"
