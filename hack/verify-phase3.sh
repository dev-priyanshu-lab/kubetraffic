#!/usr/bin/env bash
# Phase 3 acceptance checks: the controller runs in-cluster with leader election,
# reconciles TrafficRoutes, maintains status, and exposes reconcile metrics.
set -euo pipefail

NS=kubetraffic-system
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }

command -v kubectl >/dev/null || fail "kubectl not found"

echo "==> controller build + unit tests"
make -C controller test >/dev/null || fail "controller tests failed"
pass "controller build + unit tests"

echo "==> build image, load into kind, deploy"
make -C controller kind-load deploy >/dev/null || fail "deploy failed"

echo "==> wait for rollout"
kubectl -n "${NS}" rollout status deploy/kubetraffic-controller --timeout=150s >/dev/null \
  || fail "controller deployment did not become ready"
ready=$(kubectl -n "${NS}" get deploy kubetraffic-controller -o jsonpath='{.status.readyReplicas}')
[ "${ready}" = "2" ] || fail "expected 2 ready replicas, got '${ready}'"
pass "controller deployment ready (2/2 replicas)"

echo "==> leader election"
holder=""
for _ in $(seq 30); do
  holder=$(kubectl -n "${NS}" get lease kubetraffic-controller -o jsonpath='{.spec.holderIdentity}' 2>/dev/null || true)
  [ -n "${holder}" ] && break
  sleep 1
done
[ -n "${holder}" ] || fail "leader-election Lease has no holder"
pass "leader lease held by ${holder}"

echo "==> valid TrafficRoute -> phase=Pending, Accepted=True"
kubectl apply -f demo/trafficroutes/payment-weighted.yaml >/dev/null
phase=""
for _ in $(seq 30); do
  phase=$(kubectl -n demo get tr payment-weighted -o jsonpath='{.status.phase}' 2>/dev/null || true)
  [ -n "${phase}" ] && break
  sleep 1
done
[ "${phase}" = "Pending" ] || fail "expected phase=Pending, got '${phase}'"
gen=$(kubectl -n demo get tr payment-weighted -o jsonpath='{.metadata.generation}')
og=$(kubectl -n demo get tr payment-weighted -o jsonpath='{.status.observedGeneration}')
[ "${og}" = "${gen}" ] || fail "observedGeneration (${og}) != generation (${gen})"
acc=$(kubectl -n demo get tr payment-weighted -o jsonpath='{.status.conditions[?(@.type=="Accepted")].status}')
[ "${acc}" = "True" ] || fail "Accepted condition = '${acc}', want True"
pass "valid route: phase=Pending, observedGeneration=${og}, Accepted=True"

echo "==> semantically-invalid TrafficRoute -> phase=Invalid (reconciler backstop)"
kubectl apply -f demo/trafficroutes/invalid-weight-sum.yaml >/dev/null
phase=""
for _ in $(seq 30); do
  phase=$(kubectl -n demo get tr invalid-weight-sum -o jsonpath='{.status.phase}' 2>/dev/null || true)
  [ "${phase}" = "Invalid" ] && break
  sleep 1
done
[ "${phase}" = "Invalid" ] || fail "expected phase=Invalid, got '${phase}'"
pass "invalid route: phase=Invalid"

echo "==> reconcile metrics (scraped from the current leader)"
leader_pod="${holder%%_*}"
kubectl -n "${NS}" port-forward "pod/${leader_pod}" 18083:8080 >/tmp/kt-pf3.log 2>&1 &
pf=$!
trap 'kill ${pf} 2>/dev/null || true' EXIT
sleep 3
body="$(curl -sf http://localhost:18083/metrics || true)"
echo "${body}" | grep -q 'traffic_controller_reconcile_total' \
  || fail "traffic_controller_reconcile_total not exposed"
echo "${body}" | grep -q 'traffic_controller_reconcile_duration_seconds' \
  || fail "traffic_controller_reconcile_duration_seconds not exposed"
total="$(echo "${body}" | awk '/^traffic_controller_reconcile_total /{print $2}')"
[ "${total%.*}" -ge 1 ] 2>/dev/null || fail "leader reconcile_total is ${total:-<missing>}, expected >= 1"
pass "metrics exposed (leader reconcile_total=${total}, errors_total=$(echo "${body}" | awk '/^traffic_controller_reconcile_errors_total /{print $2}'))"

echo "==> cleanup demo routes"
kubectl -n demo delete tr payment-weighted invalid-weight-sum --ignore-not-found >/dev/null

echo
echo "ALL PHASE 3 CHECKS PASSED"
