/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

package metrics

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestObserveReconcile(t *testing.T) {
	totalBefore := testutil.ToFloat64(reconcileTotal)
	errsBefore := testutil.ToFloat64(reconcileErrorsTotal)

	ObserveReconcile(2*time.Millisecond, nil)
	ObserveReconcile(1*time.Millisecond, errors.New("boom"))

	if got := testutil.ToFloat64(reconcileTotal) - totalBefore; got != 2 {
		t.Fatalf("reconcile_total delta = %v, want 2", got)
	}
	if got := testutil.ToFloat64(reconcileErrorsTotal) - errsBefore; got != 1 {
		t.Fatalf("reconcile_errors_total delta = %v, want 1", got)
	}
}
