#!/usr/bin/env bash
# Phase 7 acceptance checks: the Java control plane runs 2 replicas against a real
# in-cluster PostgreSQL, migrates its schema, serves the policy + audit REST API,
# and survives a replica restart with no data loss.
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

echo "==> deploy PostgreSQL + control plane"
make -C . control-plane-image >/dev/null 2>&1 || fail "image build/load failed"
kubectl apply -k deployments/control-plane >/dev/null
kubectl -n "${NS}" rollout status statefulset/kubetraffic-postgres --timeout=180s >/dev/null || fail "postgres not ready"
pass "postgres ready"
kubectl -n "${NS}" rollout status deploy/kubetraffic-control-plane --timeout=180s >/dev/null || fail "control-plane not ready"
ready=$(kubectl -n "${NS}" get deploy kubetraffic-control-plane -o jsonpath='{.status.readyReplicas}')
[ "${ready}" = "2" ] || fail "control-plane readyReplicas=${ready}, want 2"
pass "control-plane ready (2/2)"

echo "==> schema was migrated by Flyway"
mig=$(kubectl -n "${NS}" exec statefulset/kubetraffic-postgres -- \
  env PGPASSWORD=kubetraffic-dev psql -U kubetraffic -d kubetraffic -tAc \
  "select count(*) from flyway_schema_history where success" 2>/dev/null | tr -d '[:space:]')
[ "${mig}" -ge 1 ] 2>/dev/null || fail "flyway_schema_history has no successful migration (got '${mig}')"
tables=$(kubectl -n "${NS}" exec statefulset/kubetraffic-postgres -- \
  env PGPASSWORD=kubetraffic-dev psql -U kubetraffic -d kubetraffic -tAc \
  "select string_agg(table_name,',' order by table_name) from information_schema.tables where table_schema='public'" 2>/dev/null | tr -d '[:space:]')
echo "    tables: ${tables}"
[[ "${tables}" == *"policy"* && "${tables}" == *"config_version"* && "${tables}" == *"audit_log"* ]] \
  || fail "expected policy/config_version/audit_log tables"
pass "Flyway migrated schema (${mig} migration(s))"

echo "==> exercise the REST API"
kubectl -n "${NS}" port-forward svc/kubetraffic-control-plane 18080:8080 18081:8081 >/tmp/kt-cp.log 2>&1 &
pf=$!
trap 'kill ${pf} 2>/dev/null || true' EXIT
sleep 4

health=$(curl -s http://localhost:18081/actuator/health | jq -r '.status')
[ "${health}" = "UP" ] || fail "actuator health = '${health}', want UP"
pass "/actuator/health UP"

# start from a clean slate (the API is idempotent for re-runs)
curl -s -o /dev/null -XDELETE "http://localhost:18080/api/v1/policies/payment-route?namespace=demo" || true

create=$(curl -s -w '\n%{http_code}' -XPOST http://localhost:18080/api/v1/policies \
  -H 'Content-Type: application/json' \
  -d '{"namespace":"demo","name":"payment-route","spec":{"host":"api.example.com","weight":90}}')
code=$(echo "${create}" | tail -1)
[ "${code}" = "201" ] || fail "POST /policies returned ${code}: $(echo "${create}" | head -1)"
pass "POST /api/v1/policies -> 201"

gen=$(curl -s "http://localhost:18080/api/v1/policies/payment-route?namespace=demo" | jq -r '.generation')
[ "${gen}" = "1" ] || fail "generation after create = ${gen}, want 1"

dup_code=$(curl -s -o /dev/null -w '%{http_code}' -XPOST http://localhost:18080/api/v1/policies \
  -H 'Content-Type: application/json' \
  -d '{"namespace":"demo","name":"payment-route","spec":{"host":"x"}}')
[ "${dup_code}" = "409" ] || fail "duplicate POST returned ${dup_code}, want 409"
pass "duplicate -> 409"

put_gen=$(curl -s -XPUT "http://localhost:18080/api/v1/policies/payment-route?namespace=demo" \
  -H 'Content-Type: application/json' \
  -d '{"spec":{"host":"api.example.com","weight":50}}' | jq -r '.generation')
[ "${put_gen}" = "2" ] || fail "generation after PUT = ${put_gen}, want 2"
pass "PUT with changed spec -> generation 2"

audit_n=$(curl -s "http://localhost:18080/api/v1/audit?target=policy/demo/payment-route" | jq 'length')
[ "${audit_n}" -ge 2 ] 2>/dev/null || fail "audit entries = ${audit_n}, want >= 2"
pass "audit log has ${audit_n} entries for the target"

echo "==> data survives a replica restart"
victim=$(kubectl -n "${NS}" get pod -l app.kubernetes.io/component=control-plane -o jsonpath='{.items[0].metadata.name}')
kubectl -n "${NS}" delete pod "${victim}" --wait=false >/dev/null
kubectl -n "${NS}" rollout status deploy/kubetraffic-control-plane --timeout=120s >/dev/null
still=$(curl -s "http://localhost:18080/api/v1/policies/payment-route?namespace=demo" | jq -r '.generation')
[ "${still}" = "2" ] || fail "policy generation after restart = ${still}, want 2 (persistence)"
pass "policy persisted across control-plane pod restart"

echo "==> cleanup"
curl -s -o /dev/null -XDELETE "http://localhost:18080/api/v1/policies/payment-route?namespace=demo" || true

echo
echo "ALL PHASE 7 CHECKS PASSED"
