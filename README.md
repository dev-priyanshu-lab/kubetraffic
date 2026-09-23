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

---

## Phase 3 — Go controller skeleton

### What this phase delivers

* A controller-runtime **manager** (`controller/cmd/manager/`) — scheme
  registration, plain-HTTP metrics server, `/healthz` + `/readyz` probes, leader
  election (`--leader-elect`), informer resync period, structured `zap` logging,
  signal-driven graceful shutdown that releases the lease.
* A `TrafficRouteReconciler` (`controller/internal/controller/`) that watches
  `TrafficRoute` (generation-changed predicate), runs the `ValidateTrafficRoute`
  backstop, and maintains `status`: `phase`, `observedGeneration`, and the
  `Accepted` / `Programmed` conditions. It is **idempotent** — a no-op reconcile
  writes nothing. It does **not** resolve endpoints or program a proxy yet, so a
  valid route settles at `phase=Pending`, `Programmed=False`.
* Prometheus collectors (`controller/internal/metrics/`):
  `traffic_controller_reconcile_total`, `..._errors_total`,
  `..._duration_seconds`, registered on the controller-runtime registry.
* Generated RBAC (`config/rbac/role.yaml`) + static ServiceAccount, bindings and
  a namespaced leader-election Role.
* Install kustomize (`config/default` = crd + rbac + manager), a 2-replica
  Deployment with pod anti-affinity, and a metrics `Service`.

### Commands

```sh
make -C controller test           # go build + unit tests (reconciler, metrics, webhook)
make -C controller run            # run locally against the current kube context
make controller-deploy            # docker build -> kind load -> kubectl apply -k config/default
make controller-verify            # Phase 3 acceptance checks
make controller-undeploy
```

### Expected output — verified here

```
$ make controller-verify
PASS: controller build + unit tests
PASS: controller deployment ready (2/2 replicas)
PASS: leader lease held by kubetraffic-controller-577b5676d6-dnlj9_bd008070-...
PASS: valid route: phase=Pending, observedGeneration=1, Accepted=True
PASS: invalid route: phase=Invalid
PASS: metrics exposed (leader reconcile_total=2, errors_total=0)
ALL PHASE 3 CHECKS PASSED
```

`kubectl -n demo get tr payment-weighted -o jsonpath='{.status}'`:

```json
{
  "phase": "Pending",
  "observedGeneration": 1,
  "conditions": [
    { "type": "Accepted",   "status": "True",  "reason": "Valid" },
    { "type": "Programmed",  "status": "False", "reason": "DiscoveryNotImplemented",
      "message": "controller skeleton: endpoint discovery (Phase 4) and proxy programming (Phase 5) are not yet implemented" }
  ]
}
```

Leader pod `/metrics`: `traffic_controller_reconcile_total 5`,
`traffic_controller_reconcile_errors_total 0`,
`traffic_controller_reconcile_duration_seconds_count 5`.

### Failure scenarios to try

| Action | Expected |
|---|---|
| `kubectl -n kubetraffic-system delete pod <leader-pod>` | standby acquires the Lease within ~15 s; `kubectl -n kubetraffic-system get lease kubetraffic-controller -o jsonpath='{.spec.holderIdentity}'` shows the new holder; no error events |
| `kubectl -n demo edit tr payment-weighted` → break weights to sum 130 | `phase` flips to `Invalid`, `Accepted=False reason=SpecInvalid`, a `Warning/SpecInvalid` event; last good config is retained (nothing else changes) |
| fix the weights again | `phase` returns to `Pending`, `Accepted=True` |
| `kubectl -n demo delete tr payment-weighted` | reconcile returns cleanly (`IgnoreNotFound`); no requeue, no error metric |
| scale the Deployment to 1, then back to 2 | single active reconciler throughout; `reconcile_errors_total` stays 0 |

### Verify before Phase 4

