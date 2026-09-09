/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

package proxy

import (
	"strings"
	"testing"

	"github.com/kubetraffic/controller/internal/model"
)

func sampleModel() model.RoutingModel {
	return model.RoutingModel{
		Host: "api.example.com",
		Rules: []model.Rule{{
			Path:        "/payment",
			BackendName: "kt_be_deadbeef",
			Servers: []model.Server{
				{Name: "v1-0", Address: "10.0.0.1", Port: 8080, Weight: 45, Version: "v1"},
				{Name: "v1-1", Address: "10.0.0.2", Port: 8080, Weight: 45, Version: "v1"},
				{Name: "v2-0", Address: "10.0.0.3", Port: 8080, Weight: 5, Version: "v2"},
			},
		}},
	}
}

func TestRender_ContainsBaseAndRoutes(t *testing.T) {
	cfg, err := NewRenderer("s3cr3t").Render(sampleModel())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"program api",
		"user admin insecure-password s3cr3t",
		"frontend kubetraffic",
		"use_backend kt_be_deadbeef if { var(txn.host) -m str api.example.com } { path -m beg /payment }",
		"backend kt_be_deadbeef",
		"server v1-0 10.0.0.1:8080 weight 45 check",
		"server v2-0 10.0.0.3:8080 weight 5 check",
		"default_backend kubetraffic_no_route",
	} {
		if !strings.Contains(cfg.Raw, want) {
			t.Fatalf("rendered config missing %q\n---\n%s", want, cfg.Raw)
		}
	}
}

func TestRender_Deterministic(t *testing.T) {
	r := NewRenderer("x")
	a, _ := r.Render(sampleModel())
	b, _ := r.Render(sampleModel())
	if a.Raw != b.Raw || a.Sum() != b.Sum() {
		t.Fatal("render is not deterministic")
	}
}

func TestRender_NoHost(t *testing.T) {
	if _, err := NewRenderer("x").Render(model.RoutingModel{}); err == nil {
		t.Fatal("expected an error for a model with no host")
	}
}

func TestRender_LongestPathFirst(t *testing.T) {
	m := model.RoutingModel{
		Host: "api.example.com",
		Rules: []model.Rule{
			{Path: "/", BackendName: "kt_be_root"},
			{Path: "/payment/refund", BackendName: "kt_be_refund"},
			{Path: "/payment", BackendName: "kt_be_pay"},
		},
	}
	cfg, err := NewRenderer("x").Render(m)
	if err != nil {
		t.Fatal(err)
	}
	iRefund := strings.Index(cfg.Raw, "kt_be_refund if")
	iPay := strings.Index(cfg.Raw, "kt_be_pay if")
	iRoot := strings.Index(cfg.Raw, "kt_be_root if")
	if !(iRefund < iPay && iPay < iRoot) {
		t.Fatalf("route order wrong: refund=%d pay=%d root=%d", iRefund, iPay, iRoot)
	}
}
