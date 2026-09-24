# KubeTraffic — Intelligent Kubernetes Traffic Controller

KubeTraffic is a Kubernetes-native L7 traffic management platform. You describe
how traffic should be split across service versions in a single custom
resource — `TrafficRoute` — and KubeTraffic keeps a real HAProxy data plane
programmed to match it: weighted rollouts, canary progression, circuit
breaking, retries and rate limiting, all driven by one declarative spec and a
live control plane instead of hand-edited proxy config.

## Why

Shifting traffic safely between service versions is a problem every team
running Kubernetes eventually hits, and the usual answers are either too
heavy (a full service mesh sidecar on every pod) or too manual (editing
Ingress/HAProxy config by hand and hoping the reload doesn't drop
connections). KubeTraffic sits in between:

- **Declarative, GitOps-friendly** — traffic policy is a CRD, so it lives in
  version control and goes through the same review process as everything
  else, instead of being an out-of-band `kubectl edit configmap`.
- **No sidecars** — one shared HAProxy data plane per cluster/gateway, not an
  extra container per pod. Lower resource overhead, one thing to operate.
- **Progressive delivery built in** — canary ladders, automatic weight
  apportionment across replicas, and (in progress) health-driven
  promote/rollback, without bolting on a separate tool.
- **Resilience where the traffic actually flows** — timeouts, retries and
  circuit breaking are configured per route and rendered straight into the
  data plane, not left to each service to reimplement.
- **Operable by humans** — a web console for anyone who'd rather not memorize
  `kubectl` incantations or gRPC/REST payloads to run a canary.

## Architecture

```
                         ┌─────────────────────────┐
                         │   kubectl apply -f       │
                         │   TrafficRoute (CRD)     │
                         └────────────┬─────────────┘
                                      │ watch
                                      ▼
 ┌───────────────────────────────────────────────────────────────┐
 │                      Go Controller (controller-runtime)         │
 │  discover Services/EndpointSlices/Pods → build routing model    │
 └───────────────┬───────────────────────────────┬─────────────────┘
                  │ gRPC: RegisterRoute /          │ renders + pushes
                  │ GetRouteConfig /                │ HAProxy config via
                  │ StreamDecisions                 │ the Data Plane API
                  ▼                                 ▼
 ┌─────────────────────────────────┐   ┌─────────────────────────────┐
 │   Java Control Plane (Spring)    │   │   HAProxy Data Plane         │
 │  policy · canary · rate-limit ·  │   │  weighted routing, timeouts, │
 │  circuit-breaker · audit         │   │  retries, retry-on           │
 └────────────┬─────────────────────┘   └───────────────┬─────────────┘
              │                                          │
      ┌───────┴────────┐                          live traffic
      ▼                ▼                                 ▼
 ┌─────────┐      ┌─────────┐                    ┌───────────────┐
 │ Postgres │      │  Redis  │                    │ your Services  │
 │ (durable │      │ (shared │                    │  v1 / v2 / …   │
 │  state)  │      │ counters,│                    └───────────────┘
 └─────────┘      │ pub/sub) │
                   └─────────┘
                        ▲
                        │ REST (/api/v1/...)
                        │
              ┌─────────────────────┐
              │  Web Console (React) │
              └─────────────────────┘
```

| Component | Tech | Role |
|---|---|---|
| Go controller | controller-runtime, client-go | Watches `TrafficRoute` + Services/EndpointSlices/Pods, resolves endpoints, renders and pushes HAProxy config |
| Java control plane | Spring Boot 3, Java 21 | Policy storage, canary state machine, rate limiting, circuit breaking, audit trail, gRPC + REST APIs |
| Data plane | HAProxy 3.0 + Data Plane API | Actual L7 routing: weighted backends, timeouts, retries, health checks |
| State | PostgreSQL, Redis | Postgres = durable policy/canary/audit state; Redis = distributed rate-limit counters, shared circuit-breaker state, and pub/sub fan-out of routing decisions across control-plane replicas |
| Console | Vite + React + TypeScript | Web UI for routes, canary control, audit log, and a rate-limit/circuit-breaker playground |

## How it works

1. **You declare intent.** A `TrafficRoute` describes a host, one or more path
   rules, each with a backend Service, a set of weighted versions, a routing
   strategy (`WEIGHTED` / `CANARY` / `BLUE_GREEN`), and optional resilience
   (timeout/retries/circuit breaker), rate-limit and health policy.
2. **The controller resolves reality.** It watches the referenced Service,
   its EndpointSlices, and the backing Pods, groups ready endpoints by
   version label, and builds an in-memory routing model.
