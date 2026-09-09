// Package config loads the sample-app runtime configuration from the environment.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config is the fully-resolved runtime configuration.
type Config struct {
	AppName       string        // APP_NAME               logical service name (e.g. "payment")
	AppVersion    string        // APP_VERSION            version label (e.g. "v1")
	Addr          string        // ADDR                   listen address, default ":8080"
	PodName       string        // POD_NAME               downward API, defaults to hostname
	PodNamespace  string        // POD_NAMESPACE          downward API
	NodeName      string        // NODE_NAME              downward API
	ErrorRate     float64       // ERROR_RATE             0.0-1.0 fraction of requests that return 500
	Latency       time.Duration // LATENCY_MS             base artificial latency per request
	LatencyJitter time.Duration // LATENCY_JITTER_MS      random extra latency in [0, jitter)
	ReadyDelay    time.Duration // READY_DELAY_MS         delay before /readyz turns green
	FailReadiness bool          // FAIL_READINESS         if true, /readyz never turns green
	ShutdownGrace time.Duration // SHUTDOWN_GRACE_MS      graceful drain budget, default 10s
}

// FromEnv builds a Config from process environment variables, applying defaults.
func FromEnv() Config {
	host, _ := os.Hostname()
	return Config{
		AppName:       getString("APP_NAME", "sample-app"),
		AppVersion:    getString("APP_VERSION", "v1"),
		Addr:          getString("ADDR", ":8080"),
		PodName:       getString("POD_NAME", host),
		PodNamespace:  getString("POD_NAMESPACE", "default"),
		NodeName:      getString("NODE_NAME", "unknown"),
		ErrorRate:     getFloat("ERROR_RATE", 0),
		Latency:       getDurationMS("LATENCY_MS", 0),
		LatencyJitter: getDurationMS("LATENCY_JITTER_MS", 0),
		ReadyDelay:    getDurationMS("READY_DELAY_MS", 0),
		FailReadiness: getBool("FAIL_READINESS", false),
		ShutdownGrace: getDurationMS("SHUTDOWN_GRACE_MS", 10_000),
	}
}

func getString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getDurationMS(key string, defMS int) time.Duration {
	ms := defMS
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			ms = n
		}
	}
	return time.Duration(ms) * time.Millisecond
}
