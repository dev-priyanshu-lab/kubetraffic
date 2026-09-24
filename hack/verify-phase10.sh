#!/usr/bin/env bash
# Phase 10 acceptance checks: the canary progression state machine drives real
# HAProxy traffic splits end-to-end — start/promote/rollback each broadcast a
# gRPC Decision (fanned out via Redis Pub/Sub so it reaches the controller
# regardless of which control-plane replica handled the REST call) that the
# controller receives and reprograms the data plane from.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"
NS_SYS=kubetraffic-system
NS_DATA=kubetraffic-data
TR=payment-weighted

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }
for b in kubectl jq curl; do command -v "$b" >/dev/null || fail "$b not found"; done

JAVA21="$(/usr/libexec/java_home -v 21 2>/dev/null || true)"

programmed_msg() {
  kubectl -n demo get tr "${TR}" -o jsonpath='{.status.conditions[?(@.type=="Programmed")].message}' 2>/dev/null || true
}

# HAProxy's Data Plane API batches reloads (--reload-delay 5s), so "the API call
# returned" does not mean "HAProxy is already serving it". Wait for the
# TrafficRoute's Programmed condition to report a NEW config hash instead of
# guessing a sleep duration.
wait_for_new_config() {
  local baseline="$1" now=""
  for _ in $(seq 30); do
    now="$(programmed_msg)"
    if [ -n "${now}" ] && [ "${now}" != "${baseline}" ]; then
      # The condition flips as soon as the config is PUSHED to the Data Plane
      # API, but --reload-delay 5 (see deployments/haproxy) batches the actual
      # HAProxy worker reload up to 5s later. Clear that window before
      # trusting live traffic to reflect the new config.
      sleep 7
      echo "${now}"
      return 0
    fi
    sleep 1
  done
  fail "Programmed condition never changed from '${baseline}' within 30s"
}

echo "==> unit tests"
( cd control-plane && JAVA_HOME="${JAVA21}" mvn -B -q test ) || fail "control-plane unit tests failed"
pass "control-plane unit tests"

echo "==> deploy"
make control-plane-image >/dev/null || fail "image build failed"
kubectl -n "${NS_SYS}" rollout restart deploy/kubetraffic-control-plane >/dev/null
kubectl -n "${NS_SYS}" rollout status deploy/kubetraffic-control-plane --timeout=180s >/dev/null || fail "control-plane not ready"
pass "control-plane redeployed"

kubectl -n demo scale deploy/payment-v1 deploy/payment-v2 --replicas=2 >/dev/null
kubectl -n demo rollout status deploy/payment-v1 --timeout=60s >/dev/null
kubectl -n demo rollout status deploy/payment-v2 --timeout=60s >/dev/null
kubectl -n demo delete tr "${TR}" --ignore-not-found >/dev/null
kubectl apply -f demo/trafficroutes/payment-weighted.yaml >/dev/null
# Wait for phase=Ready AND ControlPlaneRegistered=True specifically: right after
# restarting control-plane, the controller can reach Ready via its own cached
# last-known-good config before this fresh route is actually registered (and
# its policy row created) with the newly-restarted control plane. Canary calls
# need that policy row to exist.
for _ in $(seq 45); do
  phase="$(kubectl -n demo get tr "${TR}" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
  cp_cond="$(kubectl -n demo get tr "${TR}" -o jsonpath='{.status.conditions[?(@.type=="ControlPlaneRegistered")].status}' 2>/dev/null || true)"
  [ "${phase}" = "Ready" ] && [ "${cp_cond}" = "True" ] && break
  sleep 2
done
[ "${phase}" = "Ready" ] && [ "${cp_cond}" = "True" ] || fail "route not Ready+ControlPlaneRegistered within 90s (phase=${phase}, cp=${cp_cond})"
pass "demo route Ready and registered with the control plane"

cp_pf() { kubectl -n "${NS_SYS}" port-forward svc/kubetraffic-control-plane "$1":8080 >/tmp/kt-p10-cp-"$1".log 2>&1 & echo $!; }
gw_pf() { kubectl -n "${NS_DATA}" port-forward svc/kubetraffic-gateway "$1":80 >/tmp/kt-p10-gw-"$1".log 2>&1 & echo $!; }

sample() { # <port> <n> -> "v1 v2 other"
  local port="$1" n="$2" v1=0 v2=0 other=0 v
  for _ in $(seq "${n}"); do
    v=$(curl -s -H 'Host: api.example.com' "http://localhost:${port}/payment" | jq -r '.version // "?"')
    case "${v}" in v1) v1=$((v1+1));; v2) v2=$((v2+1));; *) other=$((other+1));; esac
  done
  echo "${v1} ${v2} ${other}"
}