- [ ] `make -C controller test` passes (reconciler + metrics + webhook suites).
- [ ] `make controller-deploy` → `kubectl -n kubetraffic-system get deploy` shows `2/2` ready.
- [ ] `kubectl -n kubetraffic-system get lease kubetraffic-controller` has a non-empty `holderIdentity`.
- [ ] Applying `demo/trafficroutes/payment-weighted.yaml` sets `status.phase=Pending`, `observedGeneration == metadata.generation`, `Accepted=True`, `Programmed=False`.
- [ ] Applying `demo/trafficroutes/invalid-weight-sum.yaml` sets `status.phase=Invalid` (reconciler backstop, webhook not yet deployed).
- [ ] Leader pod `/metrics` exposes `traffic_controller_reconcile_total` > 0 and `..._errors_total == 0`.
- [ ] Deleting the leader pod triggers failover with no traffic to any data plane (there is none yet) and no error events.

Then say **"start phase 4"** (Service + EndpointSlice discovery → resolved endpoints in status).

---

## Phase 4 — Service + EndpointSlice discovery

### What this phase delivers

* `controller/internal/discovery` — a `Resolver` that turns a `RouteRule` into
  ready endpoints grouped by version: **Service** (validate the referenced port)
  → its **EndpointSlices** → each endpoint's target **Pod** (read from the
  informer cache) → the pod's version label → a per-version bucket. Endpoints
  that match no declared version are counted as `Unmatched`.
* The reconciler now records `status.routes[]` — `path`, `backend`,
  `resolvedEndpoints[] {version, ready, total, addresses}`, and
  `currentWeights[]` (still mirrored from the spec; dynamic in Phase 8).
* A new `Resolved` condition and a `Degraded` phase: a missing backend Service,
  a missing port, or any declared version with zero ready endpoints →
  `phase=Degraded`, `Resolved=False`, `Warning/EndpointsUnavailable` event.
  Fully resolved → `phase=Pending`, `Resolved=True` (still `Programmed=False`).
* **Secondary watches** with fan-out mapping: a field index on
  `.spec.routes.backend.service` routes `Service` and `EndpointSlice` events only
  to the owning `TrafficRoute`s; `Pod` events (filtered to readiness / IP
  changes) enqueue every route in the pod's namespace.

### Commands

```sh
make -C controller test        # discovery + reconciler unit tests
make controller-deploy         # rebuild + reload + redeploy
make discovery-verify          # Phase 4 acceptance checks (incl. scale reactions)
```

### Expected output — verified here

```
$ make discovery-verify
PASS: phase (Pending)
PASS: Resolved condition (True)
PASS: resolvedEndpoints: v1=2/2, v2=2/2
PASS: status ready total (4) matches EndpointSlices (4)
PASS: currentWeights: v1=90, v2=10
PASS: phase after scale-to-zero (Degraded)
PASS: v2 ready after scale-to-zero (0)
PASS: phase after recovery (Pending)
PASS: v2 ready after recovery (2)
ALL PHASE 4 CHECKS PASSED
```

`kubectl -n demo get tr payment-weighted -o json | jq .status.routes[0]`:

```json
{
  "path": "/payment",
  "backend": "payment:8080",
  "resolvedEndpoints": [
    { "version": "v1", "ready": 2, "total": 2, "addresses": ["10.244.1.11", "10.244.2.7"] },
    { "version": "v2", "ready": 2, "total": 2, "addresses": ["10.244.1.16", "10.244.2.17"] }
  ],
  "currentWeights": [ { "version": "v1", "weight": 90 }, { "version": "v2", "weight": 10 } ]
}
```

### Failure scenarios to try

| Action | Expected |
|---|---|
| `kubectl -n demo scale deploy/payment-v2 --replicas=0` | `phase=Degraded`, `Resolved=False`, `resolvedEndpoints` for `v2` shows `ready: 0`; scale back → recovers within one reconcile |
| `kubectl -n demo delete pod -l app.kubernetes.io/version=v1` | brief dip in `v1.ready`/`addresses` while pods restart, then back to 2 (Pod watch drives the requeue) |
| point a route at a non-existent Service | `phase=Degraded`, `Resolved` message contains `backend Service "…" not found` |
| set `spec.routes[0].backend.port` to a port the Service doesn't expose | `Resolved` message contains `port … not found on Service` |
| add a pod with `app.kubernetes.io/version=v3` (not declared) behind `svc/payment` | its ready endpoint is reported via `… ready endpoint(s) match no declared version` |

### Verify before Phase 5

