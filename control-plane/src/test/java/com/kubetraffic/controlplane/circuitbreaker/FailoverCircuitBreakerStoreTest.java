/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.circuitbreaker;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;

class FailoverCircuitBreakerStoreTest {

  private static class ThrowingStore implements CircuitBreakerStore {
    boolean broken;

    @Override
    public CircuitBreakerRecord get(String key) {
      if (broken) throw new RuntimeException("redis is down");
      return CircuitBreakerRecord.closed();
    }

    @Override
    public void put(String key, CircuitBreakerRecord record) {
      if (broken) throw new RuntimeException("redis is down");
    }
  }

  @Test
  void fallsBackToLocalWhenPrimaryThrows_andResumesWhenItRecovers() {
    var primary = new ThrowingStore();
    var store = new FailoverCircuitBreakerStore(primary);
    var record = new CircuitBreakerRecord(CircuitState.OPEN, 9, java.time.Instant.now());

    assertThat(store.isUsingFallback()).isFalse();

    primary.broken = true;
    store.put("k", record); // primary throws -> falls back to local, does not propagate
    assertThat(store.isUsingFallback()).isTrue();
    assertThat(store.get("k")).isEqualTo(record); // served from the local fallback

    primary.broken = false;
    // primary is healthy again; a fresh key isn't in the fallback's map, so this also proves
    // reads go back to the (now healthy) primary rather than a stale local copy.
    assertThat(store.get("other-key")).isEqualTo(CircuitBreakerRecord.closed());
    assertThat(store.isUsingFallback()).isFalse();
  }
}
