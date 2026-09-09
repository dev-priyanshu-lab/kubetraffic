# KubeTraffic — Intelligent Kubernetes Traffic Controller

A Kubernetes-native L7 traffic management platform: discover workloads from the
API server, drive an HAProxy data plane through a `TrafficRoute` CRD, and shift
traffic automatically (weighted / canary / blue-green) based on real-time health.

> **Status:** under construction, built phase by phase.
> Architecture and roadmap live in [`docs/architecture.md`](docs/architecture.md).
> This README is filled out fully in Phase 19.

## Components (target architecture)

| Component | Tech | Role |
|---|---|---|
| Go Controller | controller-runtime, client-go | Watch `TrafficRoute` + core objects, reconcile, program HAProxy |
| Java Control Plane | Spring Boot 3, Java 21 | Policy engine, decision engine, canary state, audit |
| Data plane | HAProxy 3.0 + Data Plane API | L7 routing, weights, timeouts, retries, rate limits |
| State | PostgreSQL, Redis | Durable config/decisions; shared counters + circuit-breaker state |
| Observability | Prometheus, Grafana, OpenTelemetry, Loki | Metrics, dashboards, traces, logs |

## Repository layout

```
controller/       Go controller + TrafficRoute API types      (Phase 3+)
control-plane/    Java Spring Boot control plane               (Phase 7+)
proto/            Shared gRPC contract                         (Phase 8+)
proxy/            HAProxy base config + Data Plane API          (Phase 5+)
deployments/      kind config, raw manifests, Helm chart
observability/    Prometheus / Grafana / OTel / Loki configs   (Phase 12+)
demo/             Sample workloads + example TrafficRoutes
tests/            e2e + failure-injection harness
docs/             architecture.md, ADRs, diagrams
hack/             dev scripts
```

---

## Phase 1 — Local cluster + sample application

### What this phase delivers

* A reproducible 3-node **kind** cluster (`deployments/kind/`).
* A dependency-free Go **sample workload** (`demo/sample-app/`) deployed five ways:
  `payment-v1`, `payment-v2`, `service-a`, `service-b`, `service-c`.
* Kubernetes manifests wiring Deployments + Services with stable
  `app.kubernetes.io/{name,version}` labels — the identifiers every later phase
  keys off for discovery and version splitting.

### Prerequisites

`go` (1.22+), `docker`, `kind`, `kubectl`. Optional: `jq`, `hey`.

### Quick start

```sh
# 1. unit-test the sample app (offline, no cluster needed)
make sample-app-test

# 2. create the cluster, build+load the image, deploy the demo
make kind-up

# 3. acceptance checks
make demo-verify

# 4. hit a service
kubectl -n demo port-forward svc/payment 8080:8080 &
curl -s localhost:8080/ | jq
hack/loadgen.sh http://localhost:8080/
```

> **Note:** `kubectl port-forward svc/...` pins to a single backing pod, so every
> request through it returns the same `version`. To see the v1/v2 split you need
> in-cluster traffic (a curl loop from a pod) — `make demo-verify` and the
> failure-scenario commands below use that.

Tear down with `make kind-down`.

### Expected output

`make demo-verify`:

```
PASS: namespace 'demo' exists
PASS: deployment 'payment-v1' ready
PASS: deployment 'payment-v2' ready
PASS: deployment 'service-a' ready
PASS: deployment 'service-b' ready
PASS: deployment 'service-c' ready
PASS: service 'payment' endpoints: 10.244.1.5 10.244.2.4 10.244.1.6 10.244.2.5
PASS: service 'payment-v1' endpoints: 10.244.1.5 10.244.2.4
...
PASS: payment service reachable and returns expected payload
ALL PHASE 1 CHECKS PASSED
```

`curl -s localhost:8080/ | jq`:

```json
{
  "app": "payment",
  "version": "v1",
  "pod": "payment-v1-6c9d4b8f7c-2xk9p",
  "namespace": "demo",
  "node": "kubetraffic-worker",
  "method": "GET",
  "path": "/",
  "client": "127.0.0.1",
  "latency_ms": 14,
  "time": "2026-09-01T12:00:00Z"
}
```

`hack/loadgen.sh` (curl fallback):

```
status code distribution:
 200 200
```

### Failure scenarios to try

| Action | Expected |
|---|---|
| `kubectl -n demo delete pod -l app.kubernetes.io/name=payment,app.kubernetes.io/version=v1` | Deployment recreates pods; `svc/payment` keeps serving from v2 meanwhile |
| `kubectl -n demo scale deploy/payment-v2 --replicas=0` | `svc/payment` now only returns `"version":"v1"`; `svc/payment-v2` endpoints empty |
| `kubectl -n demo set env deploy/payment-v2 ERROR_RATE=1.0` | ~50% of `svc/payment` calls return `500` once v2 rolls |
| `kubectl -n demo set env deploy/service-b FAIL_READINESS=true` | `service-b` pods go `NotReady`; `svc/service-b` endpoints drain to empty |
| `kubectl -n demo port-forward pod/<payment-v1-pod> 9000:8080` then `curl -XPOST localhost:9000/admin/fault -d '{"latencyMs":800}'` | that pod's responses show `latency_ms: 800` |

### Verify before Phase 2

