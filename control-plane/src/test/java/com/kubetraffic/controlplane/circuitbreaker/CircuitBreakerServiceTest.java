/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.circuitbreaker;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import org.junit.jupiter.api.Test;

class CircuitBreakerServiceTest {

  private final CircuitBreakerService service = new CircuitBreakerService(new LocalCircuitBreakerStore());

  @Test
  void opensAfterConsecutiveFailures() {
    for (int i = 0; i < 4; i++) {
      var r = service.recordFailure("demo:payment-route:v2", 5, Duration.ofSeconds(30));
      assertThat(r.state()).isEqualTo(CircuitState.CLOSED);
    }
    var opened = service.recordFailure("demo:payment-route:v2", 5, Duration.ofSeconds(30));
    assertThat(opened.state()).isEqualTo(CircuitState.OPEN);
  }

  @Test
  void successResetsAnOpenCircuit() {
    for (int i = 0; i < 5; i++) {
      service.recordFailure("k", 5, Duration.ofSeconds(30));
    }
    assertThat(service.get("k", Duration.ofSeconds(30)).state()).isEqualTo(CircuitState.OPEN);

    var reset = service.recordSuccess("k");
    assertThat(reset.state()).isEqualTo(CircuitState.CLOSED);
    assertThat(reset.consecutiveFailures()).isZero();
  }

  @Test
  void unknownKeyDefaultsToClosed() {
    assertThat(service.get("never-seen", Duration.ofSeconds(30)).state()).isEqualTo(CircuitState.CLOSED);
  }
}
