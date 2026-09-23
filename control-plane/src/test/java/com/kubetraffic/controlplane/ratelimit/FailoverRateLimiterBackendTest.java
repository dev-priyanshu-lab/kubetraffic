/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.ratelimit;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import org.junit.jupiter.api.Test;

class FailoverRateLimiterBackendTest {

  private static class ThrowingBackend implements RateLimiterBackend {
    boolean broken;

    @Override
    public long incrementAndGet(String key, Duration window) {
      if (broken) throw new RuntimeException("redis is down");
      return 999; // distinguishable from the local fallback's own counting
    }
  }

  @Test
  void fallsBackToLocalCountingWhenPrimaryThrows() {
    var primary = new ThrowingBackend();
    var backend = new FailoverRateLimiterBackend(primary);

    assertThat(backend.incrementAndGet("k", Duration.ofSeconds(60))).isEqualTo(999);
    assertThat(backend.isUsingFallback()).isFalse();

    primary.broken = true;
    assertThat(backend.incrementAndGet("k", Duration.ofSeconds(60))).isEqualTo(1); // local, fresh bucket
    assertThat(backend.incrementAndGet("k", Duration.ofSeconds(60))).isEqualTo(2);
    assertThat(backend.isUsingFallback()).isTrue();

    primary.broken = false;
    assertThat(backend.incrementAndGet("k", Duration.ofSeconds(60))).isEqualTo(999); // back on primary
    assertThat(backend.isUsingFallback()).isFalse();
  }
}
