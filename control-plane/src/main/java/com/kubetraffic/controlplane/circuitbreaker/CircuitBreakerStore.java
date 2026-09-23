/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.circuitbreaker;

/** Where a {@link CircuitBreakerRecord} lives. */
public interface CircuitBreakerStore {

  /** Returns the stored record, or {@link CircuitBreakerRecord#closed()} if key is unknown. */
  CircuitBreakerRecord get(String key);

  void put(String key, CircuitBreakerRecord record);
}
