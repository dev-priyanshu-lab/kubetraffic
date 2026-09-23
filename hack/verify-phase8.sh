#!/usr/bin/env bash
# Phase 8 acceptance checks: the controller registers TrafficRoutes with the
# Java control plane over gRPC, uses its authoritative weights (not the CRD's
# own), reacts to decisions pushed over StreamDecisions, falls back to the
# last-known-good config when the control plane is unreachable, and cleans up
# via DeleteRoute when a TrafficRoute is deleted.
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

echo "==> unit tests (Go + Java)"
( cd controller && go test ./... -count=1 >/dev/null ) || fail "controller tests failed"
pass "controller unit tests"
( cd control-plane && JAVA_HOME="${JAVA21}" mvn -B -q test ) || fail "control-plane unit tests failed"
pass "control-plane unit tests"

echo "==> rebuild + redeploy both sides"
make control-plane-image >/dev/null || fail "control-plane image build failed"
kubectl apply -k deployments/control-plane >/dev/null
kubectl -n "${NS_SYS}" rollout restart deploy/kubetraffic-control-plane >/dev/null
kubectl -n "${NS_SYS}" rollout status deploy/kubetraffic-control-plane --timeout=180s >/dev/null || fail "control-plane not ready"
make -C controller kind-load deploy >/dev/null || fail "controller deploy failed"
kubectl -n "${NS_SYS}" rollout status deploy/kubetraffic-controller --timeout=180s >/dev/null || fail "controller not ready"
pass "control plane (2/2) and controller (2/2) deployed"

kubectl -n demo scale deploy/payment-v1 deploy/payment-v2 --replicas=2 >/dev/null
kubectl -n demo rollout status deploy/payment-v1 --timeout=60s >/dev/null
kubectl -n demo rollout status deploy/payment-v2 --timeout=60s >/dev/null
kubectl -n demo delete tr "${TR}" --ignore-not-found >/dev/null

wait_cond() { # <condition-type> <want-status> <timeout-iters>
  local type="$1" want="$2" n="${3:-40}" got=""
  for _ in $(seq "${n}"); do
    got=$(kubectl -n demo get tr "${TR}" -o jsonpath="{.status.conditions[?(@.type==\"${type}\")].status}" 2>/dev/null || true)
    [ "${got}" = "${want}" ] && return 0
    sleep 2
  done
  fail "condition ${type} = '${got}', want '${want}'"
}

echo "==> apply TrafficRoute -> registers with the control plane"
kubectl apply -f demo/trafficroutes/payment-weighted.yaml >/dev/null
wait_cond ControlPlaneRegistered True
wait_cond Programmed True
pass "ControlPlaneRegistered=True, Programmed=True"

echo "==> control plane persisted the route as a policy"
kubectl -n "${NS_SYS}" port-forward svc/kubetraffic-control-plane 18090:8080 >/tmp/kt-p8-cp.log 2>&1 &
cp_pf=$!
trap 'kill ${cp_pf} ${gw_pf:-0} 2>/dev/null || true' EXIT
sleep 3
policy_ns=$(curl -s "http://localhost:18090/api/v1/policies/${TR}?namespace=demo" | jq -r '.namespace')
[ "${policy_ns}" = "demo" ] || fail "control plane has no policy for ${TR} (got namespace='${policy_ns}')"
pass "control plane holds a policy for demo/${TR}"

echo "==> live traffic follows the registered 90/10 weights"
kubectl -n "${NS_DATA}" port-forward svc/kubetraffic-gateway 18091:80 >/tmp/kt-p8-gw.log 2>&1 &
gw_pf=$!
sleep 3
sample() {
  local n="$1" v1=0 v2=0 other=0 v
  for _ in $(seq "${n}"); do
    v=$(curl -s -H 'Host: api.example.com' http://localhost:18091/payment | jq -r '.version // "?"')
    case "${v}" in v1) v1=$((v1+1));; v2) v2=$((v2+1));; *) other=$((other+1));; esac
  done
  echo "${v1} ${v2} ${other}"
}
read -r a1 a2 ao <<<"$(sample 200)"
[ "${ao}" -eq 0 ] || fail "unexpected non-v1/v2 responses"
[ "${a1}" -ge 155 ] && [ "${a1}" -le 195 ] || fail "90/10: v1=${a1}/200 outside tolerance"
pass "traffic ~90/10 (v1=${a1}, v2=${a2})"

echo "==> weight edit -> control plane broadcasts a Decision the controller receives"
kubectl -n demo patch tr "${TR}" --type=merge -p \
  '{"spec":{"routes":[{"path":"/payment","backend":{"service":"payment","port":8080},"versions":[{"name":"v1","weight":50},{"name":"v2","weight":50}],"strategy":{"type":"WEIGHTED"}}]}}' >/dev/null
