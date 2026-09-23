/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

// Package controller holds the TrafficRoute reconciler.
//
// Phase 4: the reconciler validates the spec, then resolves each route's backend
// Service + EndpointSlices into ready endpoints grouped by version, and records
// them in status. It still does NOT program a proxy (Phase 5), so a fully
// resolved route settles at phase=Pending with Programmed=False; a route whose
// backend is missing or has a version with no ready endpoints goes Degraded.
package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
	"github.com/kubetraffic/controller/internal/controlplane"
	"github.com/kubetraffic/controller/internal/discovery"
	kubetrafficv1 "github.com/kubetraffic/controller/internal/grpcapi/kubetrafficv1"
	"github.com/kubetraffic/controller/internal/metrics"
	"github.com/kubetraffic/controller/internal/model"
	"github.com/kubetraffic/controller/internal/proxy"
	webhookv1alpha1 "github.com/kubetraffic/controller/internal/webhook/v1alpha1"
)

const (
	// ConditionAccepted is True once the spec passes semantic validation.
	ConditionAccepted = "Accepted"
	// ConditionResolved is True once every backend version has ready endpoints.
	ConditionResolved = "Resolved"
	// ConditionProgrammed is True once the routing config is applied to the
	// data plane.
	ConditionProgrammed = "Programmed"
	// ConditionControlPlane is True once the route is registered with the
	// Java control plane and its effective weights are known.
	ConditionControlPlane = "ControlPlaneRegistered"

	defaultResyncInterval            = 10 * time.Minute
	defaultControlPlaneRetryInterval = 15 * time.Second
	defaultVersionLabel              = "app.kubernetes.io/version"
	maxConditionMessage              = 512

	backendServiceIndex = ".spec.routes.backend.service"

	// trafficRouteFinalizer ensures DeleteRoute reaches the control plane
	// before a TrafficRoute is removed from etcd.
	trafficRouteFinalizer = "traffic.kubetraffic.io/finalizer"
)