3. **The control plane is the source of truth for effective weights.** The
   controller registers the route over gRPC; the control plane persists it as
   a policy, applies any active canary override, and returns the *effective*
   weights — which may differ from the raw spec (e.g. mid-canary at 30%
   instead of the spec's static 90/10).
4. **The controller programs the data plane.** It translates the effective
   config into HAProxy configuration — backends, weighted `server` lines,
   per-backend timeouts and `retry-on` rules — and pushes it through the
   HAProxy Data Plane API as a hitless reload.
5. **Changes propagate without polling.** Canary transitions, rate-limit and
   circuit-breaker state changes are published as `Decision` events over
   Redis pub/sub, fanned out to every control-plane replica's gRPC stream, and
   pushed to every controller replica watching — so a canary `promote` call
   handled by any control-plane pod reaches every controller, which
   re-registers and reprograms HAProxy within moments.
6. **Everything is observable and reversible.** Every policy and canary
   mutation is recorded in an audit log; a canary can be rolled back from any
   state, including after full promotion; the data plane always keeps serving
   the last-known-good config if the control plane is temporarily unreachable
   (fail-safe, not fail-closed).

## Modules & features

**Go controller** (`controller/`)
- `TrafficRoute` CRD with a validating webhook backstop (weight sums, canary
  strategy requirements, ascending canary steps).
- Service/EndpointSlice/Pod discovery keyed by version label, with secondary
  watches so endpoint churn re-triggers reconciliation without polling.
- Exact weight apportionment across replicas (largest-remainder rounding) so
  aggregate traffic share matches the spec regardless of replica count.
- A gRPC client to the control plane with a decision-stream watcher
  (auto-reconnect with backoff) and a last-known-good config cache so a
  control-plane outage never drops live traffic.
- An HAProxy renderer + Data Plane API client: raw-config diff/push, hitless
  reloads, and per-backend timeout/retry/`retry-on` directives translated
  straight from `spec.resilience`.

**Java control plane** (`control-plane/`)
- Policy CRUD with versioning and an append-only audit trail (PostgreSQL +
  Flyway).
- A canary progression state machine (`start` / `promote` / `rollback`)
  walking a configurable weight ladder, with rollback available from any
  state.
- Distributed rate limiting and shared circuit-breaker state (Redis-backed,
  fail-open on Redis outages), exposed as REST for direct use and destined to
  drive real traffic decisions automatically.
- gRPC (`RegisterRoute`, `GetRouteConfig`, `StreamDecisions`) and REST
  (`/api/v1/...`) APIs backed by the same policy/canary services, so the
  controller and the console always see the same effective state.
- Redis pub/sub decision fan-out so every control-plane replica — not just
  the one that handled a given REST call — notifies every controller.

**HAProxy data plane** (`deployments/haproxy/`)
- Master-worker HAProxy with the Data Plane API running as a managed
  `program`, configured entirely via raw-config push (no manual `haproxy.cfg`
  editing).
- Per-backend timeouts, retry counts and `retry-on` rules rendered from the
  CRD's resilience policy; `option redispatch` so retries land on a different
  server, not the one that just failed.

**Web console** (`console/`)
- Routes list and detail view with live, canary-aware effective weights.
- Canary start/promote/rollback controls.
- Global audit log with filtering.
- A playground for exercising the rate-limiter and circuit-breaker primitives
  directly.
- Runtime-configurable API endpoint (`KUBETRAFFIC_API_BASE_URL`, injected at
  container start, not build time) so one built image works against any
  operator's control plane.

## Benefits

- **Faster, safer rollouts** — a canary ladder and automatic rollback path
  turn "change the weight and hope" into a repeatable, auditable procedure.
- **Fewer moving parts than a service mesh** — one data plane per
  cluster/gateway instead of a sidecar per pod, with a much smaller
  operational footprint.
- **Resilience as configuration, not code** — timeouts, retries and circuit
  breaking live in the route spec, not scattered across service
  implementations.
- **Everything traceable** — every policy change and canary transition is
  audited, and the effective config the data plane is running is always
  queryable, not inferred from `kubectl describe` and guesswork.
- **Approachable** — a web console means canary control and rollback don't
  require memorizing API payloads under pressure during an incident.

## Repository layout

```
controller/       Go controller + TrafficRoute CRD, webhook, HAProxy renderer
control-plane/    Java Spring Boot control plane (policy, canary, rate-limit, circuit-breaker, gRPC/REST)
console/          React/TypeScript web UI
proto/            Shared gRPC contract between controller and control plane
deployments/      Kubernetes manifests (kind cluster config, HAProxy, control plane, console)
demo/             Sample workloads + example TrafficRoutes
hack/             Dev/verification scripts
```

## Getting started

Prerequisites: `go`, `docker`, `kind`, `kubectl`, `java` 21, `mvn`, `node`.

```sh
make kind-up              # create the local cluster + sample workloads
make controller-deploy    # build + deploy the Go controller
make control-plane-deploy # build + deploy Postgres, Redis, the Java control plane
make haproxy-deploy       # deploy the HAProxy data plane
make console-deploy       # build + deploy the web console

kubectl apply -f demo/trafficroutes/payment-weighted.yaml
kubectl port-forward -n kubetraffic-system svc/kubetraffic-console 8080:8080
```

Then open `http://localhost:8080` to see the route, its live weights, and the
canary controls.
