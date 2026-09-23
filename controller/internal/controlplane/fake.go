/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

package controlplane

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/types"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
	kubetrafficv1 "github.com/kubetraffic/controller/internal/grpcapi/kubetrafficv1"
)

// Fake is an in-memory Client for tests. It mimics the Phase 8 server: no
// decision engine, so RegisterRoute stores and echoes the submitted weights.
type Fake struct {
	mu            sync.Mutex
	generations   map[string]int64
	configs       map[string]*kubetrafficv1.RouteConfig
	Registrations []*kubetrafficv1.RouteSpec
	Deleted       []types.NamespacedName
	RegisterErr   error
	DeleteErr     error
	GetErr        error
}

// NewFake returns an empty Fake.
func NewFake() *Fake {
	return &Fake{
		generations: map[string]int64{},
		configs:     map[string]*kubetrafficv1.RouteConfig{},
	}
}

func key(ns types.NamespacedName) string { return ns.Namespace + "/" + ns.Name }

// RegisterRoute implements Client.
func (f *Fake) RegisterRoute(_ context.Context, tr *trafficv1alpha1.TrafficRoute) (*kubetrafficv1.RouteConfig, error) {
	if f.RegisterErr != nil {
		return nil, f.RegisterErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	spec := BuildRouteSpec(tr)
	f.Registrations = append(f.Registrations, spec)

	k := key(types.NamespacedName{Namespace: tr.Namespace, Name: tr.Name})
	f.generations[k]++
	cfg := PassThroughConfig(spec, f.generations[k])
	f.configs[k] = cfg
	return cfg, nil
}

// DeleteRoute implements Client.
func (f *Fake) DeleteRoute(_ context.Context, ref types.NamespacedName) error {
	if f.DeleteErr != nil {
		return f.DeleteErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Deleted = append(f.Deleted, ref)
	delete(f.configs, key(ref))
	return nil
}

// GetRouteConfig implements Client.
func (f *Fake) GetRouteConfig(_ context.Context, ref types.NamespacedName) (*kubetrafficv1.RouteConfig, error) {
	if f.GetErr != nil {
		return nil, f.GetErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.configs[key(ref)], nil
}

// RegisterCount returns how many times RegisterRoute was called.
func (f *Fake) RegisterCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Registrations)
}
