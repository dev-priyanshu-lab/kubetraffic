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
 * Implements DecisionService.StreamDecisions as a simple fan-out: every
 * connected controller gets every decision. Until the rule engine (Phase 13)
 * exists, decisions are only produced by {@link RouteGrpcService} echoing spec
 * weight changes — but the wiring here (subscribe, fan-out, unsubscribe on
 * cancel) is exactly what the real engine will use.
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

  /** Pushes a decision to every currently-connected subscriber. */
  public void broadcast(Decision decision) {
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