- [ ] `make sample-app-test` passes.
- [ ] `make kind-up` completes; `kubectl get pods -n demo` shows 7 pods `Running`/`Ready`.
- [ ] `make demo-verify` prints `ALL PHASE 1 CHECKS PASSED`.
- [ ] `kubectl -n demo get endpointslices` lists ready addresses for every Service.
- [ ] `svc/payment` returns both `"version":"v1"` and `"version":"v2"` across repeated calls.
- [ ] `curl localhost:8080/metrics` (via port-forward) returns Prometheus text with
      `sample_app_requests_total` and `sample_app_request_duration_seconds_bucket`.

---

## Phase 2 — `TrafficRoute` CRD

### What this phase delivers

* Go API types for `traffic.kubetraffic.io/v1alpha1` `TrafficRoute`
  (`controller/api/v1alpha1/`) — `host` + ordered `routes[]`, each with a
  `backend`, weighted `versions[]`, a `strategy` (`WEIGHTED` / `CANARY` /
  `BLUE_GREEN`), and optional `resilience` / `security` / `health` policy.
* A rich `status` subresource: `phase`, `observedGeneration`, per-route
  `resolvedEndpoints` / `currentWeights` / `backendHealth`, `lastDecision`, and
  standard `conditions[]`.
* `controller-gen` wired via `controller/Makefile` — generates
  `zz_generated.deepcopy.go` and the CRD OpenAPI schema
  (`config/crd/bases/…trafficroutes.yaml`).
* A **validating-webhook skeleton** (`controller/internal/webhook/v1alpha1/`)
  with the pure, unit-tested `ValidateTrafficRoute` function for the cross-field
  checks a structural schema can't express (weights sum to 100, `CANARY` needs
  ≥2 versions, ascending canary steps, …). Wired into the manager in Phase 3.
* Example manifests in `demo/trafficroutes/` — two valid, three schema-invalid,
  one semantically-invalid (webhook-only).

### Two layers of validation

| Layer | Enforces | Active in |
|---|---|---|
| CRD OpenAPI schema | required fields, enums, numeric ranges, list bounds, patterns | Phase 2 (API server) |
| Validating webhook / `ValidateTrafficRoute` | weights sum to 100, strategy↔versions rules, ascending steps, JWT needs issuer, … | wired Phase 3; logic unit-tested now |

### Commands

```sh
make crd-generate        # deepcopy + CRD/webhook YAML  (installs controller-gen to controller/bin)
make crd-test            # go build + unit tests for the validator
make crd-install         # kubectl apply -f controller/config/crd/bases

# valid — accepted
kubectl apply --dry-run=server -f demo/trafficroutes/payment-weighted.yaml
kubectl apply --dry-run=server -f demo/trafficroutes/payment-canary.yaml

# invalid — rejected by the schema
kubectl apply --dry-run=server -f demo/trafficroutes/invalid-missing-host.yaml
kubectl apply --dry-run=server -f demo/trafficroutes/invalid-bad-strategy.yaml
kubectl apply --dry-run=server -f demo/trafficroutes/invalid-weight-range.yaml
```

### Expected output

`make crd-test` → `ok  github.com/kubetraffic/controller/internal/webhook/v1alpha1`.

```
$ kubectl apply --dry-run=server -f demo/trafficroutes/payment-weighted.yaml
trafficroute.traffic.kubetraffic.io/payment-weighted created (server dry run)

$ kubectl apply --dry-run=server -f demo/trafficroutes/invalid-missing-host.yaml
The TrafficRoute "invalid-missing-host" is invalid: spec.host: Required value

$ kubectl apply --dry-run=server -f demo/trafficroutes/invalid-bad-strategy.yaml
... spec.routes[0].strategy.type: Unsupported value: "ROLLING": supported values: "WEIGHTED", "CANARY", "BLUE_GREEN"

$ kubectl apply --dry-run=server -f demo/trafficroutes/invalid-weight-range.yaml
... spec.routes[0].versions[0].weight: Invalid value: 150: ... should be less than or equal to 100

$ kubectl -n demo get tr
NAME               HOST              PHASE   AGE
payment-canary     api.example.com           0s
payment-weighted   api.example.com           0s
```

`invalid-weight-sum.yaml` **applies** at this phase (each weight is in range) —
the webhook rejects it once deployed in Phase 3.

### Failure scenarios to try

| Action | Expected |
|---|---|
| `kubectl explain trafficroute.spec.routes.strategy` | shows the generated schema with `type` enum |
| add a 4th `version` with `weight: 10` to `payment-weighted.yaml` and apply | schema accepts it; sum is now 110 — a Phase 3 webhook / reconciler backstop will flag it |
| `strategy.type: CANARY` with a single version, dry-run | schema accepts; `ValidateTrafficRoute` (unit-tested) rejects it |
| `kubectl get tr`, `kubectl get troute`, `kubectl get trafficroutes` | all three resolve (short names `tr`, `troute`) |

### Verify before Phase 3

- [ ] `make crd-generate` regenerates `zz_generated.deepcopy.go` + the CRD YAML with no diff churn beyond intended changes.
- [ ] `make crd-test` passes (`ValidateTrafficRoute` table tests + `ValidateCreate` wrapper).
- [ ] `make crd-install` creates `trafficroutes.traffic.kubetraffic.io`.
- [ ] Both `demo/trafficroutes/payment-*.yaml` pass `kubectl apply --dry-run=server`.
- [ ] All three `demo/trafficroutes/invalid-{missing-host,bad-strategy,weight-range}.yaml` are rejected by the API server.
- [ ] `kubectl -n demo get tr` shows the `HOST` / `PHASE` / `AGE` print columns.

Then say **"start phase 3"** (Go controller skeleton: manager, informers, workqueue, no-op reconcile, leader election, `/metrics`).
