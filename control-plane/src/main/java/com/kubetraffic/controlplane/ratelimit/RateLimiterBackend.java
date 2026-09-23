/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.ratelimit;

import java.time.Duration;

/** A fixed-window counter. Implementations decide where the count lives. */
public interface RateLimiterBackend {

  /**
   * Atomically increments key's counter for the current window and returns the new count. The
   * window is keyed by wall-clock time sliced into {@code window}-sized buckets, so the count
   * resets automatically once a bucket expires.
   */
  long incrementAndGet(String key, Duration window);
}
