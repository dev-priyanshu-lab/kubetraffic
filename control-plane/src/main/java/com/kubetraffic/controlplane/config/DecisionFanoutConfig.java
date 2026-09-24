/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.config;

import com.kubetraffic.controlplane.grpc.DecisionFanoutSubscriber;
import com.kubetraffic.controlplane.grpc.RedisDecisionPublisher;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.data.redis.connection.RedisConnectionFactory;
import org.springframework.data.redis.listener.ChannelTopic;
import org.springframework.data.redis.listener.RedisMessageListenerContainer;

/** Subscribes every replica to the Redis Pub/Sub channel decisions are broadcast on. */
@Configuration
public class DecisionFanoutConfig {

  @Bean
  public RedisMessageListenerContainer decisionFanoutContainer(
      RedisConnectionFactory connectionFactory, DecisionFanoutSubscriber subscriber) {
    RedisMessageListenerContainer container = new RedisMessageListenerContainer();
    container.setConnectionFactory(connectionFactory);
    container.addMessageListener(subscriber, new ChannelTopic(RedisDecisionPublisher.CHANNEL));
    return container;
  }
}
