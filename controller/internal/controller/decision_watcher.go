/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

package controller

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
	"github.com/kubetraffic/controller/internal/controlplane"
	kubetrafficv1 "github.com/kubetraffic/controller/internal/grpcapi/kubetrafficv1"
)

// DecisionWatcher is a manager Runnable that subscribes to the control plane's
// decision stream for the life of the process, reconnecting with backoff, and
// enqueues a reconcile for every route a decision names. It carries no state
// of its own: the reconciler re-registers and re-reads the effective config on
// every reconcile, so "enqueue" is all this needs to do.
type DecisionWatcher struct {
	Client  controlplane.DecisionWatcher
	Trigger chan<- event.GenericEvent

	// MinBackoff/MaxBackoff bound the reconnect delay. Zero uses defaults.
	MinBackoff time.Duration
	MaxBackoff time.Duration
}

// Start implements manager.Runnable.
func (w *DecisionWatcher) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("decision-watcher")
	backoff := w.minBackoff()

	for {
		if ctx.Err() != nil {
			return nil
		}
		logger.Info("connecting to decision stream")
		err := w.Client.WatchDecisions(ctx, func(d *kubetrafficv1.Decision) {
			w.handle(logger, d)
		})
		if ctx.Err() != nil {
			return nil
		}
		logger.Info("decision stream disconnected, reconnecting", "error", err, "backoff", backoff)

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > w.maxBackoff() {
			backoff = w.maxBackoff()
		}
	}
}

func (w *DecisionWatcher) handle(logger interface{ Info(string, ...any) }, d *kubetrafficv1.Decision) {
	ref := d.GetRef()
	logger.Info("received decision",
		"namespace", ref.GetNamespace(), "name", ref.GetName(),
		"path", d.GetPath(), "version", d.GetVersion(),
		"oldWeight", d.GetOldWeight(), "newWeight", d.GetNewWeight(), "reason", d.GetReason())

	w.Trigger <- event.GenericEvent{Object: &trafficv1alpha1.TrafficRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: ref.GetNamespace(), Name: ref.GetName()},
	}}
}

func (w *DecisionWatcher) minBackoff() time.Duration {
	if w.MinBackoff > 0 {
		return w.MinBackoff
	}
	return time.Second
}

func (w *DecisionWatcher) maxBackoff() time.Duration {
	if w.MaxBackoff > 0 {
		return w.MaxBackoff
	}
	return 30 * time.Second
}
