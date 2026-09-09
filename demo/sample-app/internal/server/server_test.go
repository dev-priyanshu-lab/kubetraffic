package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kubetraffic/sample-app/internal/config"
)

func newTestServer(t *testing.T, cfg config.Config) *Server {
	t.Helper()
	if cfg.AppName == "" {
		cfg.AppName = "payment"
	}
	if cfg.AppVersion == "" {
		cfg.AppVersion = "v1"
	}
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func do(s *Server, method, target string, body io.Reader) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest(method, target, body))
	return rr
}

func TestHealthzAlwaysOK(t *testing.T) {
	s := newTestServer(t, config.Config{})
	if got := do(s, http.MethodGet, "/healthz", nil).Code; got != http.StatusOK {
		t.Fatalf("healthz = %d, want 200", got)
	}
}

func TestReadyzGating(t *testing.T) {
	s := newTestServer(t, config.Config{})

	if got := do(s, http.MethodGet, "/readyz", nil).Code; got != http.StatusServiceUnavailable {
		t.Fatalf("readyz before warm = %d, want 503", got)
	}
	s.ready.Store(true)
	if got := do(s, http.MethodGet, "/readyz", nil).Code; got != http.StatusOK {
		t.Fatalf("readyz after warm = %d, want 200", got)
	}
}

func TestRootReturnsInstanceIdentity(t *testing.T) {
	s := newTestServer(t, config.Config{AppVersion: "v2"})
	rr := do(s, http.MethodGet, "/payment", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp response
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Version != "v2" {
		t.Fatalf("version = %q, want v2", resp.Version)
	}
	if resp.Path != "/payment" {
		t.Fatalf("path = %q, want /payment", resp.Path)
	}
}

func TestErrorRateForcesFailure(t *testing.T) {
	s := newTestServer(t, config.Config{ErrorRate: 1})
	if got := do(s, http.MethodGet, "/", nil).Code; got != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", got)
	}
}

func TestLatencyIsApplied(t *testing.T) {
	s := newTestServer(t, config.Config{Latency: 40 * time.Millisecond})
	start := time.Now()
	rr := do(s, http.MethodGet, "/", nil)
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("elapsed = %s, want >= 40ms", elapsed)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestFaultEndpointUpdatesState(t *testing.T) {
	s := newTestServer(t, config.Config{})

	rr := do(s, http.MethodPost, "/admin/fault", strings.NewReader(`{"errorRate":1.0}`))
	if rr.Code != http.StatusOK {
		t.Fatalf("fault POST = %d, want 200", rr.Code)
	}
	if got := do(s, http.MethodGet, "/", nil).Code; got != http.StatusInternalServerError {
		t.Fatalf("status after fault = %d, want 500", got)
	}
}

func TestMetricsExposition(t *testing.T) {
	s := newTestServer(t, config.Config{})
	do(s, http.MethodGet, "/", nil) // generate one observation

	body := do(s, http.MethodGet, "/metrics", nil).Body.String()
	for _, want := range []string{
		"sample_app_build_info",
		"sample_app_requests_total",
		"sample_app_request_duration_seconds_bucket",
		`le="+Inf"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics output missing %q\n%s", want, body)
		}
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	s := newTestServer(t, config.Config{Addr: "127.0.0.1:0", ShutdownGrace: time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}
