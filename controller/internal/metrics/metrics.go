/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

// Package metrics defines the controller's Prometheus collectors. They are
// registered with the controller-runtime metrics registry, so they are exposed
// on the manager's --metrics-bind-address alongside the built-in workqueue and
// controller metrics.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	reconcileTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "traffic_controller_reconcile_total",
		Help: "Total number of TrafficRoute reconciliations.",
	})

	reconcileErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "traffic_controller_reconcile_errors_total",
		Help: "Total number of TrafficRoute reconciliations that returned an error.",
	})

	reconcileDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "traffic_controller_reconcile_duration_seconds",
		Help:    "Wall-clock duration of a TrafficRoute reconciliation.",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	ctrlmetrics.Registry.MustRegister(reconcileTotal, reconcileErrorsTotal, reconcileDuration)
}

// ObserveReconcile records the outcome of a single reconciliation.
func ObserveReconcile(d time.Duration, err error) {
	reconcileTotal.Inc()
	reconcileDuration.Observe(d.Seconds())
	if err != nil {
		reconcileErrorsTotal.Inc()
	}
}
