/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
	"github.com/kubetraffic/controller/internal/proxy"
)

func newReconciler(t *testing.T, objs ...client.Object) (*TrafficRouteReconciler, *record.FakeRecorder) {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := trafficv1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	rec := record.NewFakeRecorder(32)
	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objs...).
		WithStatusSubresource(&trafficv1alpha1.TrafficRoute{}).
		WithIndex(&trafficv1alpha1.TrafficRoute{}, backendServiceIndex, indexByBackendService).
		Build()
	return &TrafficRouteReconciler{Client: c, Scheme: s, Recorder: rec, ResyncInterval: time.Minute}, rec
}

func paymentService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "payment", Namespace: "demo"},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app.kubernetes.io/name": "payment"},
			Ports:    []corev1.ServicePort{{Name: "http", Port: 8080, TargetPort: intstr.FromString("http")}},
		},
	}
}

func readyPod(name, version string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "demo",
			Labels: map[string]string{"app.kubernetes.io/name": "payment", "app.kubernetes.io/version": version},
		},
		Status: corev1.PodStatus{
			PodIP:      "10.1.0." + name[len(name)-1:],
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		},
	}
}

func paymentSlice(eps ...discoveryv1.Endpoint) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: "payment-xyz", Namespace: "demo",
			Labels: map[string]string{discoveryv1.LabelServiceName: "payment"},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   eps,
	}
}

func ep(addr, podName string) discoveryv1.Endpoint {
	ready := true
	return discoveryv1.Endpoint{
		Addresses:  []string{addr},
		Conditions: discoveryv1.EndpointConditions{Ready: &ready},
		TargetRef:  &corev1.ObjectReference{Kind: "Pod", Name: podName, Namespace: "demo"},
	}
}

func validRoute() *trafficv1alpha1.TrafficRoute {
	return &trafficv1alpha1.TrafficRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "payment-route", Namespace: "demo", Generation: 1},
		Spec: trafficv1alpha1.TrafficRouteSpec{
			Host: "api.example.com",
			Routes: []trafficv1alpha1.RouteRule{{
				Path:    "/payment",
				Backend: trafficv1alpha1.BackendRef{Service: "payment", Port: 8080},
				Versions: []trafficv1alpha1.BackendVersion{
					{Name: "v1", Weight: 90},
					{Name: "v2", Weight: 10},
				},
			}},
		},
	}
}

func request(tr *trafficv1alpha1.TrafficRoute) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: tr.Name, Namespace: tr.Namespace}}
}

