/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.grpc;

import com.kubetraffic.controlplane.grpc.v1.Decision;
import com.kubetraffic.controlplane.grpc.v1.DecisionServiceGrpc;
import com.kubetraffic.controlplane.grpc.v1.WatchDecisionsRequest;
import io.grpc.stub.ServerCallStreamObserver;
import io.grpc.stub.StreamObserver;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Component;

/**
 * Implements DecisionService.StreamDecisions as a simple fan-out to whichever
 * controllers happen to be subscribed <b>to this pod</b>. That "this pod" is
 * the reason business services never call {@link #fanOutLocally} directly: a
 * gRPC connection sticks to one replica for its lifetime, so a decision
 * produced while handling a REST call on replica B would never reach a
 * controller subscribed to replica A. {@link RedisDecisionPublisher} is the
 * real {@link DecisionPublisher} — it publishes to every replica over Redis
 * Pub/Sub, and {@link DecisionFanoutSubscriber} calls back into this class's
 * {@link #fanOutLocally} on each one.
 */
@Component
public class DecisionGrpcService extends DecisionServiceGrpc.DecisionServiceImplBase {

  private static final Logger log = LoggerFactory.getLogger(DecisionGrpcService.class);

  private final Set<StreamObserver<Decision>> subscribers = ConcurrentHashMap.newKeySet();

  @Override
  public void streamDecisions(
      WatchDecisionsRequest request, StreamObserver<Decision> responseObserver) {
    subscribers.add(responseObserver);
    log.info("decision stream subscriber connected (total={})", subscribers.size());

    if (responseObserver instanceof ServerCallStreamObserver<Decision> serverObserver) {
      serverObserver.setOnCancelHandler(
          () -> {
            subscribers.remove(responseObserver);
            log.info("decision stream subscriber disconnected (total={})", subscribers.size());
          });
    }
  }

  /** Pushes a decision to every subscriber connected to this replica. */
  public void fanOutLocally(Decision decision) {
    log.info(
        "decision route={}/{} path={} version={} weight {}->{} reason={}",
        decision.getRef().getNamespace(),
        decision.getRef().getName(),
        decision.getPath(),
        decision.getVersion(),
        decision.getOldWeight(),
        decision.getNewWeight(),
        decision.getReason());

    for (StreamObserver<Decision> subscriber : subscribers) {
      try {
        subscriber.onNext(decision);
      } catch (Exception e) {
        log.warn("dropping decision stream subscriber after send failure: {}", e.getMessage());
        subscribers.remove(subscriber);
      }
    }
  }

  int subscriberCount() {
    return subscribers.size();
  }
}
