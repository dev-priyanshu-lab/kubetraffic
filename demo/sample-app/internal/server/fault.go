package server

import (
	"sync"
	"time"
)

// FaultState holds the mutable fault-injection configuration. It is safe for
// concurrent use: the request path reads it, /admin/fault writes it.
type FaultState struct {
	mu      sync.RWMutex
	rate    float64       // probability [0,1] that a request returns 500
	latency time.Duration // fixed artificial latency added per request
	jitter  time.Duration // random extra latency in [0, jitter)
}

// NewFaultState seeds the state from static configuration.
func NewFaultState(rate float64, latency, jitter time.Duration) *FaultState {
	return &FaultState{rate: clamp01(rate), latency: max0(latency), jitter: max0(jitter)}
}

// Snapshot returns a consistent copy of the current fault parameters.
func (f *FaultState) Snapshot() (rate float64, latency, jitter time.Duration) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.rate, f.latency, f.jitter
}

// Set replaces the error rate and base latency (jitter is config-only).
func (f *FaultState) Set(rate float64, latency time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rate = clamp01(rate)
	f.latency = max0(latency)
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

func max0(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	return d
}