func TestReconcile_MissingObject_NoError(t *testing.T) {
	r, _ := newReconciler(t)
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "gone", Namespace: "demo"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != (ctrl.Result{}) {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestReconcile_ResolvedRoute_SetsPending(t *testing.T) {
	tr := validRoute()
	r, _ := newReconciler(t, tr,
		paymentService(),
		readyPod("p1", "v1"), readyPod("p2", "v2"),
		paymentSlice(ep("10.1.0.1", "p1"), ep("10.1.0.2", "p2")),
	)

	res, err := r.Reconcile(context.Background(), request(tr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RequeueAfter != time.Minute {
		t.Fatalf("RequeueAfter = %v, want 1m", res.RequeueAfter)
	}

	var got trafficv1alpha1.TrafficRoute
	if err := r.Get(context.Background(), request(tr).NamespacedName, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != trafficv1alpha1.PhasePending {
		t.Fatalf("phase = %q, want Pending", got.Status.Phase)
	}
	if got.Status.ObservedGeneration != 1 {
		t.Fatalf("observedGeneration = %d, want 1", got.Status.ObservedGeneration)
	}
	if c := conditionStatus(got, ConditionAccepted); c != metav1.ConditionTrue {
		t.Fatalf("Accepted = %q, want True", c)
	}
	if c := conditionStatus(got, ConditionResolved); c != metav1.ConditionTrue {
		t.Fatalf("Resolved = %q, want True", c)
	}
	if c := conditionStatus(got, ConditionProgrammed); c != metav1.ConditionFalse {
		t.Fatalf("Programmed = %q, want False", c)
	}

	if len(got.Status.Routes) != 1 {
		t.Fatalf("status.routes len = %d, want 1", len(got.Status.Routes))
	}
	rs := got.Status.Routes[0]
	if rs.Path != "/payment" || rs.Backend != "payment:8080" {
		t.Fatalf("route status = %+v", rs)
	}
	got1 := endpointsFor(rs, "v1")
	got2 := endpointsFor(rs, "v2")
	if got1.Ready != 1 || got1.Total != 1 {
		t.Fatalf("v1 endpoints = %+v, want ready 1/total 1", got1)
	}
	if got2.Ready != 1 {
		t.Fatalf("v2 endpoints = %+v, want ready 1", got2)
	}
	if w := weightFor(rs, "v1"); w != 90 {
		t.Fatalf("v1 weight = %d, want 90", w)
	}
}

func TestReconcile_WithProxy_SetsReadyAndProgrammed(t *testing.T) {
	tr := validRoute()
	r, rec := newReconciler(t, tr,
		paymentService(),
		readyPod("p1", "v1"), readyPod("p2", "v2"),
		paymentSlice(ep("10.1.0.1", "p1"), ep("10.1.0.2", "p2")),
	)
	fake := proxy.NewFake()
	r.Proxy = fake

	if _, err := r.Reconcile(context.Background(), request(tr)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got trafficv1alpha1.TrafficRoute
	if err := r.Get(context.Background(), request(tr).NamespacedName, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != trafficv1alpha1.PhaseReady {
		t.Fatalf("phase = %q, want Ready", got.Status.Phase)
	}
	if c := conditionStatus(got, ConditionProgrammed); c != metav1.ConditionTrue {
		t.Fatalf("Programmed = %q, want True", c)
	}
	if fake.ApplyCount() != 1 {
		t.Fatalf("ApplyConfig called %d times, want 1", fake.ApplyCount())
	}
	if !strings.Contains(fake.LastApplied().Raw, "backend kt_be_") {
		t.Fatalf("applied config missing a backend:\n%s", fake.LastApplied().Raw)
	}
	assertEvent(t, rec, "Programmed")
}

func TestReconcile_ProxyError_Degraded(t *testing.T) {
	tr := validRoute()
	r, rec := newReconciler(t, tr,
		paymentService(),
		readyPod("p1", "v1"), readyPod("p2", "v2"),
		paymentSlice(ep("10.1.0.1", "p1"), ep("10.1.0.2", "p2")),
	)
	fake := proxy.NewFake()
	fake.ApplyErr = context.DeadlineExceeded
	r.Proxy = fake

	_, err := r.Reconcile(context.Background(), request(tr))
	if err == nil {
		t.Fatal("expected the reconcile to return the proxy error for retry")
	}

	var got trafficv1alpha1.TrafficRoute
	if err := r.Get(context.Background(), request(tr).NamespacedName, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != trafficv1alpha1.PhaseDegraded {
		t.Fatalf("phase = %q, want Degraded", got.Status.Phase)
	}
	if c := conditionStatus(got, ConditionProgrammed); c != metav1.ConditionFalse {
		t.Fatalf("Programmed = %q, want False", c)
	}
	assertEvent(t, rec, "ProxyError")
}

func TestReconcile_DegradedWhenVersionHasNoEndpoints(t *testing.T) {
	tr := validRoute()
	r, rec := newReconciler(t, tr,
		paymentService(),
		readyPod("p1", "v1"),
		paymentSlice(ep("10.1.0.1", "p1")), // nothing for v2
	)

	if _, err := r.Reconcile(context.Background(), request(tr)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got trafficv1alpha1.TrafficRoute
	if err := r.Get(context.Background(), request(tr).NamespacedName, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != trafficv1alpha1.PhaseDegraded {
		t.Fatalf("phase = %q, want Degraded", got.Status.Phase)
	}
	if c := conditionStatus(got, ConditionResolved); c != metav1.ConditionFalse {
		t.Fatalf("Resolved = %q, want False", c)
	}
	assertEvent(t, rec, "EndpointsUnavailable")
}

func TestReconcile_DegradedWhenServiceMissing(t *testing.T) {
	tr := validRoute()
	r, _ := newReconciler(t, tr) // no Service

	if _, err := r.Reconcile(context.Background(), request(tr)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got trafficv1alpha1.TrafficRoute
	if err := r.Get(context.Background(), request(tr).NamespacedName, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != trafficv1alpha1.PhaseDegraded {
		t.Fatalf("phase = %q, want Degraded", got.Status.Phase)
	}
	msg := conditionMessage(got, ConditionResolved)
	if !strings.Contains(msg, "not found") {
		t.Fatalf("Resolved message = %q, want it to mention 'not found'", msg)
	}
}

func TestMapEndpointSlice_EnqueuesOwningRoutes(t *testing.T) {
	tr := validRoute()
	r, _ := newReconciler(t, tr)

	reqs := r.mapEndpointSlice(context.Background(), paymentSlice())
	if len(reqs) != 1 || reqs[0].Name != tr.Name || reqs[0].Namespace != tr.Namespace {
		t.Fatalf("mapEndpointSlice = %+v, want one request for %s/%s", reqs, tr.Namespace, tr.Name)
	}

	// a slice for an unrelated service enqueues nothing
	other := paymentSlice()
	other.Labels[discoveryv1.LabelServiceName] = "unrelated"
	if reqs := r.mapEndpointSlice(context.Background(), other); len(reqs) != 0 {
		t.Fatalf("mapEndpointSlice for unrelated service = %+v, want none", reqs)
	}
}

func TestReconcile_InvalidRoute_SetsInvalid(t *testing.T) {
	tr := validRoute()
	tr.Spec.Routes[0].Versions[1].Weight = 40 // sum = 130
	r, rec := newReconciler(t, tr)

	if _, err := r.Reconcile(context.Background(), request(tr)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got trafficv1alpha1.TrafficRoute
	if err := r.Get(context.Background(), request(tr).NamespacedName, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != trafficv1alpha1.PhaseInvalid {
		t.Fatalf("phase = %q, want Invalid", got.Status.Phase)
	}
	if c := conditionStatus(got, ConditionAccepted); c != metav1.ConditionFalse {
		t.Fatalf("Accepted = %q, want False", c)
	}
	assertEvent(t, rec, "SpecInvalid")
}

func TestReconcile_Idempotent_NoSecondWrite(t *testing.T) {
	tr := validRoute()
	r, _ := newReconciler(t, tr,
		paymentService(),
		readyPod("p1", "v1"), readyPod("p2", "v2"),
		paymentSlice(ep("10.1.0.1", "p1"), ep("10.1.0.2", "p2")),
	)

	if _, err := r.Reconcile(context.Background(), request(tr)); err != nil {
		t.Fatal(err)
	}
	var first trafficv1alpha1.TrafficRoute
	if err := r.Get(context.Background(), request(tr).NamespacedName, &first); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Reconcile(context.Background(), request(tr)); err != nil {
		t.Fatal(err)
	}
	var second trafficv1alpha1.TrafficRoute
	if err := r.Get(context.Background(), request(tr).NamespacedName, &second); err != nil {
		t.Fatal(err)
	}

	if first.ResourceVersion != second.ResourceVersion {
		t.Fatalf("status was rewritten on a no-op reconcile: %s -> %s",
			first.ResourceVersion, second.ResourceVersion)
	}
}

func conditionStatus(tr trafficv1alpha1.TrafficRoute, condType string) metav1.ConditionStatus {
	for _, c := range tr.Status.Conditions {
		if c.Type == condType {
			return c.Status
		}
	}
	return "<absent>"
}

func conditionMessage(tr trafficv1alpha1.TrafficRoute, condType string) string {
	for _, c := range tr.Status.Conditions {
		if c.Type == condType {
			return c.Message
		}
	}
	return ""
}

func endpointsFor(rs trafficv1alpha1.RouteStatus, version string) trafficv1alpha1.VersionEndpoints {
	for _, e := range rs.ResolvedEndpoints {
		if e.Version == version {
			return e
		}
	}
	return trafficv1alpha1.VersionEndpoints{Version: "<absent>"}
}

func weightFor(rs trafficv1alpha1.RouteStatus, version string) int32 {
	for _, w := range rs.CurrentWeights {
		if w.Version == version {
			return w.Weight
		}
	}
	return -1
}

func assertEvent(t *testing.T, rec *record.FakeRecorder, wantReason string) {
	t.Helper()
	select {
	case e := <-rec.Events:
		if !strings.Contains(e, wantReason) {
			t.Fatalf("event %q does not mention %q", e, wantReason)
		}
	default:
		t.Fatalf("expected an event mentioning %q, got none", wantReason)
	}
}
