package server

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// durationBuckets matches the Prometheus client default histogram buckets.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// Metrics is a tiny, dependency-free Prometheus text-exposition collector.
// It is deliberately minimal; Phase 12 replaces consumption with a real scrape.
type Metrics struct {
	app, version, pod string

	mu           sync.Mutex
	reqTotal     map[string]int64 // key: "<method>|<status>"
	bucketCounts []uint64         // cumulative counts aligned with durationBuckets
	sum          float64
	count        uint64

	inflight int64 // atomic
}

// NewMetrics returns a collector whose series carry the given constant labels.
func NewMetrics(app, version, pod string) *Metrics {
	return &Metrics{
		app:          app,
		version:      version,
		pod:          pod,
		reqTotal:     make(map[string]int64),
		bucketCounts: make([]uint64, len(durationBuckets)),
	}
}

func (m *Metrics) incInflight() { atomic.AddInt64(&m.inflight, 1) }
func (m *Metrics) decInflight() { atomic.AddInt64(&m.inflight, -1) }

func (m *Metrics) observe(method string, status int, seconds float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.reqTotal[method+"|"+strconv.Itoa(status)]++
	m.sum += seconds
	m.count++
	for i, b := range durationBuckets {
		if seconds <= b {
			m.bucketCounts[i]++
		}
	}
}

// WriteTo renders the current metrics in Prometheus text format.
func (m *Metrics) WriteTo(w io.Writer, ready bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cl := fmt.Sprintf("app=%q,version=%q,pod=%q", m.app, m.version, m.pod)

	fmt.Fprint(w, "# HELP sample_app_build_info Static build information.\n")
	fmt.Fprint(w, "# TYPE sample_app_build_info gauge\n")
	fmt.Fprintf(w, "sample_app_build_info{%s} 1\n", cl)

	readyVal := 0
	if ready {
		readyVal = 1
	}
	fmt.Fprint(w, "# HELP sample_app_ready Whether the instance is serving traffic (1) or not (0).\n")
	fmt.Fprint(w, "# TYPE sample_app_ready gauge\n")
	fmt.Fprintf(w, "sample_app_ready{%s} %d\n", cl, readyVal)

	fmt.Fprint(w, "# HELP sample_app_inflight_requests Requests currently being served.\n")
	fmt.Fprint(w, "# TYPE sample_app_inflight_requests gauge\n")
	fmt.Fprintf(w, "sample_app_inflight_requests{%s} %d\n", cl, atomic.LoadInt64(&m.inflight))

	fmt.Fprint(w, "# HELP sample_app_requests_total Total HTTP requests handled.\n")
	fmt.Fprint(w, "# TYPE sample_app_requests_total counter\n")
	keys := make([]string, 0, len(m.reqTotal))
	for k := range m.reqTotal {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts := strings.SplitN(k, "|", 2)
		fmt.Fprintf(w, "sample_app_requests_total{%s,method=%q,status=%q} %d\n",
			cl, parts[0], parts[1], m.reqTotal[k])
	}

	fmt.Fprint(w, "# HELP sample_app_request_duration_seconds HTTP request latency in seconds.\n")
	fmt.Fprint(w, "# TYPE sample_app_request_duration_seconds histogram\n")
	for i, b := range durationBuckets {
		fmt.Fprintf(w, "sample_app_request_duration_seconds_bucket{%s,le=%q} %d\n",
			cl, strconv.FormatFloat(b, 'g', -1, 64), m.bucketCounts[i])
	}
	fmt.Fprintf(w, "sample_app_request_duration_seconds_bucket{%s,le=\"+Inf\"} %d\n", cl, m.count)
	fmt.Fprintf(w, "sample_app_request_duration_seconds_sum{%s} %s\n",
		cl, strconv.FormatFloat(m.sum, 'g', -1, 64))
	fmt.Fprintf(w, "sample_app_request_duration_seconds_count{%s} %d\n", cl, m.count)
}
