/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.circuitbreaker;

import java.time.Instant;

/** The current state of one circuit (one route/version, or whatever key the caller chooses). */
public record CircuitBreakerRecord(CircuitState state, int consecutiveFailures, Instant openedAt) {

  public static CircuitBreakerRecord closed() {
    return new CircuitBreakerRecord(CircuitState.CLOSED, 0, null);
  }
}