// TrafficRouteReconciler reconciles a TrafficRoute object.
type TrafficRouteReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	ResyncInterval time.Duration

	// Proxy applies the routing model to the data plane. When nil, the
	// reconciler resolves endpoints but leaves Programmed=False.
	Proxy proxy.Proxy

	// ControlPlane registers routes with the Java control plane and returns
	// their authoritative effective weights. When nil, the reconciler falls
	// back to the spec's own weights (Phase 4/5 behaviour).
	ControlPlane controlplane.Client

	// DecisionEvents, when set, is wired into the watch so decisions received
	// out-of-band (see DecisionWatcher) trigger a reconcile.
	DecisionEvents <-chan event.GenericEvent

	// ControlPlaneRetryInterval overrides the requeue delay used while the
	// control plane is unreachable (default 15s; always shorter than
	// ResyncInterval so registration self-heals promptly after an outage).
	ControlPlaneRetryInterval time.Duration

	cpCacheMu sync.Mutex
	cpCache   map[string]*kubetrafficv1.RouteConfig
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
		return r.finalize(ctx, &tr)
	}
	if !controllerutil.ContainsFinalizer(&tr, trafficRouteFinalizer) {
		controllerutil.AddFinalizer(&tr, trafficRouteFinalizer)
		if err := r.Update(ctx, &tr); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
		// Adding a finalizer only touches metadata, which GenerationChangedPredicate
		// filters out of the watch — so fall through and finish reconciling this
		// pass instead of relying on a requeue that would never come.
	}

	logger.Info("reconciling", "generation", tr.Generation, "resourceVersion", tr.ResourceVersion)

	// --- 1. semantic validation --------------------------------------------
	if errs := webhookv1alpha1.ValidateTrafficRoute(&tr); len(errs) > 0 {
		msg := truncate(errs.ToAggregate().Error(), maxConditionMessage)
		r.Recorder.Event(&tr, corev1.EventTypeWarning, "SpecInvalid", msg)
		logger.Info("spec is invalid, holding last good configuration", "error", msg)

		return result, r.applyStatus(ctx, &tr, trafficv1alpha1.PhaseInvalid, tr.Status.Routes,
			metav1.Condition{Type: ConditionAccepted, Status: metav1.ConditionFalse, Reason: "SpecInvalid", Message: msg},
			metav1.Condition{Type: ConditionProgrammed, Status: metav1.ConditionFalse, Reason: "SpecInvalid",
				Message: "spec must be valid before it can be programmed"},
		)
	}

	// --- 2. discovery -----------------------------------------------------
	resolver := &discovery.Resolver{Client: r.Client}
	routeStatuses := make([]trafficv1alpha1.RouteStatus, 0, len(tr.Spec.Routes))
	resolutions := make(map[string]discovery.Resolution, len(tr.Spec.Routes))
	var issues []string

	for i := range tr.Spec.Routes {
		rule := tr.Spec.Routes[i]
		res, err := resolver.Resolve(ctx, tr.Namespace, rule, versionLabelKey(&tr))
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("resolve route %q: %w", routePath(rule), err)
		}
		resolutions[routePath(rule)] = res

		rs := trafficv1alpha1.RouteStatus{
			Path:    routePath(rule),
			Backend: fmt.Sprintf("%s:%d", rule.Backend.Service, rule.Backend.Port),
		}
		switch {
		case !res.ServiceFound:
			issues = append(issues, fmt.Sprintf("route %q: backend Service %q not found", routePath(rule), rule.Backend.Service))
		case !res.PortFound:
			issues = append(issues, fmt.Sprintf("route %q: port %d not found on Service %q", routePath(rule), rule.Backend.Port, rule.Backend.Service))
		}

		for _, es := range res.Versions {
			rs.ResolvedEndpoints = append(rs.ResolvedEndpoints, trafficv1alpha1.VersionEndpoints{
				Version:   es.Version,
				Ready:     es.Ready,
				Total:     es.Ready + es.NotReady,
				Addresses: es.Addresses,
			})
			rs.CurrentWeights = append(rs.CurrentWeights, trafficv1alpha1.VersionWeight{
				Version: es.Version,
				Weight:  specWeight(rule, es.Version),
			})
			if res.ServiceFound && res.PortFound && es.Ready == 0 {
				issues = append(issues, fmt.Sprintf("route %q: version %q has no ready endpoints",
					routePath(rule), versionOrAll(es.Version)))
			}
		}
		if res.Unmatched > 0 {
			issues = append(issues, fmt.Sprintf("route %q: %d ready endpoint(s) match no declared version",
				routePath(rule), res.Unmatched))
		}
		routeStatuses = append(routeStatuses, rs)
	}

	// --- 3. conditions: Accepted + Resolved ------------------------------
	conds := []metav1.Condition{
		{Type: ConditionAccepted, Status: metav1.ConditionTrue, Reason: "Valid", Message: "spec passed semantic validation"},
	}
	phase := trafficv1alpha1.PhasePending
	resolved := len(issues) == 0

	if resolved {
		conds = append(conds, metav1.Condition{
			Type: ConditionResolved, Status: metav1.ConditionTrue, Reason: "EndpointsResolved",
			Message: "all backend versions have ready endpoints",
		})
	} else {
		phase = trafficv1alpha1.PhaseDegraded
		joined := truncate(strings.Join(issues, "; "), maxConditionMessage)
		conds = append(conds, metav1.Condition{
			Type: ConditionResolved, Status: metav1.ConditionFalse, Reason: "EndpointsUnavailable", Message: joined,
		})
		r.Recorder.Event(&tr, corev1.EventTypeWarning, "EndpointsUnavailable", truncate(strings.Join(issues, "; "), 256))
	}

	// --- 4. register with the control plane -----------------------------
	cpCond, cpConfig := r.registerWithControlPlane(ctx, &tr)
	conds = append(conds, cpCond)

	// --- 5. program the data plane, using the control plane's effective
	// weights when available (falling back to the spec's own weights) -----
	effectiveRoutes := applyEffectiveWeights(tr.Spec.Routes, cpConfig)
	progCond, phaseAfterProgram, progErr := r.program(ctx, &tr, resolved, effectiveRoutes, resolutions)
	conds = append(conds, progCond)
	if progErr == nil && resolved && phaseAfterProgram != "" {
		phase = phaseAfterProgram
	} else if progErr != nil {
		phase = trafficv1alpha1.PhaseDegraded
	}

	// --- 5. write status --------------------------------------------
	requeueAfter := r.resync()
	if cpCond.Status == metav1.ConditionFalse && r.ControlPlane != nil {
		// Retry sooner than the full resync interval while the control plane
		// is unreachable, so registration self-heals quickly once it recovers
		// instead of waiting up to --resync-interval.
		requeueAfter = r.controlPlaneRetryInterval()
	}
	result = ctrl.Result{RequeueAfter: requeueAfter}
	if err := r.applyStatus(ctx, &tr, phase, routeStatuses, conds...); err != nil {
		return result, err
	}
	return result, progErr
}

