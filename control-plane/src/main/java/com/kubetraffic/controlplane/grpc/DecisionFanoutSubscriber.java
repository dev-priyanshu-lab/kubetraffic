/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.grpc;

import com.google.protobuf.util.JsonFormat;
import com.kubetraffic.controlplane.grpc.v1.Decision;
import java.nio.charset.StandardCharsets;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.data.redis.connection.Message;
import org.springframework.data.redis.connection.MessageListener;
import org.springframework.stereotype.Component;

/** Receives every decision published to Redis (by any replica) and fans it out locally. */
@Component
public class DecisionFanoutSubscriber implements MessageListener {

  private static final Logger log = LoggerFactory.getLogger(DecisionFanoutSubscriber.class);

  private final DecisionGrpcService local;

  public DecisionFanoutSubscriber(DecisionGrpcService local) {
    this.local = local;
  }

  @Override
  public void onMessage(Message message, byte[] pattern) {
    try {
      Decision.Builder builder = Decision.newBuilder();
      JsonFormat.parser().ignoringUnknownFields().merge(new String(message.getBody(), StandardCharsets.UTF_8), builder);
      local.fanOutLocally(builder.build());
    } catch (Exception e) {
      log.warn("failed to decode a decision received via Redis: {}", e.getMessage());
    }
  }
}
