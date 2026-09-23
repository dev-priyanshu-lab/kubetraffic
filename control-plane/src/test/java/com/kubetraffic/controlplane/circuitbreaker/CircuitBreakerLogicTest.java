/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.circuitbreaker;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import java.time.Instant;
import org.junit.jupiter.api.Test;

class CircuitBreakerLogicTest {

  @Test
  void staysClosedBelowThreshold() {
    var r = CircuitBreakerLogic.onFailure(CircuitBreakerRecord.closed(), 5);
    r = CircuitBreakerLogic.onFailure(r, 5);
    assertThat(r.state()).isEqualTo(CircuitState.CLOSED);
    assertThat(r.consecutiveFailures()).isEqualTo(2);
    assertThat(r.openedAt()).isNull();
  }

  @Test
  void opensAtThreshold() {
    var r = CircuitBreakerRecord.closed();
    for (int i = 0; i < 5; i++) {
      r = CircuitBreakerLogic.onFailure(r, 5);
    }
    assertThat(r.state()).isEqualTo(CircuitState.OPEN);
    assertThat(r.consecutiveFailures()).isEqualTo(5);
    assertThat(r.openedAt()).isNotNull();
  }

  @Test
  void successFullyResets() {
    var r = new CircuitBreakerRecord(CircuitState.OPEN, 7, Instant.now());
    r = CircuitBreakerLogic.onSuccess(r);
    assertThat(r).isEqualTo(CircuitBreakerRecord.closed());
  }

  @Test
  void recoveryTransitionsOpenToHalfOpenAfterTimeout() {
    Instant openedAt = Instant.parse("2026-01-01T00:00:00Z");
    var open = new CircuitBreakerRecord(CircuitState.OPEN, 5, openedAt);

    var tooSoon = CircuitBreakerLogic.withRecoveryCheck(open, Duration.ofSeconds(30), openedAt.plusSeconds(10));
    assertThat(tooSoon.state()).isEqualTo(CircuitState.OPEN);

    var due = CircuitBreakerLogic.withRecoveryCheck(open, Duration.ofSeconds(30), openedAt.plusSeconds(30));
    assertThat(due.state()).isEqualTo(CircuitState.HALF_OPEN);
    assertThat(due.consecutiveFailures()).isEqualTo(5); // preserved, not reset
  }

  @Test
  void recoveryCheckIsNoopWhenNotOpen() {
    var closed = CircuitBreakerRecord.closed();
    var r = CircuitBreakerLogic.withRecoveryCheck(closed, Duration.ofSeconds(1), Instant.now().plusSeconds(100));
    assertThat(r).isEqualTo(closed);
  }

  @Test
  void failureDuringHalfOpenReopensImmediately() {
    var halfOpen = new CircuitBreakerRecord(CircuitState.HALF_OPEN, 5, Instant.now());
    var r = CircuitBreakerLogic.onFailure(halfOpen, 100); // threshold irrelevant during a trial
    assertThat(r.state()).isEqualTo(CircuitState.OPEN);
    assertThat(r.consecutiveFailures()).isEqualTo(6);
  }
}
