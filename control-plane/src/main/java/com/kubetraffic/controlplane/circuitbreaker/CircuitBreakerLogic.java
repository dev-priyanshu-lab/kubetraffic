/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.circuitbreaker;

import java.time.Duration;
import java.time.Instant;

/**
 * Pure CLOSED/OPEN/HALF_OPEN state transitions, independent of storage. Kept storage-agnostic
 * (and thus trivially unit-testable) so both the Redis-backed and local-fallback {@link
 * CircuitBreakerStore} implementations produce identical behaviour.
 */
public final class CircuitBreakerLogic {

  private CircuitBreakerLogic() {}

  /**
   * Records a failure. A failure while HALF_OPEN (a failed trial request) reopens immediately
   * regardless of the raw count; otherwise the circuit opens once consecutiveFailures reaches
   * failureThreshold.
   */
  public static CircuitBreakerRecord onFailure(CircuitBreakerRecord current, int failureThreshold) {
    int failures = current.consecutiveFailures() + 1;
    if (current.state() == CircuitState.HALF_OPEN || failures >= failureThreshold) {
      return new CircuitBreakerRecord(CircuitState.OPEN, failures, Instant.now());
    }
    return new CircuitBreakerRecord(CircuitState.CLOSED, failures, null);
  }

  /** A successful call (including a successful HALF_OPEN trial) fully resets the circuit. */
  public static CircuitBreakerRecord onSuccess(CircuitBreakerRecord current) {
    return CircuitBreakerRecord.closed();
  }

  /** Applies the OPEN -&gt; HALF_OPEN transition once recoveryTimeout has elapsed since openedAt. */
  public static CircuitBreakerRecord withRecoveryCheck(
      CircuitBreakerRecord current, Duration recoveryTimeout, Instant now) {
    if (current.state() == CircuitState.OPEN
        && current.openedAt() != null
        && !now.isBefore(current.openedAt().plus(recoveryTimeout))) {
      return new CircuitBreakerRecord(CircuitState.HALF_OPEN, current.consecutiveFailures(), current.openedAt());
    }
    return current;
  }
}
