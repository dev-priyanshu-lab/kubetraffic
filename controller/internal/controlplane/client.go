/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

// Package controlplane is the controller's gRPC client to the Java control
// plane: it registers TrafficRoutes, fetches their authoritative effective
// weights, and watches for out-of-band decisions.
package controlplane

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/apimachinery/pkg/types"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
	kubetrafficv1 "github.com/kubetraffic/controller/internal/grpcapi/kubetrafficv1"
)

// Client is what the reconciler needs from the control plane.
type Client interface {
	// RegisterRoute idempotently upserts tr's spec and returns the control
	// plane's authoritative effective configuration.
	RegisterRoute(ctx context.Context, tr *trafficv1alpha1.TrafficRoute) (*kubetrafficv1.RouteConfig, error)
	// DeleteRoute removes a route the controller no longer manages. It must be
	// safe to call more than once (finalizer retries).
	DeleteRoute(ctx context.Context, ref types.NamespacedName) error
	// GetRouteConfig polls the effective configuration.
	GetRouteConfig(ctx context.Context, ref types.NamespacedName) (*kubetrafficv1.RouteConfig, error)
}

// DecisionWatcher is the subset of Client used by the decision-stream Runnable.
type DecisionWatcher interface {
	// WatchDecisions blocks, invoking onDecision for every decision received,
	// until ctx is cancelled or the stream errors.
	WatchDecisions(ctx context.Context, onDecision func(*kubetrafficv1.Decision)) error
}

// GRPCClient is the real Client, backed by a gRPC connection.
type GRPCClient struct {
	conn      *grpc.ClientConn
	route     kubetrafficv1.RouteServiceClient
	decisions kubetrafficv1.DecisionServiceClient
	timeout   time.Duration
}

// Dial opens a (lazy, auto-reconnecting) gRPC connection to target, e.g.
// "kubetraffic-control-plane.kubetraffic-system.svc:9090".
func Dial(target string, timeout time.Duration) (*GRPCClient, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial control plane %q: %w", target, err)
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &GRPCClient{
		conn:      conn,
		route:     kubetrafficv1.NewRouteServiceClient(conn),
		decisions: kubetrafficv1.NewDecisionServiceClient(conn),
		timeout:   timeout,
	}, nil
}

// Close releases the underlying connection.
func (c *GRPCClient) Close() error { return c.conn.Close() }

// RegisterRoute implements Client.
func (c *GRPCClient) RegisterRoute(ctx context.Context, tr *trafficv1alpha1.TrafficRoute) (*kubetrafficv1.RouteConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	return c.route.RegisterRoute(ctx, BuildRouteSpec(tr))
}

// DeleteRoute implements Client.
func (c *GRPCClient) DeleteRoute(ctx context.Context, ref types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	_, err := c.route.DeleteRoute(ctx, &kubetrafficv1.RouteRef{Namespace: ref.Namespace, Name: ref.Name})
	return err
}

// GetRouteConfig implements Client.
func (c *GRPCClient) GetRouteConfig(ctx context.Context, ref types.NamespacedName) (*kubetrafficv1.RouteConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	return c.route.GetRouteConfig(ctx, &kubetrafficv1.RouteRef{Namespace: ref.Namespace, Name: ref.Name})
}

// WatchDecisions implements DecisionWatcher. It blocks for the life of the
// stream; callers are expected to loop (with backoff) across calls.
func (c *GRPCClient) WatchDecisions(ctx context.Context, onDecision func(*kubetrafficv1.Decision)) error {
	stream, err := c.decisions.StreamDecisions(ctx, &kubetrafficv1.WatchDecisionsRequest{})
	if err != nil {
		return fmt.Errorf("open decision stream: %w", err)
	}
	for {
		decision, err := stream.Recv()
		if err != nil {
			return err
		}
		onDecision(decision)
	}
}

// BuildRouteSpec converts a TrafficRoute's spec into the wire RouteSpec.
func BuildRouteSpec(tr *trafficv1alpha1.TrafficRoute) *kubetrafficv1.RouteSpec {
	spec := &kubetrafficv1.RouteSpec{
		Ref:        &kubetrafficv1.RouteRef{Namespace: tr.Namespace, Name: tr.Name},
		Host:       tr.Spec.Host,
		Generation: tr.Generation,
	}
	for _, rule := range tr.Spec.Routes {
		path := rule.Path
		if path == "" {
			path = "/"
		}
		wireRule := &kubetrafficv1.RulePolicy{
			Path:           path,
			BackendService: rule.Backend.Service,
			BackendPort:    rule.Backend.Port,
			Strategy:       string(rule.Strategy.Type),
		}
		for _, v := range rule.Versions {
			wireRule.Versions = append(wireRule.Versions, &kubetrafficv1.RouteVersionSpec{
				Name: v.Name, Weight: v.Weight,
			})
		}
		spec.Rules = append(spec.Rules, wireRule)
	}
	return spec
}

// PassThroughConfig builds a RouteConfig whose weights are copied verbatim
// from spec. It is what a decision-engine-less control plane returns, and is
// reused by Fake so tests see the same behaviour as the real server (Phase 8).
func PassThroughConfig(spec *kubetrafficv1.RouteSpec, generation int64) *kubetrafficv1.RouteConfig {
	cfg := &kubetrafficv1.RouteConfig{Ref: spec.GetRef(), Generation: generation}
	for _, rule := range spec.GetRules() {
		rw := &kubetrafficv1.RuleWeights{Path: rule.GetPath()}
		for _, v := range rule.GetVersions() {
			rw.Weights = append(rw.Weights, &kubetrafficv1.VersionWeight{Version: v.GetName(), Weight: v.GetWeight()})
		}
		cfg.Rules = append(cfg.Rules, rw)
	}
	return cfg
}
