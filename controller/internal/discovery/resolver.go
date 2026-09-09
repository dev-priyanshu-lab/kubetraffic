/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

// Package discovery resolves a TrafficRoute backend into concrete, ready
// endpoints grouped by version.
//
// Resolution walks: Service (validate the referenced port) -> its EndpointSlices
// -> each endpoint's target Pod (read from the informer cache) -> the pod's
// version label -> a per-version bucket.
package discovery

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
)

// EndpointSet is the resolved endpoint count for one version.
type EndpointSet struct {
	// Version is the declared version name, or "" for a route with no versions.
	Version   string
	Ready     int32
	NotReady  int32
	Addresses []string
}

// Resolution is the full result of resolving one RouteRule.
type Resolution struct {
	ServiceFound bool
	PortFound    bool
	PortName     string
	TargetPort   string
	// EndpointPort is the numeric port the endpoints listen on (from the
	// EndpointSlice), i.e. the pod container port to dial. Zero if unknown.
	EndpointPort int32
	// Versions is one EndpointSet per declared version, in spec order (or a
	// single "" entry when the rule declares no versions).
	Versions []EndpointSet
	// Unmatched counts ready endpoints that could not be attributed to any
	// declared version (missing pod, or version label matching nothing).
	Unmatched int32
}

// Resolver resolves backends using a controller-runtime client (cache-backed).
type Resolver struct {
	Client client.Client
}

type versionBucket struct {
	set      *EndpointSet
	selector map[string]string
}

// Resolve resolves rule's backend in namespace. A missing Service or port is
// reported via the Resolution flags (not an error); only genuine API failures
// return an error.
func (r *Resolver) Resolve(ctx context.Context, namespace string, rule trafficv1alpha1.RouteRule, versionLabel string) (Resolution, error) {
	var res Resolution

	var svc corev1.Service
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: rule.Backend.Service}, &svc); err != nil {
		if apierrors.IsNotFound(err) {
			return res, nil
		}
		return res, fmt.Errorf("get service %s/%s: %w", namespace, rule.Backend.Service, err)
	}
	res.ServiceFound = true

	for _, p := range svc.Spec.Ports {
		if p.Port == rule.Backend.Port {
			res.PortFound = true
			res.PortName = p.Name
			res.TargetPort = p.TargetPort.String()
			break
		}
	}

	var slices discoveryv1.EndpointSliceList
	if err := r.Client.List(ctx, &slices,
		client.InNamespace(namespace),
		client.MatchingLabels{discoveryv1.LabelServiceName: rule.Backend.Service},
	); err != nil {
		return res, fmt.Errorf("list endpointslices for service %s: %w", rule.Backend.Service, err)
	}

	buckets := newBuckets(rule, versionLabel)

	for si := range slices.Items {
		if p := endpointPort(&slices.Items[si], res.PortName); p != 0 && res.EndpointPort == 0 {
			res.EndpointPort = p
		}
		for ei := range slices.Items[si].Endpoints {
			ep := &slices.Items[si].Endpoints[ei]
			ready := ep.Conditions.Ready == nil || *ep.Conditions.Ready

			var target *versionBucket
			if len(rule.Versions) == 0 {
				target = buckets[0]
			} else {
				target = r.attribute(ctx, namespace, ep, buckets)
			}
			if target == nil {
				if ready {
					res.Unmatched++
				}
				continue
			}

			if ready {
				target.set.Ready++
				if len(ep.Addresses) > 0 {
					target.set.Addresses = append(target.set.Addresses, ep.Addresses[0])
				}
			} else {
				target.set.NotReady++
			}
		}
	}

	for _, b := range buckets {
		sort.Strings(b.set.Addresses)
		res.Versions = append(res.Versions, *b.set)
	}
	return res, nil
}

func newBuckets(rule trafficv1alpha1.RouteRule, versionLabel string) []*versionBucket {
	if len(rule.Versions) == 0 {
		return []*versionBucket{{set: &EndpointSet{Version: ""}}}
	}
	buckets := make([]*versionBucket, 0, len(rule.Versions))
	for _, v := range rule.Versions {
		sel := v.Labels
		if len(sel) == 0 {
			sel = map[string]string{versionLabel: v.Name}
		}
		buckets = append(buckets, &versionBucket{set: &EndpointSet{Version: v.Name}, selector: sel})
	}
	return buckets
}

// attribute finds the version bucket for an endpoint by reading its target
// pod's labels. Overlapping selectors resolve to the first match in spec order.
func (r *Resolver) attribute(ctx context.Context, namespace string, ep *discoveryv1.Endpoint, buckets []*versionBucket) *versionBucket {
	if ep.TargetRef == nil || ep.TargetRef.Kind != "Pod" {
		return nil
	}
	var pod corev1.Pod
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ep.TargetRef.Name}, &pod); err != nil {
		return nil
	}
	for _, b := range buckets {
		if selectorMatches(pod.Labels, b.selector) {
			return b
		}
	}
	return nil
}

// endpointPort picks the numeric port from a slice. It prefers the port whose
// name matches the Service port; otherwise the sole port, if there is one.
func endpointPort(slice *discoveryv1.EndpointSlice, portName string) int32 {
	if len(slice.Ports) == 1 && slice.Ports[0].Port != nil {
		return *slice.Ports[0].Port
	}
	for i := range slice.Ports {
		p := &slice.Ports[i]
		if p.Port == nil {
			continue
		}
		if (p.Name == nil && portName == "") || (p.Name != nil && *p.Name == portName) {
			return *p.Port
		}
	}
	return 0
}

func selectorMatches(have, want map[string]string) bool {
	if len(want) == 0 {
		return false
	}
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}
