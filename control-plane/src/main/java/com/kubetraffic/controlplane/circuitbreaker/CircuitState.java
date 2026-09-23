/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.circuitbreaker;

/** CLOSED -&gt; OPEN -&gt; HALF_OPEN -&gt; CLOSED, per the architecture's resilience design. */
public enum CircuitState {
  CLOSED,
  OPEN,
  HALF_OPEN
}