// finalize handles a TrafficRoute pending deletion: it tells the control
// plane the route is gone, then drops the finalizer. Returning an error here
// leaves the finalizer in place and retries with backoff.
func (r *TrafficRouteReconciler) finalize(ctx context.Context, tr *trafficv1alpha1.TrafficRoute) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(tr, trafficRouteFinalizer) {
		return ctrl.Result{}, nil
	}
	nn := types.NamespacedName{Namespace: tr.Namespace, Name: tr.Name}

	if r.ControlPlane != nil {
		if err := r.ControlPlane.DeleteRoute(ctx, nn); err != nil {
			log.FromContext(ctx).Error(err, "failed to delete route from control plane; retrying")
			return ctrl.Result{}, fmt.Errorf("delete route from control plane: %w", err)
		}
	}
	r.clearCachedConfig(nn)

	controllerutil.RemoveFinalizer(tr, trafficRouteFinalizer)
	if err := r.Update(ctx, tr); err != nil {
		return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
	}
	return ctrl.Result{}, nil
}

// registerWithControlPlane upserts tr with the control plane. On failure it
// falls back to the last-known-good RouteConfig (so the data plane keeps
// serving the last approved weights) rather than failing the reconcile.
func (r *TrafficRouteReconciler) registerWithControlPlane(
	ctx context.Context, tr *trafficv1alpha1.TrafficRoute,
) (metav1.Condition, *kubetrafficv1.RouteConfig) {
	if r.ControlPlane == nil {
		return metav1.Condition{
			Type: ConditionControlPlane, Status: metav1.ConditionFalse, Reason: "ControlPlaneNotConfigured",
			Message: "controller started without --control-plane-grpc-url; using spec weights directly",
		}, nil
	}

	nn := types.NamespacedName{Namespace: tr.Namespace, Name: tr.Name}
	cfg, err := r.ControlPlane.RegisterRoute(ctx, tr)
	if err == nil {
		r.setCachedConfig(nn, cfg)
		return metav1.Condition{
			Type: ConditionControlPlane, Status: metav1.ConditionTrue, Reason: "Registered",
			Message: fmt.Sprintf("registered at control-plane generation %d", cfg.GetGeneration()),
		}, cfg
	}

	r.Recorder.Event(tr, corev1.EventTypeWarning, "ControlPlaneUnavailable", truncate(err.Error(), 256))
	if cached := r.cachedConfig(nn); cached != nil {
		return metav1.Condition{
			Type: ConditionControlPlane, Status: metav1.ConditionFalse, Reason: "Unavailable",
			Message: truncate("using last-known-good config: "+err.Error(), maxConditionMessage),
		}, cached
	}
	return metav1.Condition{
		Type: ConditionControlPlane, Status: metav1.ConditionFalse, Reason: "Unavailable",
		Message: truncate(err.Error(), maxConditionMessage),
	}, nil
}

func (r *TrafficRouteReconciler) cachedConfig(nn types.NamespacedName) *kubetrafficv1.RouteConfig {
	r.cpCacheMu.Lock()
	defer r.cpCacheMu.Unlock()
	return r.cpCache[nn.String()]
}

func (r *TrafficRouteReconciler) setCachedConfig(nn types.NamespacedName, cfg *kubetrafficv1.RouteConfig) {
	r.cpCacheMu.Lock()
	defer r.cpCacheMu.Unlock()
	if r.cpCache == nil {
		r.cpCache = map[string]*kubetrafficv1.RouteConfig{}
	}
	r.cpCache[nn.String()] = cfg
}

func (r *TrafficRouteReconciler) clearCachedConfig(nn types.NamespacedName) {
	r.cpCacheMu.Lock()
	defer r.cpCacheMu.Unlock()
	delete(r.cpCache, nn.String())
}

