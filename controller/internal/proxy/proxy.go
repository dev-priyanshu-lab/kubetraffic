/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

// Package proxy is the data-plane abstraction. The controller builds a
// model.RoutingModel; a Proxy renders it to a concrete configuration and applies
// it. Only HAProxy is implemented today; the interface keeps Envoy addable.
package proxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/kubetraffic/controller/internal/model"
)

// Config is a rendered, proxy-specific configuration.
type Config struct {
	// Raw is the full proxy configuration text.
	Raw string
}

// Sum is a stable content hash, handy for change detection and status.
func (c Config) Sum() string {
	s := sha256.Sum256([]byte(c.Raw))
	return hex.EncodeToString(s[:])
}

// Proxy is the data-plane contract.
type Proxy interface {
	// GenerateConfig renders a RoutingModel to a proxy configuration.
	GenerateConfig(m model.RoutingModel) (Config, error)
	// ValidateConfig checks a configuration without applying it.
	ValidateConfig(ctx context.Context, c Config) error
	// ApplyConfig validates and applies a configuration, reloading if needed.
	// It is a no-op when the live configuration already matches.
	ApplyConfig(ctx context.Context, c Config) error
}
