/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

package proxy

import (
	"context"
	"sync"

	"github.com/kubetraffic/controller/internal/model"
)

// Fake is an in-memory Proxy for tests. It records what was applied and can be
// primed to fail.
type Fake struct {
	mu          sync.Mutex
	renderer    *Renderer
	Applied     []Config
	ApplyErr    error
	ValidateErr error
}

// NewFake returns a Fake proxy.
func NewFake() *Fake {
	return &Fake{renderer: NewRenderer("test-password")}
}

// GenerateConfig renders via the real renderer.
func (f *Fake) GenerateConfig(m model.RoutingModel) (Config, error) {
	return f.renderer.Render(m)
}

// ValidateConfig returns the primed ValidateErr.
func (f *Fake) ValidateConfig(_ context.Context, _ Config) error {
	return f.ValidateErr
}

// ApplyConfig records the config unless ApplyErr is primed.
func (f *Fake) ApplyConfig(_ context.Context, c Config) error {
	if f.ApplyErr != nil {
		return f.ApplyErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Applied = append(f.Applied, c)
	return nil
}

// LastApplied returns the most recently applied config, or the zero value.
func (f *Fake) LastApplied() Config {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Applied) == 0 {
		return Config{}
	}
	return f.Applied[len(f.Applied)-1]
}

// ApplyCount returns how many times ApplyConfig recorded a config.
func (f *Fake) ApplyCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Applied)
}
