/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.ratelimit;

import java.time.Duration;
import org.springframework.stereotype.Service;

/**
 * Distributed fixed-window rate limiting. Not yet wired to live traffic decisions (that lands with
 * the rule engine in Phase 13, which will read {@code spec.security.rateLimit} from each route);
 * Phase 9's job is the shared, fail-open counter primitive itself — {@code limit}/{@code window}
 * are caller-supplied until then.
 */
@Service
public class RateLimiterService {

  private final RateLimiterBackend backend;

  public RateLimiterService(RateLimiterBackend backend) {
    this.backend = backend;
  }

  /** Consumes one unit from key's bucket and reports whether it was still under limit. */
  public RateLimitResult tryAcquire(String key, long limit, Duration window) {
    long count = backend.incrementAndGet(key, window);
    boolean allowed = count <= limit;
    long remaining = Math.max(0, limit - count);
    return new RateLimitResult(allowed, count, limit, remaining);
  }
}
