/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.ratelimit;

import java.time.Duration;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * The rate limiter the app actually uses: a primary backend (Redis, shared across every
 * control-plane replica) with a local in-memory fallback if the primary errors — fail-open,
 * matching the architecture's documented Redis-outage behaviour (a degraded, per-replica limit
 * beats rejecting everything). The primary is injected rather than constructed here so this class
 * is testable without a real Redis connection.
 */
public class FailoverRateLimiterBackend implements RateLimiterBackend {

  private static final Logger log = LoggerFactory.getLogger(FailoverRateLimiterBackend.class);

  private final RateLimiterBackend primary;
  private final RateLimiterBackend localBackend = new LocalRateLimiterBackend();
  private volatile boolean usingFallback;

  public FailoverRateLimiterBackend(RateLimiterBackend primary) {
    this.primary = primary;
  }

  @Override
  public long incrementAndGet(String key, Duration window) {
    try {
      long count = primary.incrementAndGet(key, window);
      if (usingFallback) {
        log.info("Redis reachable again; rate limiting resumed on the shared counter");
      }
      usingFallback = false;
      return count;
    } catch (Exception e) {
      if (!usingFallback) {
        log.warn(
            "Redis unavailable for rate limiting ({}); falling back to a local, per-replica counter",
            e.getMessage());
      }
      usingFallback = true;
      return localBackend.incrementAndGet(key, window);
    }
  }

  /** Exposed for observability/tests: true while serving from the local fallback. */
  public boolean isUsingFallback() {
    return usingFallback;
  }
}