- [ ] `make -C controller test` passes (`discovery` + `controller` suites).
- [ ] `make discovery-verify` prints `ALL PHASE 4 CHECKS PASSED`.
- [ ] `status.routes[].resolvedEndpoints` ready total equals the `ready==true` endpoint count from `kubectl -n demo get endpointslices -l kubernetes.io/service-name=payment`.
- [ ] Scaling a version to 0 flips `phase` to `Degraded`; scaling back returns it to `Pending` without editing the `TrafficRoute`.
- [ ] `currentWeights` mirror the spec weights.

Then say **"start phase 5"** (HAProxy integration: `Proxy` interface + Data Plane API implementation; render + validate + apply config; `Programmed` finally goes True).

---

## Phase 5 — HAProxy data plane

### What this phase delivers

* `internal/model` — the proxy-agnostic **RoutingModel** (`Host` → `Rule`s →
  weighted `Server`s), built from spec + discovery. Per-version weight is split
  evenly across that version's ready endpoints so the aggregate stays
  proportional regardless of replica count.
* `internal/proxy` — the `Proxy` interface (`GenerateConfig` / `ValidateConfig` /
  `ApplyConfig`), a text/template **Renderer** for HAProxy, a **Data Plane API**
  client using the raw-config endpoint (GET version → compare → POST full file →
  hitless reload; no-op when the live config already matches), and a `Fake` for
  tests.
* Reconciler: after discovery it builds the model, renders, and applies. Success
  → `Programmed=True`, `phase=Ready`, `Programmed` event with the config hash. A
  proxy error → `phase=Degraded`, `Programmed=False`, the error is returned so
  the item is retried. `--haproxy-dataplane-url` unset → `ProxyNotConfigured`
  (Phase 4 behaviour).
* `deployments/haproxy/` — `haproxytech/haproxy-alpine:3.0` in master-worker mode
  with the Data Plane API as a `program`; bootstrap config in a ConfigMap
  (copied to a writable emptyDir by an init container), `kubetraffic-dataplane`
  (`:5555`/`:8404`) and `kubetraffic-gateway` (NodePort 30080 → host `:8080`)
  Services. Dev-only shared credential (`kubetraffic-dev-not-secret`) — Phase 15
  replaces it with TLS + a generated Secret.

### Commands

```sh
make -C controller test
make haproxy-deploy
make controller-deploy       # now passes --haproxy-dataplane-url
make proxy-verify
cd controller && go run ./hack/genconfig   # print the bootstrap config the renderer expects
```

### Expected output — verified here

```
$ make proxy-verify
PASS: haproxy deployment ready
PASS: controller redeployed
PASS: phase=Ready, Programmed=True
PASS: Data Plane API backend present
    v1=270 v2=30 other=0 (of 300)
PASS: weighted split ~90/10 (v1=270, v2=30)
PASS: unknown host -> 503 (kubetraffic_no_route)
ALL PHASE 5 CHECKS PASSED
```

`curl -H 'Host: api.example.com' http://localhost:8080/payment` (via `kubectl -n kubetraffic-data port-forward svc/kubetraffic-gateway 8080:80`) returns `{"version":"v1"}` ~90% of the time, `{"version":"v2"}` ~10%.

### Failure scenarios to try

| Action | Expected |
|---|---|
| `kubectl -n kubetraffic-data delete pod -l app.kubernetes.io/component=data-plane` | HAProxy restarts from the bootstrap ConfigMap; controller detects the config drift on next reconcile and re-pushes; brief connection resets only |
| point a route at a Service with an invalid config (e.g. duplicate backend name via two routes with same host+path) | validation rejects it earlier; a genuine `haproxy -c` failure → `Programmed=False reason=ProxyError`, event, retry with backoff, last good config stays live |
| `kubectl -n demo scale deploy/payment-v2 --replicas=0` | route goes `Degraded`; controller stops pushing (last good config retained); HAProxy `check` also drains the dead servers |
| `curl -H 'Host: unknown' .../payment` | `503` from `kubetraffic_no_route` |

### Verify before Phase 6

- [ ] `make -C controller test` passes (`model`, `proxy`, `controller` suites).
- [ ] `make haproxy-deploy` → `kubetraffic-haproxy` pod `1/1 Ready`.
- [ ] `make proxy-verify` prints `ALL PHASE 5 CHECKS PASSED`.
- [ ] A `TrafficRoute` reaches `phase=Ready` / `Programmed=True`.
- [ ] `GET /v3/services/haproxy/configuration/backends` on the Data Plane API lists a `kt_be_*` backend.
- [ ] Traffic through `kubetraffic-gateway` splits ≈ to the spec weights; unknown host → `503`.

