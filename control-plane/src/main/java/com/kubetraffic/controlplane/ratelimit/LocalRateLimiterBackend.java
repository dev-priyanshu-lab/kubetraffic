/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.ratelimit;

import java.time.Duration;
import java.time.Instant;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Per-JVM fallback used while Redis is unreachable. Same wall-clock-aligned bucketing as {@link
 * RedisRateLimiterBackend} so behaviour doesn't change character on failover — it just stops being
 * shared across replicas. Buckets are never purged (acceptable: this path only runs during a Redis
 * outage, and process restarts reclaim the memory).
 */
public class LocalRateLimiterBackend implements RateLimiterBackend {

  private final ConcurrentHashMap<String, AtomicLong> buckets = new ConcurrentHashMap<>();

  @Override
  public long incrementAndGet(String key, Duration window) {
    long windowSeconds = Math.max(1, window.toSeconds());
    long bucket = Instant.now().getEpochSecond() / windowSeconds;
    String bucketKey = key + ":" + bucket;
    return buckets.computeIfAbsent(bucketKey, k -> new AtomicLong()).incrementAndGet();
  }
}
