/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.grpc;

import com.google.protobuf.util.JsonFormat;
import com.kubetraffic.controlplane.grpc.v1.Decision;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;

/**
 * The {@link DecisionPublisher} business services actually depend on: it publishes to a Redis
 * Pub/Sub channel that every control-plane replica subscribes to (see {@link
 * DecisionFanoutSubscriber}), so a decision reaches every controller regardless of which replica
 * handled the request that produced it — a single gRPC connection sticks to one replica for its
 * lifetime, so an in-process-only fan-out would silently drop decisions produced on any other
 * replica.
 *
 * <p>Fails open: if Redis is unreachable, the REST/gRPC call that produced the decision still
 * succeeds (the state change is already durable in Postgres) — the controller just won't be
 * notified until its next resync.
 */
@Component
public class RedisDecisionPublisher implements DecisionPublisher {

  public static final String CHANNEL = "kubetraffic:decisions";

  private static final Logger log = LoggerFactory.getLogger(RedisDecisionPublisher.class);

  private final StringRedisTemplate redis;

  public RedisDecisionPublisher(StringRedisTemplate redis) {
    this.redis = redis;
  }

  @Override
  public void publish(Decision decision) {
    try {
      String json = JsonFormat.printer().print(decision);
      redis.convertAndSend(CHANNEL, json);
    } catch (Exception e) {
      log.warn(
          "failed to publish decision via Redis ({}); the controller will only see this on its next resync",
          e.getMessage());
    }
  }
}
