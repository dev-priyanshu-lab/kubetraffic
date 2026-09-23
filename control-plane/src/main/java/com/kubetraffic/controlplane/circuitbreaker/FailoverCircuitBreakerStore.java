/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.circuitbreaker;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * The store the app actually uses: a primary (Redis, shared across every control-plane replica)
 * with a local, per-replica fallback if the primary errors — fail-open, matching the
 * architecture's documented Redis-outage behaviour. The primary is injected rather than
 * constructed here so this class is testable without a real Redis connection.
 */
public class FailoverCircuitBreakerStore implements CircuitBreakerStore {

  private static final Logger log = LoggerFactory.getLogger(FailoverCircuitBreakerStore.class);

  private final CircuitBreakerStore primary;
  private final CircuitBreakerStore localStore = new LocalCircuitBreakerStore();
  private volatile boolean usingFallback;

  public FailoverCircuitBreakerStore(CircuitBreakerStore primary) {
    this.primary = primary;
  }

  @Override
  public CircuitBreakerRecord get(String key) {
    try {
      CircuitBreakerRecord record = primary.get(key);
      resumedIfWasFallback();
      return record;
    } catch (Exception e) {
      warnOnce(e);
      return localStore.get(key);
    }
  }

  @Override
  public void put(String key, CircuitBreakerRecord record) {
    try {
      primary.put(key, record);
      resumedIfWasFallback();
    } catch (Exception e) {
      warnOnce(e);
      localStore.put(key, record);
    }
  }

  private void warnOnce(Exception e) {
    if (!usingFallback) {
      log.warn(
          "Redis unavailable for circuit-breaker state ({}); falling back to a local, per-replica store",
          e.getMessage());
    }
    usingFallback = true;
  }

  private void resumedIfWasFallback() {
    if (usingFallback) {
      log.info("Redis reachable again; circuit-breaker state resumed on the shared store");
    }
    usingFallback = false;
  }

  /** Exposed for observability/tests: true while serving from the local fallback. */
  public boolean isUsingFallback() {
    return usingFallback;
  }
}
