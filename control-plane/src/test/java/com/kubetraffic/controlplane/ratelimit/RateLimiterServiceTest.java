/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.ratelimit;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import org.junit.jupiter.api.Test;

class RateLimiterServiceTest {

  private final RateLimiterService service = new RateLimiterService(new LocalRateLimiterBackend());

  @Test
  void allowsUpToTheLimitThenDenies() {
    RateLimitResult last = null;
    for (int i = 1; i <= 3; i++) {
      last = service.tryAcquire("k", 3, Duration.ofSeconds(60));
      assertThat(last.allowed()).as("request %d of 3", i).isTrue();
      assertThat(last.count()).isEqualTo(i);
    }
    assertThat(last.remaining()).isZero();

    var denied = service.tryAcquire("k", 3, Duration.ofSeconds(60));
    assertThat(denied.allowed()).isFalse();
    assertThat(denied.count()).isEqualTo(4);
    assertThat(denied.remaining()).isZero();
  }

  @Test
  void differentKeysHaveIndependentBuckets() {
    for (int i = 0; i < 5; i++) {
      service.tryAcquire("a", 5, Duration.ofSeconds(60));
    }
    var b = service.tryAcquire("b", 5, Duration.ofSeconds(60));
    assertThat(b.allowed()).isTrue();
    assertThat(b.count()).isEqualTo(1);
  }
}