echo "==> start canary (5,50,100) v1(stable) / v2(canary)"
baseline="$(programmed_msg)"
pf=$(cp_pf 19301); sleep 3
start=$(curl -s -XPOST "http://localhost:19301/api/v1/canary/${TR}/start?namespace=demo&path=/payment" \
  -H 'Content-Type: application/json' -d '{"stableVersion":"v1","canaryVersion":"v2","steps":[5,50,100]}')
kill "${pf}" 2>/dev/null || true
[ "$(echo "${start}" | jq -r '.status')" = "PROGRESSING" ] || fail "start status = $(echo "${start}" | jq -r '.status'), want PROGRESSING"
[ "$(echo "${start}" | jq -r '.currentCanaryWeight')" = "5" ] || fail "start weight != 5: ${start}"
pass "canary started at 5%"

baseline="$(wait_for_new_config "${baseline}")"
pf=$(gw_pf 19302); sleep 3
read -r a1 a2 ao <<<"$(sample 19302 100)"
kill "${pf}" 2>/dev/null || true
[ "${ao}" -eq 0 ] || fail "unexpected non-v1/v2 responses"
[ "${a1}" -ge 90 ] && [ "${a1}" -le 100 ] || fail "5%% canary: v1=${a1}/100 outside [90,100]"
pass "live traffic follows 95/5 (v1=${a1}, v2=${a2})"

echo "==> promote to 50%"
pf=$(cp_pf 19303); sleep 3
mid=$(curl -s -XPOST "http://localhost:19303/api/v1/canary/${TR}/promote?namespace=demo&path=/payment")
kill "${pf}" 2>/dev/null || true
[ "$(echo "${mid}" | jq -r '.currentCanaryWeight')" = "50" ] || fail "promote-1 weight != 50: ${mid}"

baseline="$(wait_for_new_config "${baseline}")"
pf=$(gw_pf 19304); sleep 3
read -r b1 b2 bo <<<"$(sample 19304 100)"
kill "${pf}" 2>/dev/null || true
[ "${bo}" -eq 0 ] || fail "unexpected non-v1/v2 responses"
[ "${b1}" -ge 35 ] && [ "${b1}" -le 65 ] || fail "50%% canary: v1=${b1}/100 outside [35,65]"
pass "live traffic follows ~50/50 (v1=${b1}, v2=${b2})"

echo "==> promote to 100% -> PROMOTED"
pf=$(cp_pf 19305); sleep 3
final=$(curl -s -XPOST "http://localhost:19305/api/v1/canary/${TR}/promote?namespace=demo&path=/payment")
[ "$(echo "${final}" | jq -r '.status')" = "PROMOTED" ] || fail "final status = $(echo "${final}" | jq -r '.status'), want PROMOTED"
noop=$(curl -s -XPOST "http://localhost:19305/api/v1/canary/${TR}/promote?namespace=demo&path=/payment")
kill "${pf}" 2>/dev/null || true
[ "$(echo "${noop}" | jq -r '.status')" = "PROMOTED" ] || fail "promote past PROMOTED changed state: ${noop}"
# the terminal promote DOES change the effective config (-> 100%); the
# following no-op promote does not, so wait on THIS transition only.
baseline="$(wait_for_new_config "${baseline}")"
pass "reached PROMOTED at 100%; further promote is a no-op"

echo "==> rollback returns to 0, even from PROMOTED"
pf=$(cp_pf 19306); sleep 3
rb=$(curl -s -XPOST "http://localhost:19306/api/v1/canary/${TR}/rollback?namespace=demo&path=/payment")
kill "${pf}" 2>/dev/null || true
[ "$(echo "${rb}" | jq -r '.status')" = "ROLLED_BACK" ] || fail "rollback status = $(echo "${rb}" | jq -r '.status')"
[ "$(echo "${rb}" | jq -r '.currentCanaryWeight')" = "0" ] || fail "rollback weight != 0: ${rb}"

baseline="$(wait_for_new_config "${baseline}")"
pf=$(gw_pf 19307); sleep 3
read -r c1 c2 co <<<"$(sample 19307 60)"
kill "${pf}" 2>/dev/null || true
[ "${co}" -eq 0 ] || fail "unexpected non-v1/v2 responses"
[ "${c2}" -eq 0 ] || fail "expected 0%% to v2 after rollback, got v2=${c2}/60"
pass "traffic fully back on v1 after rollback (v1=${c1}, v2=${c2})"

echo "==> audit trail recorded every transition"
pf=$(cp_pf 19308); sleep 3
audit_n=$(curl -s "http://localhost:19308/api/v1/audit?target=policy/demo/${TR}/payment" | jq 'length')
kill "${pf}" 2>/dev/null || true
[ "${audit_n}" -ge 4 ] 2>/dev/null || fail "audit entries = ${audit_n}, want >= 4 (start+promote+promote+rollback)"
pass "audit log has ${audit_n} canary entries"

echo "==> cleanup"
kubectl -n demo delete tr "${TR}" --ignore-not-found >/dev/null

echo
echo "ALL PHASE 10 CHECKS PASSED"
