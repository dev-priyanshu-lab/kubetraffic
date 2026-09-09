/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

package v1alpha1

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
)

func baseRoute() *trafficv1alpha1.TrafficRoute {
	return &trafficv1alpha1.TrafficRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "payment-route", Namespace: "demo"},
		Spec: trafficv1alpha1.TrafficRouteSpec{
			Host: "api.example.com",
			Routes: []trafficv1alpha1.RouteRule{{
				Path:    "/payment",
				Backend: trafficv1alpha1.BackendRef{Service: "payment", Port: 8080},
				Versions: []trafficv1alpha1.BackendVersion{
					{Name: "v1", Weight: 90},
					{Name: "v2", Weight: 10},
				},
				Strategy: trafficv1alpha1.RoutingStrategy{Type: trafficv1alpha1.StrategyWeighted},
			}},
		},
	}
}

func TestValidateTrafficRoute_Valid(t *testing.T) {
	if errs := ValidateTrafficRoute(baseRoute()); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
}

func TestValidateTrafficRoute_Table(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(tr *trafficv1alpha1.TrafficRoute)
		wantSub string // substring expected in the aggregated error
	}{
		{
			name:    "missing host",
			mutate:  func(tr *trafficv1alpha1.TrafficRoute) { tr.Spec.Host = "  " },
			wantSub: "host is required",
		},
		{
			name:    "no routes",
			mutate:  func(tr *trafficv1alpha1.TrafficRoute) { tr.Spec.Routes = nil },
			wantSub: "at least one route",
		},
		{
			name:    "weights do not sum to 100",
			mutate:  func(tr *trafficv1alpha1.TrafficRoute) { tr.Spec.Routes[0].Versions[1].Weight = 20 },
			wantSub: "must sum to 100",
		},
		{
			name:    "weight out of range",
			mutate:  func(tr *trafficv1alpha1.TrafficRoute) { tr.Spec.Routes[0].Versions[0].Weight = 150 },
			wantSub: "weight must be between 0 and 100",
		},
		{
			name: "duplicate version name",
			mutate: func(tr *trafficv1alpha1.TrafficRoute) {
				tr.Spec.Routes[0].Versions[1].Name = "v1"
			},
			wantSub: "Duplicate value",
		},
		{
			name:    "empty backend service",
			mutate:  func(tr *trafficv1alpha1.TrafficRoute) { tr.Spec.Routes[0].Backend.Service = "" },
			wantSub: "backend service is required",
		},
		{
			name:    "bad port",
			mutate:  func(tr *trafficv1alpha1.TrafficRoute) { tr.Spec.Routes[0].Backend.Port = 99999 },
			wantSub: "port must be between 1 and 65535",
		},
		{
			name: "canary with one version",
			mutate: func(tr *trafficv1alpha1.TrafficRoute) {
				tr.Spec.Routes[0].Strategy.Type = trafficv1alpha1.StrategyCanary
				tr.Spec.Routes[0].Versions = []trafficv1alpha1.BackendVersion{{Name: "v1", Weight: 100}}
			},
			wantSub: "CANARY strategy requires at least two versions",
		},
		{
			name: "canary steps not ascending",
			mutate: func(tr *trafficv1alpha1.TrafficRoute) {
				tr.Spec.Routes[0].Strategy.Type = trafficv1alpha1.StrategyCanary
				tr.Spec.Routes[0].Strategy.Canary = &trafficv1alpha1.CanaryStrategy{Steps: []int32{10, 10, 50}}
			},
			wantSub: "strictly ascending",
		},
		{
			name: "blue-green with three versions",
			mutate: func(tr *trafficv1alpha1.TrafficRoute) {
				tr.Spec.Routes[0].Strategy.Type = trafficv1alpha1.StrategyBlueGreen
				tr.Spec.Routes[0].Versions = append(tr.Spec.Routes[0].Versions, trafficv1alpha1.BackendVersion{Name: "v3", Weight: 0})
				tr.Spec.Routes[0].Versions[0].Weight = 90
			},
			wantSub: "BLUE_GREEN strategy requires exactly two versions",
		},
		{
			name: "retry attempts too high",
			mutate: func(tr *trafficv1alpha1.TrafficRoute) {
				tr.Spec.Routes[0].Resilience = &trafficv1alpha1.ResiliencePolicy{
					Retries: &trafficv1alpha1.RetryPolicy{Attempts: 25},
				}
			},
			wantSub: "attempts must be between 0 and 10",
		},
		{
			name: "circuit breaker enabled without threshold",
			mutate: func(tr *trafficv1alpha1.TrafficRoute) {
				tr.Spec.Routes[0].Resilience = &trafficv1alpha1.ResiliencePolicy{
					CircuitBreaker: &trafficv1alpha1.CircuitBreakerPolicy{Enabled: true},
				}
			},
			wantSub: "failureThreshold must be >= 1",
		},
		{
			name: "rate limit below 1",
			mutate: func(tr *trafficv1alpha1.TrafficRoute) {
				tr.Spec.Routes[0].Security = &trafficv1alpha1.SecurityPolicy{
					RateLimit: &trafficv1alpha1.RateLimitPolicy{RequestsPerSecond: 0},
				}
			},
			wantSub: "requestsPerSecond must be >= 1",
		},
		{
			name: "jwt without issuer",
			mutate: func(tr *trafficv1alpha1.TrafficRoute) {
				tr.Spec.Routes[0].Security = &trafficv1alpha1.SecurityPolicy{
					JWT: &trafficv1alpha1.JWTPolicy{},
				}
			},
			wantSub: "issuer is required",
		},
		{
			name: "duplicate path",
			mutate: func(tr *trafficv1alpha1.TrafficRoute) {
				tr.Spec.Routes = append(tr.Spec.Routes, tr.Spec.Routes[0])
			},
			wantSub: "Duplicate value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := baseRoute()
			tc.mutate(tr)
			errs := ValidateTrafficRoute(tr)
			if len(errs) == 0 {
				t.Fatalf("expected a validation error containing %q, got none", tc.wantSub)
			}
			if !strings.Contains(errs.ToAggregate().Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", errs.ToAggregate().Error(), tc.wantSub)
			}
		})
	}
}

func TestValidator_ValidateCreate(t *testing.T) {
	v := &TrafficRouteValidator{}

	if _, err := v.ValidateCreate(context.Background(), baseRoute()); err != nil {
		t.Fatalf("valid object rejected: %v", err)
	}

	bad := baseRoute()
	bad.Spec.Routes[0].Versions[1].Weight = 50 // sum = 140
	if _, err := v.ValidateCreate(context.Background(), bad); err == nil {
		t.Fatal("invalid object accepted")
	}
}

func TestValidator_WarnsOnCanaryWithoutLatencyThreshold(t *testing.T) {
	tr := baseRoute()
	tr.Spec.Routes[0].Strategy.Type = trafficv1alpha1.StrategyCanary
	tr.Spec.Routes[0].Versions[0].Weight = 100
	tr.Spec.Routes[0].Versions[1].Weight = 0

	w, err := (&TrafficRouteValidator{}).ValidateCreate(context.Background(), tr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(w) == 0 {
		t.Fatal("expected a warning about missing latencyThreshold")
	}
}
