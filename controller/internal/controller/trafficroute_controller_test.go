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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
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
		Build()
	return &TrafficRouteReconciler{Client: c, Scheme: s, Recorder: rec, ResyncInterval: time.Minute}, rec
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

func TestReconcile_ValidRoute_SetsPendingAndAccepted(t *testing.T) {
	tr := validRoute()
	r, rec := newReconciler(t, tr)

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
	if c := conditionStatus(got, ConditionProgrammed); c != metav1.ConditionFalse {
		t.Fatalf("Programmed = %q, want False", c)
	}
	assertEvent(t, rec, "Accepted")
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
	r, _ := newReconciler(t, tr)

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
