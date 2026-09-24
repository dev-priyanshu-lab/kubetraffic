/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

// Package model is the routing model: the proxy-agnostic intermediate
// representation that sits between Kubernetes objects and proxy configuration.
//
//	Kubernetes objects  ->  domain (spec + discovery.Resolution)
//	                    ->  model.RoutingModel   (this package)
//	                    ->  proxy config         (internal/proxy)
package model

import (
	"fmt"
	"hash/fnv"
	"sort"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
	"github.com/kubetraffic/controller/internal/discovery"
)

// RoutingModel is everything one proxy instance needs to route a single host.
type RoutingModel struct {
	Host  string
	Rules []Rule
}

// Rule maps a host+path to a set of weighted upstream servers.
type Rule struct {
	Path        string
	BackendName string
	Servers     []Server
	Resilience  *Resilience
}

// Resilience is spec.resilience translated into HAProxy-native terms. HAProxy
// has no per-try-vs-overall timeout distinction, so ConnectTimeoutMS/TimeoutMS
// map onto its "timeout connect"/"timeout server" directives directly, and
// RetryOn is already normalized to HAProxy's own retry-on tokens.
type Resilience struct {
	TimeoutMS        int
	ConnectTimeoutMS int
	Retries          int
	RetryOn          []string
}

// Server is one concrete upstream endpoint.
type Server struct {
	Name    string
	Address string
	Port    int32
	Weight  int
	Version string
}

// Build turns a TrafficRoute's routes plus their resolved endpoints into a
// RoutingModel. resolutions is keyed by the route's effective path.
func Build(host string, routes []trafficv1alpha1.RouteRule, resolutions map[string]discovery.Resolution) RoutingModel {
	m := RoutingModel{Host: host}

	for i := range routes {
		rule := routes[i]
		path := rule.Path
		if path == "" {
			path = "/"
		}
		res := resolutions[path]

		port := res.EndpointPort
		if port == 0 {
			port = rule.Backend.Port
		}

		r := Rule{Path: path, BackendName: backendName(host, path), Resilience: buildResilience(rule.Resilience)}

		for _, es := range res.Versions {
			weights := apportion(specWeight(rule, es.Version), len(es.Addresses))
			for j, addr := range es.Addresses {
				r.Servers = append(r.Servers, Server{
					Name:    fmt.Sprintf("%s-%d", serverPrefix(es.Version), j),
					Address: addr,
					Port:    port,
					Weight:  weights[j],
					Version: es.Version,
				})
			}
		}
		sort.Slice(r.Servers, func(a, b int) bool { return r.Servers[a].Name < r.Servers[b].Name })
		m.Rules = append(m.Rules, r)
	}
	return m
}

// apportion splits a version's weight across n endpoints using largest-remainder
// rounding so the per-server weights sum exactly to versionWeight.
//
//   - versionWeight == 0 -> every server gets weight 0 (up, health-checked, but
//     receives no traffic: the correct state for a canary parked at 0%).
//   - versionWeight  > 0 -> every server gets at least 1 so no ready pod is
//     silently parked; at extreme ratios (weight < n) this slightly inflates
//     the version's aggregate share, which is acceptable.
func apportion(versionWeight int32, n int) []int {
	out := make([]int, n)
	if n == 0 || versionWeight <= 0 {
		return out
	}
	total := int(versionWeight)
	base := total / n
	rem := total % n
	for i := range out {
		w := base
		if i < rem {
			w++
		}
		if w < 1 {
			w = 1
		}
		if w > 256 {
			w = 256
		}
		out[i] = w
	}
	return out
}

// retryOnTokens maps the CRD's retryOn enum to HAProxy's own retry-on tokens.
// HAProxy has no "reset" or "retriable-status-codes" token; the closest native
// equivalents are used (see README Phase 11 notes).
var retryOnTokens = map[string]string{
	"5xx":                    "5xx",
	"gateway-error":          "502 503 504",
	"connect-failure":        "conn-failure",
	"reset":                  "empty-response conn-failure",
	"retriable-status-codes": "all-retryable-errors",
}

// buildResilience translates the CRD's optional resilience block into
// HAProxy-native terms. Returns nil when the rule declares none, so rules
// without spec.resilience render byte-identically to before Phase 11.
func buildResilience(r *trafficv1alpha1.ResiliencePolicy) *Resilience {
	if r == nil {
		return nil
	}
	out := &Resilience{
		TimeoutMS:        int(r.Timeout.Duration.Milliseconds()),
		ConnectTimeoutMS: int(r.Timeout.Duration.Milliseconds()),
	}
	if r.Retries != nil {
		out.Retries = int(r.Retries.Attempts)
		if perTry := r.Retries.PerTryTimeout.Duration.Milliseconds(); perTry > 0 {
			out.TimeoutMS = int(perTry)
		}
		seen := map[string]bool{}
		for _, ro := range r.Retries.RetryOn {
			token := retryOnTokens[ro]
			if token == "" || seen[token] {
				continue
			}
			seen[token] = true
			out.RetryOn = append(out.RetryOn, token)
		}
	}
	return out
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

func serverPrefix(version string) string {
	if version == "" {
		return "ep"
	}
	return version
}

func backendName(host, path string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(host))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(path))
	return fmt.Sprintf("kt_be_%08x", h.Sum32())
}
