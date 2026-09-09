#!/usr/bin/env bash
# Phase 1 acceptance checks: cluster up, demo deployed, services reachable.
set -euo pipefail

NS=demo
DEPLOYMENTS=(payment-v1 payment-v2 service-a service-b service-c)
SERVICES=(payment payment-v1 payment-v2 service-a service-b service-c)

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "PASS: $*"; }

command -v kubectl >/dev/null || fail "kubectl not found"

kubectl get ns "${NS}" >/dev/null 2>&1 || fail "namespace '${NS}' missing (run: make kind-up)"
pass "namespace '${NS}' exists"

for d in "${DEPLOYMENTS[@]}"; do
  kubectl -n "${NS}" rollout status "deploy/${d}" --timeout=90s >/dev/null || fail "deployment '${d}' not ready"
  pass "deployment '${d}' ready"
done

for s in "${SERVICES[@]}"; do
  addrs="$(kubectl -n "${NS}" get endpointslices \
    -l "kubernetes.io/service-name=${s}" \
    -o jsonpath='{range .items[*]}{range .endpoints[*]}{.addresses[0]}{" "}{end}{end}')"
  [ -n "${addrs// /}" ] || fail "service '${s}' has no ready endpoints"
  pass "service '${s}' endpoints: ${addrs}"
done

echo "==> in-cluster HTTP check against payment.${NS}.svc"
# Run the probe pod in 'default' (the 'demo' namespace enforces PodSecurity
# 'restricted', which an ad-hoc `kubectl run` pod would violate).
overrides='{"spec":{"containers":[{"name":"probe","image":"curlimages/curl:8.10.1",
  "command":["curl","-s","--max-time","10","http://payment.'"${NS}"'.svc.cluster.local:8080/"],
  "securityContext":{"allowPrivilegeEscalation":false,"runAsNonRoot":true,"runAsUser":65532,
  "capabilities":{"drop":["ALL"]},"seccompProfile":{"type":"RuntimeDefault"}}}]}}'
out="$(kubectl -n default run "verify-$RANDOM" \
  --image=curlimages/curl:8.10.1 --restart=Never --rm -i --quiet \
  --overrides="${overrides}")"
echo "    ${out}"
echo "${out}" | grep -q '"app":"payment"' || fail "unexpected payload from payment service"
echo "${out}" | grep -Eq '"version":"v[12]"' || fail "payload missing version"
pass "payment service reachable and returns expected payload"

echo
echo "ALL PHASE 1 CHECKS PASSED"
