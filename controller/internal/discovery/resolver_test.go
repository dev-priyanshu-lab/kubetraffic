/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

package discovery

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
)

const ns = "demo"

func boolp(b bool) *bool { return &b }

func fakeClient(objs ...client.Object) client.Client {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	return fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
}

func svc(name string, port int32) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app.kubernetes.io/name": name},
			Ports: []corev1.ServicePort{{
				Name: "http", Port: port, TargetPort: intstr.FromString("http"),
			}},
		},
	}
}

func pod(name, version string, ready bool) *corev1.Pod {
	st := corev1.ConditionFalse
	if ready {
		st = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: ns,
			Labels: map[string]string{"app.kubernetes.io/name": "payment", "app.kubernetes.io/version": version},
		},
		Status: corev1.PodStatus{
			PodIP:      "10.0.0." + name[len(name)-1:],
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: st}},
		},
	}
}

func endpoint(addr, podName string, ready bool) discoveryv1.Endpoint {
	return discoveryv1.Endpoint{
		Addresses:  []string{addr},
		Conditions: discoveryv1.EndpointConditions{Ready: boolp(ready)},
		TargetRef:  &corev1.ObjectReference{Kind: "Pod", Name: podName, Namespace: ns},
	}
}

func slice(name, service string, eps ...discoveryv1.Endpoint) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: ns,
			Labels: map[string]string{discoveryv1.LabelServiceName: service},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   eps,
	}
}

func weightedRule() trafficv1alpha1.RouteRule {
	return trafficv1alpha1.RouteRule{
		Path:    "/payment",
		Backend: trafficv1alpha1.BackendRef{Service: "payment", Port: 8080},
		Versions: []trafficv1alpha1.BackendVersion{
			{Name: "v1", Weight: 90},
			{Name: "v2", Weight: 10},
		},
	}
}

func TestResolve_BucketsByVersion(t *testing.T) {
	c := fakeClient(
		svc("payment", 8080),
		pod("p1", "v1", true), pod("p2", "v1", true),
		pod("p3", "v2", true),
		slice("payment-a", "payment",
			endpoint("10.0.0.1", "p1", true),
			endpoint("10.0.0.2", "p2", true),
			endpoint("10.0.0.3", "p3", true),
		),
	)
	r := &Resolver{Client: c}

	res, err := r.Resolve(context.Background(), ns, weightedRule(), "app.kubernetes.io/version")
	if err != nil {
		t.Fatal(err)
	}
	if !res.ServiceFound || !res.PortFound {
		t.Fatalf("ServiceFound=%v PortFound=%v", res.ServiceFound, res.PortFound)
	}
	if len(res.Versions) != 2 {
		t.Fatalf("want 2 version buckets, got %d", len(res.Versions))
	}
	if res.Versions[0].Version != "v1" || res.Versions[0].Ready != 2 {
		t.Fatalf("v1 = %+v, want ready 2", res.Versions[0])
	}
	if res.Versions[1].Version != "v2" || res.Versions[1].Ready != 1 {
		t.Fatalf("v2 = %+v, want ready 1", res.Versions[1])
	}
	if res.Unmatched != 0 {
		t.Fatalf("Unmatched = %d, want 0", res.Unmatched)
	}
}

func TestResolve_ServiceNotFound(t *testing.T) {
	r := &Resolver{Client: fakeClient()}
	res, err := r.Resolve(context.Background(), ns, weightedRule(), "app.kubernetes.io/version")
	if err != nil {
		t.Fatal(err)
	}
	if res.ServiceFound {
		t.Fatal("ServiceFound = true, want false")
	}
}

func TestResolve_PortNotFound(t *testing.T) {
	r := &Resolver{Client: fakeClient(svc("payment", 9999))}
	res, err := r.Resolve(context.Background(), ns, weightedRule(), "app.kubernetes.io/version")
	if err != nil {
		t.Fatal(err)
	}
	if !res.ServiceFound || res.PortFound {
		t.Fatalf("ServiceFound=%v PortFound=%v, want true/false", res.ServiceFound, res.PortFound)
	}
}

func TestResolve_NotReadyCounted(t *testing.T) {
	c := fakeClient(
		svc("payment", 8080),
		pod("p1", "v1", true), pod("p2", "v1", false),
		pod("p3", "v2", true),
		slice("payment-a", "payment",
			endpoint("10.0.0.1", "p1", true),
			endpoint("10.0.0.2", "p2", false),
			endpoint("10.0.0.3", "p3", true),
		),
	)
	res, err := (&Resolver{Client: c}).Resolve(context.Background(), ns, weightedRule(), "app.kubernetes.io/version")
	if err != nil {
		t.Fatal(err)
	}
	if res.Versions[0].Ready != 1 || res.Versions[0].NotReady != 1 {
		t.Fatalf("v1 = %+v, want ready 1 / notReady 1", res.Versions[0])
	}
	if got := res.Versions[0].Ready + res.Versions[0].NotReady; got != 2 {
		t.Fatalf("v1 total = %d, want 2", got)
	}
}

func TestResolve_NoVersionsSingleBucket(t *testing.T) {
	rule := trafficv1alpha1.RouteRule{
		Path:    "/",
		Backend: trafficv1alpha1.BackendRef{Service: "payment", Port: 8080},
	}
	c := fakeClient(
		svc("payment", 8080),
		slice("payment-a", "payment",
			endpoint("10.0.0.1", "p1", true),
			endpoint("10.0.0.2", "p2", true),
		),
	)
	res, err := (&Resolver{Client: c}).Resolve(context.Background(), ns, rule, "app.kubernetes.io/version")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Versions) != 1 || res.Versions[0].Version != "" || res.Versions[0].Ready != 2 {
		t.Fatalf("buckets = %+v, want one '' bucket with ready 2", res.Versions)
	}
}

func TestResolve_UnmatchedWhenPodMissing(t *testing.T) {
	c := fakeClient(
		svc("payment", 8080),
		// no pods in the cache
		slice("payment-a", "payment", endpoint("10.0.0.9", "ghost", true)),
	)
	res, err := (&Resolver{Client: c}).Resolve(context.Background(), ns, weightedRule(), "app.kubernetes.io/version")
	if err != nil {
		t.Fatal(err)
	}
	if res.Unmatched != 1 {
		t.Fatalf("Unmatched = %d, want 1", res.Unmatched)
	}
	if res.Versions[0].Ready != 0 || res.Versions[1].Ready != 0 {
		t.Fatalf("expected no attributed endpoints, got %+v", res.Versions)
	}
}
