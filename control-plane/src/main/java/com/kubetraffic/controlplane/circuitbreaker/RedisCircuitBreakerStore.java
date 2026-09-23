/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.circuitbreaker;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.data.redis.core.StringRedisTemplate;

/**
 * Shares circuit state across every control-plane replica via Redis, so a pod restart or a
 * decision made by one replica is immediately visible to the others.
 */
public class RedisCircuitBreakerStore implements CircuitBreakerStore {

  private static final String KEY_PREFIX = "cb:";

  private final StringRedisTemplate redis;
  private final ObjectMapper mapper;

  public RedisCircuitBreakerStore(StringRedisTemplate redis, ObjectMapper mapper) {
    this.redis = redis;
    this.mapper = mapper;
  }

  @Override
  public CircuitBreakerRecord get(String key) {
    String json = redis.opsForValue().get(KEY_PREFIX + key);
    if (json == null) {
      return CircuitBreakerRecord.closed();
    }
    try {
      return mapper.readValue(json, CircuitBreakerRecord.class);
    } catch (Exception e) {
      throw new IllegalStateException("corrupt circuit breaker record for " + key, e);
    }
  }

  @Override
  public void put(String key, CircuitBreakerRecord record) {
    try {
      redis.opsForValue().set(KEY_PREFIX + key, mapper.writeValueAsString(record));
    } catch (Exception e) {
      throw new IllegalStateException("failed to serialize circuit breaker record for " + key, e);
    }
  }
}
