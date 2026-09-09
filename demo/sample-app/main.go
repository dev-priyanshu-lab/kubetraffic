// Command sample-app is the KubeTraffic demo workload.
//
// A single binary is deployed multiple times (payment v1/v2, service-a/b/c) and
// specialised purely through environment variables. It exposes:
//
//	GET  /            business endpoint, returns JSON identifying the instance
//	GET  /healthz     liveness  (always 200 while the process is up)
//	GET  /readyz      readiness (200 once warm, 503 otherwise)
//	GET  /metrics     Prometheus text exposition
//	POST /admin/fault runtime fault injection ({"errorRate":0.5,"latencyMs":200})
//
// It is intentionally dependency-free (standard library only) so Phase 1 builds
// and runs offline.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/kubetraffic/sample-app/internal/config"
	"github.com/kubetraffic/sample-app/internal/server"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	cfg := config.FromEnv()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := server.New(cfg, log)
	if err := srv.Run(ctx); err != nil {
		log.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	log.Info("server stopped cleanly")
}
