/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

// Package controller holds the TrafficRoute reconciler.
//
// Phase 3 is a skeleton: it watches TrafficRoute, runs the semantic validation
// backstop, and maintains status (phase, observedGeneration, conditions). It
// does NOT yet resolve endpoints (Phase 4) or program a proxy (Phase 5), so a
// valid route settles at phase=Pending with Programmed=False.
package controller

import (
	"context"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
	"github.com/kubetraffic/controller/internal/metrics"
	webhookv1alpha1 "github.com/kubetraffic/controller/internal/webhook/v1alpha1"
)

const (
	// ConditionAccepted is True once the spec passes semantic validation.
	ConditionAccepted = "Accepted"
	// ConditionProgrammed is True once the routing config has been applied to
	// the data plane. Always False in the Phase 3 skeleton.
	ConditionProgrammed = "Programmed"

	defaultResyncInterval = 10 * time.Minute
	maxConditionMessage   = 512
)

// TrafficRouteReconciler reconciles a TrafficRoute object.
type TrafficRouteReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	ResyncInterval time.Duration
}

// +kubebuilder:rbac:groups=traffic.kubetraffic.io,resources=trafficroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=traffic.kubetraffic.io,resources=trafficroutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=traffic.kubetraffic.io,resources=trafficroutes/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch

// Reconcile brings a single TrafficRoute's status in line with its spec.
func (r *TrafficRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	logger := log.FromContext(ctx)
	start := time.Now()
	defer func() { metrics.ObserveReconcile(time.Since(start), retErr) }()

	var tr trafficv1alpha1.TrafficRoute
	if err := r.Get(ctx, req.NamespacedName, &tr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !tr.DeletionTimestamp.IsZero() {
		// No finalizer yet: proxy/control-plane teardown arrives in Phase 5.
		return ctrl.Result{}, nil
	}

	logger.Info("reconciling", "generation", tr.Generation, "resourceVersion", tr.ResourceVersion)

	if errs := webhookv1alpha1.ValidateTrafficRoute(&tr); len(errs) > 0 {
		msg := truncate(errs.ToAggregate().Error(), maxConditionMessage)
		r.Recorder.Event(&tr, "Warning", "SpecInvalid", msg)
		logger.Info("spec is invalid, holding last good configuration", "error", msg)

		return result, r.applyStatus(ctx, &tr, trafficv1alpha1.PhaseInvalid,
			metav1.Condition{Type: ConditionAccepted, Status: metav1.ConditionFalse, Reason: "SpecInvalid", Message: msg},
			metav1.Condition{Type: ConditionProgrammed, Status: metav1.ConditionFalse, Reason: "SpecInvalid",
				Message: "spec must be valid before it can be programmed"},
		)
	}

	r.Recorder.Event(&tr, "Normal", "Accepted", "TrafficRoute spec accepted")
	result = ctrl.Result{RequeueAfter: r.resync()}

	return result, r.applyStatus(ctx, &tr, trafficv1alpha1.PhasePending,
		metav1.Condition{Type: ConditionAccepted, Status: metav1.ConditionTrue, Reason: "Valid",
			Message: "spec passed semantic validation"},
		metav1.Condition{Type: ConditionProgrammed, Status: metav1.ConditionFalse, Reason: "DiscoveryNotImplemented",
			Message: "controller skeleton: endpoint discovery (Phase 4) and proxy programming (Phase 5) are not yet implemented"},
	)
}

// applyStatus writes phase, observedGeneration and the given conditions, but
// only if something actually changed (avoids a write/reconcile loop).
func (r *TrafficRouteReconciler) applyStatus(
	ctx context.Context,
	tr *trafficv1alpha1.TrafficRoute,
	phase trafficv1alpha1.TrafficRoutePhase,
	conditions ...metav1.Condition,
) error {
	updated := tr.DeepCopy()
	updated.Status.Phase = phase
	updated.Status.ObservedGeneration = tr.Generation
	for _, c := range conditions {
		c.ObservedGeneration = tr.Generation
		meta.SetStatusCondition(&updated.Status.Conditions, c)
	}

	if apiequality.Semantic.DeepEqual(tr.Status, updated.Status) {
		return nil
	}
	if err := r.Status().Update(ctx, updated); err != nil {
		if apierrors.IsConflict(err) {
			return nil // a fresher reconcile is already queued
		}
		return err
	}
	return nil
}

func (r *TrafficRouteReconciler) resync() time.Duration {
	if r.ResyncInterval > 0 {
		return r.ResyncInterval
	}
	return defaultResyncInterval
}

// SetupWithManager wires the reconciler into the manager.
func (r *TrafficRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&trafficv1alpha1.TrafficRoute{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named("trafficroute").
		Complete(r)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
