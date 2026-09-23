/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

package controlplane

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/apimachinery/pkg/types"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
	kubetrafficv1 "github.com/kubetraffic/controller/internal/grpcapi/kubetrafficv1"
)

// stubRouteServer is a minimal, in-test RouteService implementation standing
// in for the Java control plane, so the Go client (proto marshalling, RPC
// plumbing, timeouts) is verified without needing the JVM.
type stubRouteServer struct {
	kubetrafficv1.UnimplementedRouteServiceServer
	lastSpec *kubetrafficv1.RouteSpec
}

func (s *stubRouteServer) RegisterRoute(_ context.Context, spec *kubetrafficv1.RouteSpec) (*kubetrafficv1.RouteConfig, error) {
	s.lastSpec = spec
	return PassThroughConfig(spec, 7), nil
}

func (s *stubRouteServer) DeleteRoute(_ context.Context, _ *kubetrafficv1.RouteRef) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *stubRouteServer) GetRouteConfig(_ context.Context, ref *kubetrafficv1.RouteRef) (*kubetrafficv1.RouteConfig, error) {
	return &kubetrafficv1.RouteConfig{Ref: ref, Generation: 3}, nil
}

type stubDecisionServer struct {
	kubetrafficv1.UnimplementedDecisionServiceServer
	decisions []*kubetrafficv1.Decision
}

func (s *stubDecisionServer) StreamDecisions(_ *kubetrafficv1.WatchDecisionsRequest, stream kubetrafficv1.DecisionService_StreamDecisionsServer) error {
	for _, d := range s.decisions {
		if err := stream.Send(d); err != nil {
			return err
		}
	}
	return nil
}

func dialBufconn(t *testing.T, route kubetrafficv1.RouteServiceServer, decisions kubetrafficv1.DecisionServiceServer) *GRPCClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	kubetrafficv1.RegisterRouteServiceServer(s, route)
	kubetrafficv1.RegisterDecisionServiceServer(s, decisions)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(s.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return &GRPCClient{
		conn:      conn,
		route:     kubetrafficv1.NewRouteServiceClient(conn),
		decisions: kubetrafficv1.NewDecisionServiceClient(conn),
		timeout:   5 * time.Second,
	}
}

func testRoute() *trafficv1alpha1.TrafficRoute {
	tr := &trafficv1alpha1.TrafficRoute{}
	tr.Namespace, tr.Name = "demo", "payment-route"
	tr.Spec.Host = "api.example.com"
	tr.Spec.Routes = []trafficv1alpha1.RouteRule{{
		Path:    "/payment",
		Backend: trafficv1alpha1.BackendRef{Service: "payment", Port: 8080},
		Versions: []trafficv1alpha1.BackendVersion{
			{Name: "v1", Weight: 90},
			{Name: "v2", Weight: 10},
		},
	}}
	return tr
}

func TestGRPCClient_RegisterRoute(t *testing.T) {
	route := &stubRouteServer{}
	c := dialBufconn(t, route, &stubDecisionServer{})

	cfg, err := c.RegisterRoute(context.Background(), testRoute())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GetGeneration() != 7 {
		t.Fatalf("generation = %d, want 7", cfg.GetGeneration())
	}
	if route.lastSpec.GetRef().GetName() != "payment-route" || route.lastSpec.GetRef().GetNamespace() != "demo" {
		t.Fatalf("server received unexpected ref: %+v", route.lastSpec.GetRef())
	}
	if got := cfg.GetRules()[0].GetWeights()[0].GetWeight(); got != 90 {
		t.Fatalf("weight = %d, want 90", got)
	}
}

func TestGRPCClient_DeleteRoute(t *testing.T) {
	c := dialBufconn(t, &stubRouteServer{}, &stubDecisionServer{})
	if err := c.DeleteRoute(context.Background(), types.NamespacedName{Namespace: "demo", Name: "x"}); err != nil {
		t.Fatal(err)
	}
}

func TestGRPCClient_GetRouteConfig(t *testing.T) {
	c := dialBufconn(t, &stubRouteServer{}, &stubDecisionServer{})
	cfg, err := c.GetRouteConfig(context.Background(), types.NamespacedName{Namespace: "demo", Name: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GetGeneration() != 3 {
		t.Fatalf("generation = %d, want 3", cfg.GetGeneration())
	}
}

func TestGRPCClient_WatchDecisions(t *testing.T) {
	decisions := &stubDecisionServer{decisions: []*kubetrafficv1.Decision{
		{Ref: &kubetrafficv1.RouteRef{Namespace: "demo", Name: "x"}, Path: "/payment", Version: "v2", OldWeight: 10, NewWeight: 20},
	}}
	c := dialBufconn(t, &stubRouteServer{}, decisions)

	var got []*kubetrafficv1.Decision
	err := c.WatchDecisions(context.Background(), func(d *kubetrafficv1.Decision) { got = append(got, d) })
	if err == nil {
		t.Fatal("expected the stream to end with an error once the server finishes sending")
	}
	if len(got) != 1 || got[0].GetNewWeight() != 20 {
		t.Fatalf("decisions received = %+v", got)
	}
}