Then say **"start phase 6"** (weighted routing: dynamic weight changes re-program the data plane; exact weight apportionment).

---

## Phase 6 — Weighted routing

### What this phase delivers

* Exact weight **apportionment** (`internal/model`): a version's weight is split
  across its ready endpoints with largest-remainder rounding so the per-server
  weights sum exactly to the spec weight. `weight: 0` parks a version's servers
  (up + health-checked, zero traffic — the canary-at-0 state); `weight > 0`
  guarantees every ready pod gets ≥ 1.
* Confirmation that a `kubectl apply` that only changes weights bumps
  `.metadata.generation`, re-reconciles, re-renders, and the Data Plane API
  performs a **hitless reload** — `status.routes[].currentWeights` and the live
  traffic split both follow, with no HAProxy restart.

### Commands

```sh
make -C controller test        # + apportionment tests
make weighted-verify           # 90/10 -> edit -> 50/50, assert re-split + same HAProxy pod
```

### Expected output — verified here

```
$ make weighted-verify
    90/10 -> v1=270 v2=30 other=0
PASS: status currentWeights updated to 50/50
    50/50 -> v1=200 v2=200 other=0
PASS: traffic re-split to ~50/50 (v1=200, v2=200)
PASS: HAProxy pod unchanged: re-program was a hitless reload
ALL PHASE 6 CHECKS PASSED
```

### Failure scenarios to try

| Action | Expected |
|---|---|
| set weights to `1 / 99` with 3 replicas on the `1` side | every pod still gets `weight 1`; the `1`-side aggregate is slightly inflated (documented trade-off) — no pod is parked |
| set a version to `weight: 0` | its `server` lines render `weight 0`; `curl` never hits that version; the pods stay `check`ed and ready |
| rapid successive weight edits | each generation reconciles; `ApplyConfig` skips when the rendered config already matches the live one (no redundant reload) |

### Verify before Phase 7

- [ ] `make -C controller test` passes (`apportion` sums exactly; `0` parks; tiny weight keeps every server ≥ 1).
- [ ] `make weighted-verify` prints `ALL PHASE 6 CHECKS PASSED`.
- [ ] Editing weights advances `.metadata.generation` and updates `status.routes[].currentWeights`.
- [ ] The HAProxy pod name is unchanged across the re-split (hitless reload, not a restart).

Then say **"start phase 7"** (Java Spring Boot control plane: modules, REST skeleton, PostgreSQL + Flyway, health/readiness).

---

## Phase 7 — Java control plane

### What this phase delivers

* `control-plane/` — a Spring Boot 3.3 / Java 21 service (Maven), packages
  `persistence` (JPA entities + repos), `policy` (service, DTOs, versioning),
  `audit` (append-only trail), `api` (REST + `ProblemDetail` error handling).
* **PostgreSQL 16** (in-cluster `StatefulSet`, official `postgres:16-alpine`
  running as uid 70 under PodSecurity `restricted`) + **Flyway** migration
  `V1__init.sql` (`policy`, `config_version`, `audit_log`). `ddl-auto=validate`.
* REST API: `POST/GET/PUT/DELETE /api/v1/policies/{name}` (with `?namespace=`),
  `GET /api/v1/audit?target=&limit=`. Every mutation writes a `config_version`
  row and an audit entry in one transaction; `PUT` bumps `generation` only when
  the spec actually changes.
* Actuator health/liveness/readiness on `:8081`, Prometheus at
  `/actuator/prometheus`. 2 replicas, pod anti-affinity, graceful shutdown, a
  `wait-for-postgres` init container.
* Tests: `PolicyServiceTest` (Mockito unit — create/get/update/delete/conflict/
  not-found/no-op), `PolicyApiIT` + `AbstractPostgresIT` (Testcontainers
  `@ServiceConnection`, full lifecycle + 404 + 400 + actuator).

### Environment notes

* Local `mvn` must run on **Java 21** (`JAVA_HOME=$(/usr/libexec/java_home -v 21)`) —
  Mockito's inline mock maker breaks on newer JDKs. The Makefile targets set this.
