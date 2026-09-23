/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.circuitbreaker;

import java.time.Duration;
import java.time.Instant;
import org.springframework.stereotype.Service;

/**
 * Shared CLOSED/OPEN/HALF_OPEN circuit state. Not yet wired to live health signals (the rule
 * engine that will call {@link #recordFailure} / {@link #recordSuccess} from real traffic
 * decisions lands in Phase 13, reading {@code spec.resilience.circuitBreaker} from each route);
 * Phase 9's job is the shared, fail-open state machine and its storage.
 */
@Service
public class CircuitBreakerService {

  private final CircuitBreakerStore store;

  public CircuitBreakerService(CircuitBreakerStore store) {
    this.store = store;
  }

  /** Reads the current state, applying the OPEN -&gt; HALF_OPEN transition if it's due. */
  public CircuitBreakerRecord get(String key, Duration recoveryTimeout) {
    CircuitBreakerRecord current = store.get(key);
    CircuitBreakerRecord updated =
        CircuitBreakerLogic.withRecoveryCheck(current, recoveryTimeout, Instant.now());
    if (!updated.equals(current)) {
      store.put(key, updated);
    }
    return updated;
  }

  public CircuitBreakerRecord recordFailure(String key, int failureThreshold, Duration recoveryTimeout) {
    CircuitBreakerRecord current = get(key, recoveryTimeout); // apply any pending recovery first
    CircuitBreakerRecord updated = CircuitBreakerLogic.onFailure(current, failureThreshold);
    store.put(key, updated);
    return updated;
  }

  public CircuitBreakerRecord recordSuccess(String key) {
    CircuitBreakerRecord updated = CircuitBreakerLogic.onSuccess(store.get(key));
    store.put(key, updated);
    return updated;
  }
}
