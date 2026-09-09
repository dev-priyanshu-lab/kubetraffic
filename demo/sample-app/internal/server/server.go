// Package server wires the sample-app HTTP surface, metrics and lifecycle.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/kubetraffic/sample-app/internal/config"
)

// Server is the sample-app HTTP server.
type Server struct {
	cfg     config.Config
	log     *slog.Logger
	fault   *FaultState
	metrics *Metrics
	mux     *http.ServeMux
	ready   atomic.Bool
	start   time.Time
}

// New constructs a Server and registers its routes.
func New(cfg config.Config, log *slog.Logger) *Server {
	s := &Server{
		cfg:     cfg,
		log:     log,
		fault:   NewFaultState(cfg.ErrorRate, cfg.Latency, cfg.LatencyJitter),
		metrics: NewMetrics(cfg.AppName, cfg.AppVersion, cfg.PodName),
		mux:     http.NewServeMux(),
		start:   time.Now(),
	}
	s.routes()
	return s
}

// Handler exposes the router for tests and embedding.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.instrument(s.handleHealth))
	s.mux.HandleFunc("/readyz", s.instrument(s.handleReady))
	s.mux.HandleFunc("/admin/fault", s.instrument(s.handleFault))
	s.mux.HandleFunc("/metrics", s.handleMetrics) // not instrumented: avoids self-noise
	s.mux.HandleFunc("/", s.instrument(s.handleRoot))
}

// Run starts the HTTP listener and blocks until ctx is cancelled, then performs
// a graceful drain bounded by cfg.ShutdownGrace.
func (s *Server) Run(ctx context.Context) error {
	hs := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go s.markReadyAfterDelay(ctx)

	errCh := make(chan error, 1)
	go func() {
		s.log.Info("listening", "addr", s.cfg.Addr, "app", s.cfg.AppName, "version", s.cfg.AppVersion)
		if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.log.Info("shutdown requested, draining connections")
		s.ready.Store(false) // fail readiness first so load balancers stop sending traffic
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownGrace)
		defer cancel()
		return hs.Shutdown(shutdownCtx)
	}
}

func (s *Server) markReadyAfterDelay(ctx context.Context) {
	if s.cfg.FailReadiness {
		s.log.Warn("readiness permanently disabled via FAIL_READINESS")
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(s.cfg.ReadyDelay):
		s.ready.Store(true)
		s.log.Info("service ready", "ready_delay", s.cfg.ReadyDelay.String())
	}
}

func (s *Server) instrument(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		s.metrics.incInflight()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next(rec, r)

		s.metrics.decInflight()
		dur := time.Since(start)
		s.metrics.observe(r.Method, rec.status, dur.Seconds())
		s.log.Info("request",
			"method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration_ms", dur.Milliseconds(), "client", clientIP(r))
	}
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	s.metrics.WriteTo(w, s.ready.Load())
}

// statusRecorder captures the response status code for metrics/logging.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.written {
		r.status = code
		r.written = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.written = true
	return r.ResponseWriter.Write(b)
}