* **Testcontainers ITs need a Docker daemon that speaks API ≤ the pinned
  version.** Docker Engine 29 (which this machine runs) dropped API < 1.44, and
  the bundled docker-java pins 1.43, so `mvn verify` ITs fail *locally* with
  "Could not find a valid Docker environment". They run in CI (Phase 18). The
  Phase 7 gate therefore uses unit tests + a live in-cluster PostgreSQL check.

### Commands

```sh
make control-plane-test            # unit tests (Java 21)
make control-plane-verify-full     # mvn verify incl. Testcontainers ITs (needs compatible Docker)
make control-plane-deploy          # build image -> kind load -> kubectl apply -k deployments/control-plane
make control-plane-e2e-verify      # Phase 7 acceptance checks
```

### Expected output — verified here

```
$ make control-plane-e2e-verify
PASS: control-plane unit tests
PASS: postgres ready
PASS: control-plane ready (2/2)
PASS: Flyway migrated schema (1 migration(s))
PASS: /actuator/health UP
PASS: POST /api/v1/policies -> 201
PASS: duplicate -> 409
PASS: PUT with changed spec -> generation 2
PASS: audit log has N entries for the target
PASS: policy persisted across control-plane pod restart
ALL PHASE 7 CHECKS PASSED
```

### Failure scenarios to try

| Action | Expected |
|---|---|
| `kubectl -n kubetraffic-system delete pod kubetraffic-postgres-0` | control-plane readiness flips to DOWN (DB unreachable) → pods `NotReady`; Postgres restarts from its PVC; data intact; readiness recovers |
| `POST` a policy with a blank `name` | `400` `ProblemDetail` listing the field error |
| `PUT` with the same spec (reformatted) | `200`, `generation` unchanged, no new `config_version` row, no audit entry |
| `GET /api/v1/policies/does-not-exist` | `404` `ProblemDetail` |
| roll both control-plane replicas | stateless — API stays available on the other replica throughout |

### Verify before Phase 8

- [ ] `make control-plane-test` passes (10 unit tests).
- [ ] `make control-plane-deploy` → `kubetraffic-postgres-0` `1/1`, `kubetraffic-control-plane` `2/2`.
- [ ] `flyway_schema_history` has a successful row; `policy` / `config_version` / `audit_log` tables exist.
- [ ] `POST` → `201`, duplicate → `409`, `PUT` bumps `generation`, `DELETE` → `204`, audit reflects the actions.
- [ ] Deleting a control-plane pod does not lose the stored policy.

Then say **"start phase 8"** (gRPC between the Go controller and the Java control plane: `.proto` contract, `RegisterRoute` / `GetRouteConfig` / `StreamDecisions`, the controller consumes control-plane weights).

---

## Phase 8 — gRPC: controller ↔ control plane

### What this phase delivers

* `proto/kubetraffic/v1/route.proto` — the shared contract, generated into both
  languages from the same file: `RouteService` (`RegisterRoute`, `DeleteRoute`,
  `GetRouteConfig`) and `DecisionService` (`StreamDecisions`, server-streaming).
  Go codegen via `controller/hack/gen-proto.sh` (protoc + protoc-gen-go/-grpc);
  Java codegen via the `protobuf-maven-plugin` reading `../proto` at build time
  (`control-plane/Dockerfile` therefore builds from the **repo root**, not
  `control-plane/`, so the shared proto is in its Docker build context).
* **Java (`com.kubetraffic.controlplane.grpc`)**: a hand-rolled `SmartLifecycle`
  Netty gRPC server on `:9090`. `RouteGrpcService` reuses the **Phase 7**
  `PolicyService` for persistence — a route is just a policy keyed by its
  `RouteRef`, with the proto spec stored as JSON (`JsonFormat`). There is no
  rule engine yet (Phase 13), so `RegisterRoute` echoes the submitted weights
  back as the effective `RouteConfig` — but it diffs the new spec against
  whatever was previously stored and **broadcasts a `Decision`** for every
  version whose weight changed, proving the `StreamDecisions` fan-out before
  anything autonomous drives it.
