/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.grpc;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.kubetraffic.controlplane.grpc.v1.Decision;
import com.kubetraffic.controlplane.grpc.v1.DecisionServiceGrpc;
import com.kubetraffic.controlplane.grpc.v1.RouteConfig;
import com.kubetraffic.controlplane.grpc.v1.RouteRef;
import com.kubetraffic.controlplane.grpc.v1.RouteServiceGrpc;
import com.kubetraffic.controlplane.grpc.v1.RouteSpec;
import com.kubetraffic.controlplane.grpc.v1.RouteVersionSpec;
import com.kubetraffic.controlplane.grpc.v1.RulePolicy;
import com.kubetraffic.controlplane.grpc.v1.WatchDecisionsRequest;
import com.kubetraffic.controlplane.policy.PolicyDtos.Response;
import com.kubetraffic.controlplane.policy.PolicyService;
import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import io.grpc.stub.StreamObserver;
import java.time.Instant;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

/** Exercises RouteGrpcService + DecisionGrpcService over an in-process gRPC channel. */
class RouteGrpcServiceTest {

  private final PolicyService policies = mock(PolicyService.class);
  private final DecisionGrpcService decisionService = new DecisionGrpcService();
  private final ObjectMapper mapper = new ObjectMapper();

  private Server server;
  private ManagedChannel channel;

  @BeforeEach
  void startInProcessServer() throws Exception {
    String name = InProcessServerBuilder.generateName();
    server =
        InProcessServerBuilder.forName(name)
            .directExecutor()
            .addService(new RouteGrpcService(policies, decisionService, mapper))
            .addService(decisionService)
            .build()
            .start();
    channel = InProcessChannelBuilder.forName(name).directExecutor().build();
  }

  @AfterEach
  void stop() {
    channel.shutdownNow();
    server.shutdownNow();
  }

  private static RouteSpec spec(String ns, String name, int w1, int w2) {
    return RouteSpec.newBuilder()
        .setRef(RouteRef.newBuilder().setNamespace(ns).setName(name))
        .setHost("api.example.com")
        .addRules(
            RulePolicy.newBuilder()
                .setPath("/payment")
                .setBackendService("payment")
                .setBackendPort(8080)
                .addVersions(RouteVersionSpec.newBuilder().setName("v1").setWeight(w1))
                .addVersions(RouteVersionSpec.newBuilder().setName("v2").setWeight(w2)))
        .build();
  }

  private static Response response(String ns, String name, long generation) {
    return new Response(
        UUID.randomUUID(), ns, name, generation, new ObjectMapper().createObjectNode(), Instant.now(), Instant.now());
  }

  @Test
  void registerRoute_createsWhenAbsent() {
    when(policies.tryGet("demo", "payment-route")).thenReturn(Optional.empty());
    when(policies.upsert(eq("demo"), eq("payment-route"), any()))
        .thenReturn(response("demo", "payment-route", 1L));

    RouteConfig config =
        RouteServiceGrpc.newBlockingStub(channel).registerRoute(spec("demo", "payment-route", 90, 10));

    assertThat(config.getGeneration()).isEqualTo(1L);
    assertThat(config.getRules(0).getPath()).isEqualTo("/payment");
    assertThat(config.getRules(0).getWeights(0).getVersion()).isEqualTo("v1");
    assertThat(config.getRules(0).getWeights(0).getWeight()).isEqualTo(90);
    verify(policies).upsert(eq("demo"), eq("payment-route"), any());
  }

  @Test
  void registerRoute_broadcastsDecisionOnWeightChange() throws Exception {
    var previousSpec =
        mapper.readTree(
            "{\"rules\":[{\"path\":\"/payment\",\"versions\":"
                + "[{\"name\":\"v1\",\"weight\":90},{\"name\":\"v2\",\"weight\":10}]}]}");
    when(policies.tryGet("demo", "payment-route"))
        .thenReturn(Optional.of(new Response(UUID.randomUUID(), "demo", "payment-route", 1L, previousSpec, Instant.now(), Instant.now())));
    when(policies.upsert(eq("demo"), eq("payment-route"), any()))
        .thenReturn(response("demo", "payment-route", 2L));

    BlockingQueue<Decision> received = new LinkedBlockingQueue<>();
    DecisionServiceGrpc.newStub(channel)
        .streamDecisions(
            WatchDecisionsRequest.getDefaultInstance(),
            new StreamObserver<>() {
              @Override
              public void onNext(Decision value) {
                received.add(value);
              }

              @Override
              public void onError(Throwable t) {}

              @Override
              public void onCompleted() {}
            });

    RouteServiceGrpc.newBlockingStub(channel).registerRoute(spec("demo", "payment-route", 50, 50));

    Decision first = received.poll(2, TimeUnit.SECONDS);
    Decision second = received.poll(2, TimeUnit.SECONDS);
    assertThat(first).isNotNull();
    assertThat(second).isNotNull();
    assertThat(first.getOldWeight()).isEqualTo(90);
    assertThat(first.getNewWeight()).isEqualTo(50);
    assertThat(first.getReason()).isEqualTo("spec-updated");
    assertThat(second.getOldWeight()).isEqualTo(10);
    assertThat(second.getNewWeight()).isEqualTo(50);
  }

  @Test
  void registerRoute_noBroadcastWhenWeightsUnchanged() throws Exception {
    var previousSpec =
        mapper.readTree("{\"rules\":[{\"path\":\"/payment\",\"versions\":[{\"name\":\"v1\",\"weight\":90},{\"name\":\"v2\",\"weight\":10}]}]}");
    when(policies.tryGet("demo", "payment-route"))
        .thenReturn(Optional.of(new Response(UUID.randomUUID(), "demo", "payment-route", 1L, previousSpec, Instant.now(), Instant.now())));
    when(policies.upsert(eq("demo"), eq("payment-route"), any()))
        .thenReturn(response("demo", "payment-route", 1L));

    BlockingQueue<Decision> received = new LinkedBlockingQueue<>();
    DecisionServiceGrpc.newStub(channel)
        .streamDecisions(
            WatchDecisionsRequest.getDefaultInstance(),
            new StreamObserver<>() {
              @Override
              public void onNext(Decision value) {
                received.add(value);
              }

              @Override
              public void onError(Throwable t) {}

              @Override
              public void onCompleted() {}
            });

    RouteServiceGrpc.newBlockingStub(channel).registerRoute(spec("demo", "payment-route", 90, 10));

    assertThat(received.poll(500, TimeUnit.MILLISECONDS)).isNull();
  }

  @Test
  void getRouteConfig_notFound() {
    when(policies.tryGet("demo", "missing")).thenReturn(Optional.empty());
    var ref = RouteRef.newBuilder().setNamespace("demo").setName("missing").build();

    assertThatThrownBy(() -> RouteServiceGrpc.newBlockingStub(channel).getRouteConfig(ref))
        .isInstanceOf(StatusRuntimeException.class)
        .extracting(e -> ((StatusRuntimeException) e).getStatus().getCode())
        .isEqualTo(Status.Code.NOT_FOUND);
  }

  @Test
  void deleteRoute_isIdempotent() {
    var ref = RouteRef.newBuilder().setNamespace("demo").setName("payment-route").build();
    RouteServiceGrpc.newBlockingStub(channel).deleteRoute(ref);
    verify(policies).deleteIfExists("demo", "payment-route");
  }
}