for _ in $(seq 30); do
  w2=$(kubectl -n demo get tr "${TR}" -o jsonpath='{.status.routes[0].currentWeights[?(@.version=="v2")].weight}' 2>/dev/null || true)
  [ "${w2}" = "50" ] && break
  sleep 2
done
[ "${w2}" = "50" ] || fail "status currentWeights v2 = '${w2}', want 50"
decision_seen=""
for _ in $(seq 30); do
  if kubectl -n "${NS_SYS}" logs -l app.kubernetes.io/component=controller --since=5m 2>/dev/null \
      | grep -q '"received decision".*"name":"'"${TR}"'"'; then
    decision_seen=1
    break
  fi
  sleep 1
done
[ -n "${decision_seen}" ] || fail "controller logs show no received decision for ${TR}"
pass "control plane broadcast a decision; controller's StreamDecisions watcher received it"

read -r b1 b2 bo <<<"$(sample 200)"
[ "${bo}" -eq 0 ] || fail "unexpected non-v1/v2 responses after re-split"
[ "${b1}" -ge 75 ] && [ "${b1}" -le 125 ] || fail "50/50: v1=${b1}/200 outside tolerance"
pass "traffic re-split to ~50/50 (v1=${b1}, v2=${b2})"

echo "==> control-plane outage: last-known-good config is kept, spec change is NOT applied blindly"
kubectl -n "${NS_SYS}" scale deploy/kubetraffic-control-plane --replicas=0 >/dev/null
for _ in $(seq 20); do
  cp_ready=$(kubectl -n "${NS_SYS}" get deploy kubetraffic-control-plane -o jsonpath='{.status.readyReplicas}' 2>/dev/null || true)
  [ -z "${cp_ready}" ] && break
  sleep 2
done
kubectl -n demo patch tr "${TR}" --type=merge -p \
  '{"spec":{"routes":[{"path":"/payment","backend":{"service":"payment","port":8080},"versions":[{"name":"v1","weight":90},{"name":"v2","weight":10}],"strategy":{"type":"WEIGHTED"}}]}}' >/dev/null
wait_cond ControlPlaneRegistered False

read -r c1 c2 co <<<"$(sample 200)"
[ "${co}" -eq 0 ] || fail "unexpected non-v1/v2 responses during outage"
[ "${c1}" -ge 75 ] && [ "${c1}" -le 125 ] || fail "expected the OLD cached 50/50 to still be live, got v1=${c1}/200"
pass "data plane kept serving the last-known-good 50/50 config while the control plane was down (v1=${c1})"

kubectl -n "${NS_SYS}" scale deploy/kubetraffic-control-plane --replicas=2 >/dev/null
kubectl -n "${NS_SYS}" rollout status deploy/kubetraffic-control-plane --timeout=120s >/dev/null

# While the control plane is unreachable the reconciler shortens its requeue
# to ControlPlaneRetryInterval (15s) instead of the full --resync-interval
# (10m), so registration self-heals on its own once the control plane is back
# — no operator action (spec edit) required.
wait_cond ControlPlaneRegistered True 40
pass "control plane recovered; registration self-healed without a spec change"
sleep 3 # let the HAProxy reload for the 90/10 config finish draining the old worker

# Recovery re-registers the CURRENT spec (90/10, set earlier while the control
# plane was down) — the cache was only a bridge during the outage, not a
# permanent override.
read -r d1 d2 do_ <<<"$(sample 200)"
[ "${do_}" -eq 0 ] || fail "unexpected non-v1/v2 responses after recovery"
[ "${d1}" -ge 155 ] && [ "${d1}" -le 195 ] || fail "expected the current spec (90/10) to now apply: v1=${d1}/200 outside tolerance"
pass "traffic converges to the current spec (90/10) once reconnected (v1=${d1}, v2=${d2})"

echo "==> delete calls DeleteRoute and the policy disappears"
# The control-plane port-forward opened earlier is pinned to a pod that no
# longer exists (control-plane was scaled 0 -> 2 above); restart it.
kill "${cp_pf}" 2>/dev/null || true
kubectl -n "${NS_SYS}" port-forward svc/kubetraffic-control-plane 18090:8080 >/tmp/kt-p8-cp.log 2>&1 &
cp_pf=$!
sleep 3

kubectl -n demo delete tr "${TR}" >/dev/null
for _ in $(seq 30); do
  kubectl -n demo get tr "${TR}" >/dev/null 2>&1 || break
  sleep 2
done
if kubectl -n demo get tr "${TR}" >/dev/null 2>&1; then
  fail "TrafficRoute still exists after delete"
fi
del_code=$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:18090/api/v1/policies/${TR}?namespace=demo")
[ "${del_code}" = "404" ] || fail "policy still present after delete (GET -> ${del_code})"
pass "TrafficRoute removed; control-plane policy deleted via the finalizer"

echo
echo "ALL PHASE 8 CHECKS PASSED"