* **Go (`internal/controlplane`)**: a `Client` interface, a real `GRPCClient`,
  and a `Fake` for tests. The reconciler now calls `RegisterRoute` and builds
  the HAProxy routing model from the **control plane's returned weights**, not
  the CRD spec directly (`applyEffectiveWeights`). A `DecisionWatcher`
  (`manager.Runnable`) holds the `StreamDecisions` connection open, reconnecting
  with backoff, and enqueues a reconcile for every route a decision names via a
  `source.Channel` watch.
* **Resilience**: a `trafficroute.kubetraffic.io/finalizer` finalizer calls
  `DeleteRoute` before the CRD is removed. On a `RegisterRoute` failure the
  reconciler falls back to the last-known-good `RouteConfig` cached in memory
  (new `ControlPlaneRegistered` condition = `False`) instead of either failing
  or blindly trusting a spec it couldn't get approved — and requeues after a
  short `ControlPlaneRetryInterval` (15s, not the full 10m resync) so
  registration self-heals quickly once the control plane comes back.

### Commands

```sh
controller/hack/gen-proto.sh        # regenerate Go stubs (Java regenerates on every `mvn compile`)
cd controller && go test ./...      # incl. an in-process bufconn client/server test
cd control-plane && mvn test        # incl. an in-process gRPC RouteGrpcService/DecisionGrpcService test
make control-plane-image            # NOTE: builds with repo root as context (-f control-plane/Dockerfile .)
make -C controller kind-load deploy
make grpc-verify                    # Phase 8 acceptance checks
```

### Expected output — verified here

```
$ make grpc-verify
PASS: control plane (2/2) and controller (2/2) deployed
PASS: ControlPlaneRegistered=True, Programmed=True
PASS: control plane holds a policy for demo/payment-weighted
PASS: traffic ~90/10 (v1=180, v2=20)
PASS: control plane broadcast a decision; controller's StreamDecisions watcher received it
PASS: traffic re-split to ~50/50 (v1=100, v2=100)
PASS: data plane kept serving the last-known-good 50/50 config while the control plane was down (v1=100/101)
PASS: control plane recovered; registration self-healed without a spec change
PASS: traffic converges to the current spec (90/10) once reconnected
PASS: TrafficRoute removed; control-plane policy deleted via the finalizer
ALL PHASE 8 CHECKS PASSED
```

`kubectl -n demo get tr payment-weighted -o json | jq .status.conditions` shows four
conditions once fully up: `Accepted`, `Resolved`, `ControlPlaneRegistered`,
`Programmed` — all `True`.

### Failure scenarios to try

| Action | Expected |
|---|---|
| `kubectl -n kubetraffic-system scale deploy/kubetraffic-control-plane --replicas=0` | `ControlPlaneRegistered=False reason=Unavailable`; HAProxy keeps serving the **last registered** weights (not the raw spec) unchanged; controller retries every 15s |
| edit weights while the control plane is down | the edit is accepted by the API server and shows in `spec`, but is **not** applied to the data plane until the control plane is reachable again and the route is re-registered |
| scale the control plane back up | within ~15s `ControlPlaneRegistered` flips back to `True` and the (now-current) spec's weights take over — no manual nudge needed |
| `kubectl -n demo delete tr payment-weighted` | the finalizer blocks deletion until `DeleteRoute` succeeds; `GET /api/v1/policies/payment-weighted` on the control plane then returns `404` |
| kill a `kubetraffic-controller` pod mid-registration | the standby resumes leadership and re-registers (idempotent upsert) on its next reconcile; no duplicate policies (upsert keys on namespace/name) |

### Verify before Phase 9

- [ ] `go test ./...` (controller) and `mvn test` (control-plane) both green, including the new gRPC-specific suites.
- [ ] `make grpc-verify` prints `ALL PHASE 8 CHECKS PASSED`.
- [ ] A `TrafficRoute`'s `status.conditions` include `ControlPlaneRegistered`; its data-plane weights come from the control plane's response, not directly from `spec`.
- [ ] Stopping the control plane does not stop traffic — it freezes the last-approved config; restarting it resumes registration without operator action.
- [ ] Deleting a `TrafficRoute` deletes its control-plane policy (finalizer-gated).

Then say **"start phase 9"** (PostgreSQL is already in place from Phase 7 — this phase adds Redis for distributed rate-limit counters and shared circuit-breaker state across control-plane replicas).