// applyEffectiveWeights overrides each route's declared version weights with
// the control plane's effective ones, matched by path then version name.
// Versions the control plane didn't mention (e.g. it's not configured) keep
// their spec weight untouched.
func applyEffectiveWeights(routes []trafficv1alpha1.RouteRule, cfg *kubetrafficv1.RouteConfig) []trafficv1alpha1.RouteRule {
	if cfg == nil {
		return routes
	}
	byPath := make(map[string]map[string]int32, len(cfg.GetRules()))
	for _, rw := range cfg.GetRules() {
		weights := make(map[string]int32, len(rw.GetWeights()))
		for _, w := range rw.GetWeights() {
			weights[w.GetVersion()] = w.GetWeight()
		}
		byPath[rw.GetPath()] = weights
	}

	out := make([]trafficv1alpha1.RouteRule, len(routes))
	for i, rule := range routes {
		if len(rule.Versions) == 0 {
			out[i] = rule
			continue
		}
		weights, ok := byPath[routePath(rule)]
		if !ok {
			out[i] = rule
			continue
		}
		versions := make([]trafficv1alpha1.BackendVersion, len(rule.Versions))
		copy(versions, rule.Versions)
		for j := range versions {
			if w, ok := weights[versions[j].Name]; ok {
				versions[j].Weight = w
			}
		}
		rule.Versions = versions
		out[i] = rule
	}
	return out
}

// program renders the routing model and applies it to the data plane. It returns
// the Programmed condition, the phase to adopt on success ("" to leave as-is),
// and any apply error (which the caller propagates so the work item is retried).
func (r *TrafficRouteReconciler) program(
	ctx context.Context,
	tr *trafficv1alpha1.TrafficRoute,
	resolved bool,
	effectiveRoutes []trafficv1alpha1.RouteRule,
	resolutions map[string]discovery.Resolution,
) (metav1.Condition, trafficv1alpha1.TrafficRoutePhase, error) {
	if r.Proxy == nil {
		return metav1.Condition{
			Type: ConditionProgrammed, Status: metav1.ConditionFalse, Reason: "ProxyNotConfigured",
			Message: "controller started without --haproxy-dataplane-url",
		}, "", nil
	}
	if !resolved {
		return metav1.Condition{
			Type: ConditionProgrammed, Status: metav1.ConditionFalse, Reason: "EndpointsUnavailable",
			Message: "not programming while endpoints are unavailable; last good config retained",
		}, "", nil
	}

	m := model.Build(tr.Spec.Host, effectiveRoutes, resolutions)
	cfg, err := r.Proxy.GenerateConfig(m)
	if err == nil {
		err = r.Proxy.ApplyConfig(ctx, cfg)
	}
	if err != nil {
		r.Recorder.Event(tr, corev1.EventTypeWarning, "ProxyError", truncate(err.Error(), 256))
		return metav1.Condition{
			Type: ConditionProgrammed, Status: metav1.ConditionFalse, Reason: "ProxyError",
			Message: truncate(err.Error(), maxConditionMessage),
		}, "", fmt.Errorf("apply proxy config: %w", err)
	}

	r.Recorder.Event(tr, corev1.EventTypeNormal, "Programmed",
		fmt.Sprintf("applied routing config %s to the data plane", cfg.Sum()[:12]))
	return metav1.Condition{
		Type: ConditionProgrammed, Status: metav1.ConditionTrue, Reason: "Programmed",
		Message: fmt.Sprintf("data plane serving config %s", cfg.Sum()[:12]),
	}, trafficv1alpha1.PhaseReady, nil
}

// applyStatus writes phase, observedGeneration, routes and the given conditions,
// but only if something actually changed (avoids a write/reconcile loop).
func (r *TrafficRouteReconciler) applyStatus(
	ctx context.Context,
	tr *trafficv1alpha1.TrafficRoute,
	phase trafficv1alpha1.TrafficRoutePhase,
	routes []trafficv1alpha1.RouteStatus,
	conditions ...metav1.Condition,
) error {
	updated := tr.DeepCopy()
	updated.Status.Phase = phase
	updated.Status.ObservedGeneration = tr.Generation
	updated.Status.Routes = routes
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

func (r *TrafficRouteReconciler) controlPlaneRetryInterval() time.Duration {
	if r.ControlPlaneRetryInterval > 0 {
		return r.ControlPlaneRetryInterval
	}
	return defaultControlPlaneRetryInterval
}

// SetupWithManager wires the reconciler and its secondary watches.
func (r *TrafficRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(),
		&trafficv1alpha1.TrafficRoute{}, backendServiceIndex, indexByBackendService); err != nil {
		return err
	}

	bld := ctrl.NewControllerManagedBy(mgr).
		For(&trafficv1alpha1.TrafficRoute{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(r.mapService)).
		Watches(&discoveryv1.EndpointSlice{}, handler.EnqueueRequestsFromMapFunc(r.mapEndpointSlice)).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(r.mapNamespace),
			builder.WithPredicates(podReadinessChanged())).
		Named("trafficroute")

	if r.DecisionEvents != nil {
		bld = bld.WatchesRawSource(source.Channel(r.DecisionEvents, &handler.EnqueueRequestForObject{}))
	}
	return bld.Complete(r)
}

