/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.circuitbreaker;

import java.util.concurrent.ConcurrentHashMap;

/** Per-JVM fallback used while Redis is unreachable; state stops being shared across replicas. */
public class LocalCircuitBreakerStore implements CircuitBreakerStore {

  private final ConcurrentHashMap<String, CircuitBreakerRecord> records = new ConcurrentHashMap<>();

  @Override
  public CircuitBreakerRecord get(String key) {
    return records.getOrDefault(key, CircuitBreakerRecord.closed());
  }

  @Override
  public void put(String key, CircuitBreakerRecord record) {
    records.put(key, record);
  }
}
