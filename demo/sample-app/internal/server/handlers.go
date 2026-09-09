package server

import (
	"encoding/json"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"time"
)

type response struct {
	App       string    `json:"app"`
	Version   string    `json:"version"`
	Pod       string    `json:"pod"`
	Namespace string    `json:"namespace"`
	Node      string    `json:"node"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Client    string    `json:"client"`
	LatencyMS int64     `json:"latency_ms"`
	Time      time.Time `json:"time"`
	Error     string    `json:"error,omitempty"`
}

type faultRequest struct {
	ErrorRate *float64 `json:"errorRate"`
	LatencyMS *int64   `json:"latencyMs"`
}

type faultResponse struct {
	ErrorRate float64 `json:"errorRate"`
	LatencyMS int64   `json:"latencyMs"`
	JitterMS  int64   `json:"jitterMs"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request) {
	if s.ready.Load() {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("not ready"))
}

// handleFault reports the current fault state, and updates it on POST.
func (s *Server) handleFault(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req faultRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
			return
		}
		rate, latency, _ := s.fault.Snapshot()
		if req.ErrorRate != nil {
			rate = *req.ErrorRate
		}
		if req.LatencyMS != nil {
			latency = time.Duration(*req.LatencyMS) * time.Millisecond
		}
		s.fault.Set(rate, latency)
		s.log.Info("fault state updated", "error_rate", rate, "latency_ms", latency.Milliseconds())
	}

	rate, latency, jitter := s.fault.Snapshot()
	s.writeJSON(w, http.StatusOK, faultResponse{
		ErrorRate: rate,
		LatencyMS: latency.Milliseconds(),
		JitterMS:  jitter.Milliseconds(),
	})
}

// handleRoot is the business endpoint. It applies configured latency/errors and
// echoes identifying metadata so callers can see which instance served them.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	rate, base, jitter := s.fault.Snapshot()

	delay := base
	if jitter > 0 {
		delay += time.Duration(rand.Int64N(int64(jitter) + 1))
	}
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return // client hung up during the artificial delay
		case <-timer.C:
		}
	}

	resp := response{
		App:       s.cfg.AppName,
		Version:   s.cfg.AppVersion,
		Pod:       s.cfg.PodName,
		Namespace: s.cfg.PodNamespace,
		Node:      s.cfg.NodeName,
		Method:    r.Method,
		Path:      r.URL.Path,
		Client:    clientIP(r),
		LatencyMS: delay.Milliseconds(),
		Time:      time.Now().UTC(),
	}

	if rate > 0 && rand.Float64() < rate {
		resp.Error = "injected failure"
		s.writeJSON(w, http.StatusInternalServerError, resp)
		return
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Error("failed to encode response", "err", err)
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
