/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.config;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.kubetraffic.controlplane.circuitbreaker.FailoverCircuitBreakerStore;
import com.kubetraffic.controlplane.circuitbreaker.RedisCircuitBreakerStore;
import com.kubetraffic.controlplane.ratelimit.FailoverRateLimiterBackend;
import com.kubetraffic.controlplane.ratelimit.RedisRateLimiterBackend;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.data.redis.core.StringRedisTemplate;

/**
 * Wires the Redis-backed primaries into their fail-open wrappers. Kept as explicit {@code @Bean}
 * methods (rather than {@code @Component} on the Failover classes themselves) so those classes
 * stay plain, constructor-injected, and unit-testable without a Spring context or a real Redis.
 */
@Configuration
public class ResilienceConfig {

  @Bean
  public FailoverRateLimiterBackend rateLimiterBackend(StringRedisTemplate redis) {
    return new FailoverRateLimiterBackend(new RedisRateLimiterBackend(redis));
  }

  @Bean
  public FailoverCircuitBreakerStore circuitBreakerStore(StringRedisTemplate redis, ObjectMapper mapper) {
    return new FailoverCircuitBreakerStore(new RedisCircuitBreakerStore(redis, mapper));
  }
}
