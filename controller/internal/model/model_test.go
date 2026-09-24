/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

package model

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
	"github.com/kubetraffic/controller/internal/discovery"
)

func TestBuild_WeightSplitAcrossEndpoints(t *testing.T) {
	rule := trafficv1alpha1.RouteRule{
		Path:    "/payment",
		Backend: trafficv1alpha1.BackendRef{Service: "payment", Port: 8080},
		Versions: []trafficv1alpha1.BackendVersion{
			{Name: "v1", Weight: 90},
			{Name: "v2", Weight: 10},
		},
	}
	res := discovery.Resolution{
		ServiceFound: true, PortFound: true, EndpointPort: 8080,
		Versions: []discovery.EndpointSet{
			{Version: "v1", Ready: 2, Addresses: []string{"10.0.0.1", "10.0.0.2"}},
			{Version: "v2", Ready: 2, Addresses: []string{"10.0.0.3", "10.0.0.4"}},
		},
	}

	m := Build("api.example.com", []trafficv1alpha1.RouteRule{rule}, map[string]discovery.Resolution{"/payment": res})
	if len(m.Rules) != 1 || len(m.Rules[0].Servers) != 4 {
		t.Fatalf("model = %+v", m)
	}

	var v1w, v2w int
	for _, s := range m.Rules[0].Servers {
		if s.Port != 8080 {
			t.Fatalf("server port = %d, want 8080", s.Port)
		}
		switch s.Version {
		case "v1":
			v1w += s.Weight
		case "v2":
			v2w += s.Weight
		}
	}
	// 90/2 + 90/2 = 90 ; 10/2 + 10/2 = 10
	if v1w != 90 || v2w != 10 {
		t.Fatalf("aggregate weights v1=%d v2=%d, want 90/10", v1w, v2w)
	}
}

func TestBuild_MinWeightOne(t *testing.T) {
	rule := trafficv1alpha1.RouteRule{
		Path:    "/x",
		Backend: trafficv1alpha1.BackendRef{Service: "s", Port: 80},
		Versions: []trafficv1alpha1.BackendVersion{
			{Name: "v1", Weight: 99},
			{Name: "v2", Weight: 1},
		},
	}
	res := discovery.Resolution{
		ServiceFound: true, PortFound: true, EndpointPort: 80,
		Versions: []discovery.EndpointSet{
			{Version: "v1", Addresses: []string{"1.1.1.1"}},
			{Version: "v2", Addresses: []string{"2.2.2.2", "3.3.3.3"}}, // 1/2 -> 0 -> clamped to 1
		},
	}
	m := Build("h", []trafficv1alpha1.RouteRule{rule}, map[string]discovery.Resolution{"/x": res})
	for _, s := range m.Rules[0].Servers {
		if s.Weight < 1 {
			t.Fatalf("server %s weight %d < 1", s.Name, s.Weight)
		}
	}
}

func TestBuild_FallsBackToBackendPort(t *testing.T) {
	rule := trafficv1alpha1.RouteRule{
		Path:     "/x",
		Backend:  trafficv1alpha1.BackendRef{Service: "s", Port: 8443},
		Versions: []trafficv1alpha1.BackendVersion{{Name: "v1", Weight: 100}},
	}
	res := discovery.Resolution{
		ServiceFound: true, PortFound: true, // EndpointPort left 0
		Versions: []discovery.EndpointSet{{Version: "v1", Addresses: []string{"1.1.1.1"}}},
	}
	m := Build("h", []trafficv1alpha1.RouteRule{rule}, map[string]discovery.Resolution{"/x": res})
	if m.Rules[0].Servers[0].Port != 8443 {
		t.Fatalf("port = %d, want 8443 fallback", m.Rules[0].Servers[0].Port)
	}
}

func TestApportion_SumsExactly(t *testing.T) {
	cases := []struct {
		weight int32
		n      int
	}{
		{100, 1}, {90, 2}, {10, 2}, {33, 2}, {33, 3}, {50, 4}, {1, 1}, {100, 7},
	}
	for _, c := range cases {
		got := apportion(c.weight, c.n)
		sum := 0
		for _, w := range got {
			sum += w
		}
		if sum != int(c.weight) {
			t.Fatalf("apportion(%d,%d)=%v sum=%d, want %d", c.weight, c.n, got, sum, c.weight)
		}
	}
}

func TestApportion_ZeroWeightParksServers(t *testing.T) {
	got := apportion(0, 3)
	for _, w := range got {
		if w != 0 {
			t.Fatalf("apportion(0,3)=%v, want all zero", got)
		}
	}
}

