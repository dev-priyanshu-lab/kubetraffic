#!/usr/bin/env bash
# Phase 5 acceptance checks: the controller renders a routing model and programs
# HAProxy via the Data Plane API; traffic through the gateway is split by weight.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }

for b in kubectl jq curl; do command -v "$b" >/dev/null || fail "$b not found"; done

echo "==> unit tests"
make -C controller test >/dev/null || fail "controller tests failed"
pass "controller build + unit tests"

echo "==> deploy HAProxy data plane"
kubectl apply -k deployments/haproxy >/dev/null
kubectl -n kubetraffic-data rollout status deploy/kubetraffic-haproxy --timeout=150s >/dev/null \
  || fail "haproxy did not become ready"
pass "haproxy deployment ready"

echo "==> rebuild + redeploy controller (with --haproxy-dataplane-url)"
make -C controller kind-load deploy >/dev/null || fail "controller deploy failed"
kubectl -n kubetraffic-system rollout status deploy/kubetraffic-controller --timeout=150s >/dev/null \
  || fail "controller rollout not ready"
pass "controller redeployed"

echo "==> ensure demo backend at full scale"
kubectl -n demo scale deploy/payment-v1 deploy/payment-v2 --replicas=2 >/dev/null
kubectl -n demo rollout status deploy/payment-v1 --timeout=60s >/dev/null
kubectl -n demo rollout status deploy/payment-v2 --timeout=60s >/dev/null

echo "==> apply TrafficRoute, expect phase=Ready / Programmed=True"
kubectl apply -f demo/trafficroutes/payment-weighted.yaml >/dev/null
phase=""
for _ in $(seq 40); do
  phase=$(kubectl -n demo get tr payment-weighted -o jsonpath='{.status.phase}' 2>/dev/null || true)
  [ "${phase}" = "Ready" ] && break
  sleep 2
done
[ "${phase}" = "Ready" ] || fail "phase = '${phase}', want Ready ($(kubectl -n demo get tr payment-weighted -o jsonpath='{.status.conditions[?(@.type=="Programmed")].message}'))"
prog=$(kubectl -n demo get tr payment-weighted -o jsonpath='{.status.conditions[?(@.type=="Programmed")].status}')
[ "${prog}" = "True" ] || fail "Programmed = '${prog}', want True"
pass "phase=Ready, Programmed=True"

echo "==> Data Plane API shows our backend"
DP_AUTH="admin:kubetraffic-dev-not-secret"
kubectl -n kubetraffic-data port-forward svc/kubetraffic-dataplane 15555:5555 >/tmp/kt-dp.log 2>&1 &
dp_pf=$!
sleep 3
backends=$(curl -s -u "${DP_AUTH}" "http://localhost:15555/v3/services/haproxy/configuration/backends" || true)
echo "${backends}" | grep -q 'kt_be_' || fail "Data Plane API has no kt_be_ backend"
pass "Data Plane API backend present"
kill ${dp_pf} 2>/dev/null || true

echo "==> traffic split through the gateway (Host: api.example.com /payment)"
kubectl -n kubetraffic-data port-forward svc/kubetraffic-gateway 18080:80 >/tmp/kt-gw.log 2>&1 &
gw_pf=$!
trap 'kill ${gw_pf} 2>/dev/null || true' EXIT
sleep 3

N=300
v1=0; v2=0; other=0
for _ in $(seq "${N}"); do
  body=$(curl -s -H 'Host: api.example.com' "http://localhost:18080/payment" || true)
  case "$(echo "${body}" | jq -r '.version // "?"' 2>/dev/null)" in
    v1) v1=$((v1+1)) ;;
    v2) v2=$((v2+1)) ;;
    *)  other=$((other+1)) ;;
  esac
done
echo "    v1=${v1} v2=${v2} other=${other} (of ${N})"
[ "${other}" -eq 0 ] || fail "${other} responses were not v1/v2"
[ "${v1}" -ge 230 ] && [ "${v1}" -le 290 ] || fail "v1 share ${v1}/${N} outside ~90% tolerance [230,290]"
[ "${v2}" -ge 10 ]  && [ "${v2}" -le 70 ]  || fail "v2 share ${v2}/${N} outside ~10% tolerance [10,70]"
pass "weighted split ~90/10 (v1=${v1}, v2=${v2})"

echo "==> 503 for an unknown host"
code=$(curl -s -o /dev/null -w '%{http_code}' -H 'Host: nope.example.com' "http://localhost:18080/payment" || true)
[ "${code}" = "503" ] || fail "unknown host returned ${code}, want 503"
pass "unknown host -> 503 (kubetraffic_no_route)"

echo "==> cleanup"
kubectl -n demo delete tr payment-weighted --ignore-not-found >/dev/null

echo
echo "ALL PHASE 5 CHECKS PASSED"
