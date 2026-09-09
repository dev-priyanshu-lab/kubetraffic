# sample-app

The KubeTraffic demo workload. One binary, deployed many times, specialised by
environment variables.

## Endpoints

| Method | Path            | Purpose                                                        |
|-------:|-----------------|--------------------------------------------------------------- |
| GET    | `/`             | Business endpoint. JSON with `app`, `version`, `pod`, `node`.  |
| GET    | `/healthz`      | Liveness. Always `200` while the process runs.                 |
| GET    | `/readyz`       | Readiness. `200` once warm, else `503`.                        |
| GET    | `/metrics`      | Prometheus text exposition.                                    |
| POST   | `/admin/fault`  | Runtime fault injection: `{"errorRate":0.5,"latencyMs":200}`.  |

## Configuration (environment)

| Var                 | Default   | Meaning                                        |
|---------------------|-----------|------------------------------------------------|
| `APP_NAME`          | sample-app| Logical service name                           |
| `APP_VERSION`       | v1        | Version label                                  |
| `ADDR`              | `:8080`   | Listen address                                 |
| `ERROR_RATE`        | `0`       | Fraction `0..1` of requests that return `500`  |
| `LATENCY_MS`        | `0`       | Base artificial latency per request            |
| `LATENCY_JITTER_MS` | `0`       | Random extra latency in `[0, jitter)`          |
| `READY_DELAY_MS`    | `0`       | Delay before `/readyz` turns green             |
| `FAIL_READINESS`    | `false`   | If `true`, `/readyz` never turns green         |
| `SHUTDOWN_GRACE_MS` | `10000`   | Graceful drain budget on SIGTERM               |
| `POD_NAME` / `POD_NAMESPACE` / `NODE_NAME` | (downward API) | Instance identity in responses |

## Run locally

```sh
go test ./...
APP_NAME=payment APP_VERSION=v1 LATENCY_MS=15 go run .
curl -s localhost:8080/ | jq
curl -s -XPOST localhost:8080/admin/fault -d '{"errorRate":0.3}'
```