func TestApportion_TinyWeightKeepsEveryServerReachable(t *testing.T) {
	got := apportion(1, 3) // 1 across 3 -> every server still >= 1
	for _, w := range got {
		if w < 1 {
			t.Fatalf("apportion(1,3)=%v, want every server >= 1", got)
		}
	}
}

func TestBuild_ReflectsChangedWeights(t *testing.T) {
	res := discovery.Resolution{
		ServiceFound: true, PortFound: true, EndpointPort: 8080,
		Versions: []discovery.EndpointSet{
			{Version: "v1", Addresses: []string{"10.0.0.1", "10.0.0.2"}},
			{Version: "v2", Addresses: []string{"10.0.0.3", "10.0.0.4"}},
		},
	}
	mk := func(w1, w2 int32) map[string]int {
		rule := trafficv1alpha1.RouteRule{
			Path:    "/p",
			Backend: trafficv1alpha1.BackendRef{Service: "s", Port: 8080},
			Versions: []trafficv1alpha1.BackendVersion{
				{Name: "v1", Weight: w1}, {Name: "v2", Weight: w2},
			},
		}
		m := Build("h", []trafficv1alpha1.RouteRule{rule}, map[string]discovery.Resolution{"/p": res})
		agg := map[string]int{}
		for _, s := range m.Rules[0].Servers {
			agg[s.Version] += s.Weight
		}
		return agg
	}
	if a := mk(90, 10); a["v1"] != 90 || a["v2"] != 10 {
		t.Fatalf("90/10 -> %v", a)
	}
	if a := mk(50, 50); a["v1"] != 50 || a["v2"] != 50 {
		t.Fatalf("50/50 -> %v", a)
	}
	if a := mk(100, 0); a["v1"] != 100 || a["v2"] != 0 {
		t.Fatalf("100/0 -> %v", a)
	}
}

func TestBuild_ResilienceNilWhenUnspecified(t *testing.T) {
	rule := trafficv1alpha1.RouteRule{
		Path:     "/x",
		Backend:  trafficv1alpha1.BackendRef{Service: "s", Port: 80},
		Versions: []trafficv1alpha1.BackendVersion{{Name: "v1", Weight: 100}},
	}
	res := discovery.Resolution{
		ServiceFound: true, PortFound: true, EndpointPort: 80,
		Versions: []discovery.EndpointSet{{Version: "v1", Addresses: []string{"1.1.1.1"}}},
	}
	m := Build("h", []trafficv1alpha1.RouteRule{rule}, map[string]discovery.Resolution{"/x": res})
	if m.Rules[0].Resilience != nil {
		t.Fatalf("Resilience = %+v, want nil", m.Rules[0].Resilience)
	}
}

func TestBuild_ResilienceTranslatedToHAProxyTerms(t *testing.T) {
	rule := trafficv1alpha1.RouteRule{
		Path:     "/x",
		Backend:  trafficv1alpha1.BackendRef{Service: "s", Port: 80},
		Versions: []trafficv1alpha1.BackendVersion{{Name: "v1", Weight: 100}},
		Resilience: &trafficv1alpha1.ResiliencePolicy{
			Timeout: metav1.Duration{Duration: 2 * time.Second},
			Retries: &trafficv1alpha1.RetryPolicy{
				Attempts: 3,
				RetryOn:  []string{"5xx", "connect-failure", "5xx"}, // duplicate on purpose
			},
		},
	}
	res := discovery.Resolution{
		ServiceFound: true, PortFound: true, EndpointPort: 80,
		Versions: []discovery.EndpointSet{{Version: "v1", Addresses: []string{"1.1.1.1"}}},
	}
	m := Build("h", []trafficv1alpha1.RouteRule{rule}, map[string]discovery.Resolution{"/x": res})
	r := m.Rules[0].Resilience
	if r == nil {
		t.Fatal("Resilience = nil, want populated")
	}
	if r.TimeoutMS != 2000 || r.ConnectTimeoutMS != 2000 {
		t.Fatalf("timeouts = %+v, want 2000ms/2000ms", r)
	}
	if r.Retries != 3 {
		t.Fatalf("Retries = %d, want 3", r.Retries)
	}
	if want := []string{"5xx", "conn-failure"}; !equalStrings(r.RetryOn, want) {
		t.Fatalf("RetryOn = %v, want %v (deduped, mapped to HAProxy tokens)", r.RetryOn, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBackendName_StableAndDistinct(t *testing.T) {
	a := backendName("api.example.com", "/payment")
	b := backendName("api.example.com", "/payment")
	c := backendName("api.example.com", "/orders")
	if a != b {
		t.Fatal("backendName not stable")
	}
	if a == c {
		t.Fatal("backendName collided for different paths")
	}
}
