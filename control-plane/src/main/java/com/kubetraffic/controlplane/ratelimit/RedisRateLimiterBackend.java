/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.ratelimit;

import java.time.Duration;
import java.time.Instant;
import java.util.List;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.script.DefaultRedisScript;
import org.springframework.data.redis.core.script.RedisScript;

/**
 * A fixed-window counter shared across every control-plane replica via Redis. The window is
 * wall-clock aligned ({@code epochSecond / windowSeconds}) so every replica derives the same
 * bucket independently — no coordination beyond the shared counter itself. INCR+conditional-EXPIRE
 * run as one Lua script so the counter and its TTL are set atomically.
 */
public class RedisRateLimiterBackend implements RateLimiterBackend {

  private static final RedisScript<Long> INCR_WITH_TTL =
      new DefaultRedisScript<>(
          "local c = redis.call('INCR', KEYS[1]) "
              + "if c == 1 then redis.call('EXPIRE', KEYS[1], ARGV[1]) end "
              + "return c",
          Long.class);

  private final StringRedisTemplate redis;

  public RedisRateLimiterBackend(StringRedisTemplate redis) {
    this.redis = redis;
  }

  @Override
  public long incrementAndGet(String key, Duration window) {
    long windowSeconds = Math.max(1, window.toSeconds());
    long bucket = Instant.now().getEpochSecond() / windowSeconds;
    String redisKey = "ratelimit:%s:%d".formatted(key, bucket);

    Long count =
        redis.execute(INCR_WITH_TTL, List.of(redisKey), String.valueOf(windowSeconds * 2));
    if (count == null) {
      throw new IllegalStateException("Redis INCR returned null for " + redisKey);
    }
    return count;
  }
}