// indexByBackendService lets us look up TrafficRoutes by the Service names they
// reference, so Service/EndpointSlice events fan out only to relevant routes.
func indexByBackendService(obj client.Object) []string {
	tr, ok := obj.(*trafficv1alpha1.TrafficRoute)
	if !ok {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, rule := range tr.Spec.Routes {
		if rule.Backend.Service == "" {
			continue
		}
		if _, dup := seen[rule.Backend.Service]; dup {
			continue
		}
		seen[rule.Backend.Service] = struct{}{}
		out = append(out, rule.Backend.Service)
	}
	return out
}

func (r *TrafficRouteReconciler) routesForService(ctx context.Context, namespace, name string) []reconcile.Request {
	var list trafficv1alpha1.TrafficRouteList
	if err := r.List(ctx, &list,
		client.InNamespace(namespace),
		client.MatchingFields{backendServiceIndex: name},
	); err != nil {
		return nil
	}
	return toRequests(list.Items)
}

func (r *TrafficRouteReconciler) mapService(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.routesForService(ctx, obj.GetNamespace(), obj.GetName())
}

func (r *TrafficRouteReconciler) mapEndpointSlice(ctx context.Context, obj client.Object) []reconcile.Request {
	svc := obj.GetLabels()[discoveryv1.LabelServiceName]
	if svc == "" {
		return nil
	}
	return r.routesForService(ctx, obj.GetNamespace(), svc)
}

// mapNamespace enqueues every TrafficRoute in the object's namespace. Used for
// Pod events, where mapping a pod back to specific routes would mean replaying
// every Service selector; namespaces hold few routes so this is cheap enough.
func (r *TrafficRouteReconciler) mapNamespace(ctx context.Context, obj client.Object) []reconcile.Request {
	var list trafficv1alpha1.TrafficRouteList
	if err := r.List(ctx, &list, client.InNamespace(obj.GetNamespace())); err != nil {
		return nil
	}
	return toRequests(list.Items)
}

func toRequests(items []trafficv1alpha1.TrafficRoute) []reconcile.Request {
	reqs := make([]reconcile.Request, 0, len(items))
	for i := range items {
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: items[i].Namespace, Name: items[i].Name,
		}})
	}
	return reqs
}

// podReadinessChanged filters Pod updates down to the ones that can change
// discovery output: readiness flips and IP (re)assignment.
func podReadinessChanged() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPod, ok1 := e.ObjectOld.(*corev1.Pod)
			newPod, ok2 := e.ObjectNew.(*corev1.Pod)
			if !ok1 || !ok2 {
				return false
			}
			return podReady(oldPod) != podReady(newPod) || oldPod.Status.PodIP != newPod.Status.PodIP
		},
	}
}

func podReady(p *corev1.Pod) bool {
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// --- small helpers -------------------------------------------------------

func versionLabelKey(tr *trafficv1alpha1.TrafficRoute) string {
	if tr.Spec.VersionLabel != "" {
		return tr.Spec.VersionLabel
	}
	return defaultVersionLabel
}

func routePath(r trafficv1alpha1.RouteRule) string {
	if r.Path == "" {
		return "/"
	}
	return r.Path
}

func specWeight(r trafficv1alpha1.RouteRule, version string) int32 {
	if len(r.Versions) == 0 {
		return 100
	}
	for _, v := range r.Versions {
		if v.Name == version {
			return v.Weight
		}
	}
	return 0
}

func versionOrAll(v string) string {
	if v == "" {
		return "<all>"
	}
	return v
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
