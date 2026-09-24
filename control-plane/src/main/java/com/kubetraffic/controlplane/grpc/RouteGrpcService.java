/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.grpc;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.protobuf.Empty;
import com.google.protobuf.util.JsonFormat;
import com.kubetraffic.controlplane.grpc.v1.Decision;
import com.kubetraffic.controlplane.grpc.v1.RouteConfig;
import com.kubetraffic.controlplane.grpc.v1.RouteRef;
import com.kubetraffic.controlplane.grpc.v1.RouteServiceGrpc;
import com.kubetraffic.controlplane.grpc.v1.RouteSpec;
import com.kubetraffic.controlplane.grpc.v1.RuleWeights;
import com.kubetraffic.controlplane.grpc.v1.VersionWeight;
import com.kubetraffic.controlplane.policy.PolicyDtos.Response;
import com.kubetraffic.controlplane.policy.PolicyService;
import com.kubetraffic.controlplane.routing.RouteConfigDtos;
import com.kubetraffic.controlplane.routing.RouteConfigService;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.util.HashMap;
import java.util.Map;
import java.util.Optional;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Component;

/**
 * Implements RouteService. A route is stored as a {@code policy} row (reusing
 * the Phase 7 persistence layer): {@code namespace/name} come from the
 * RouteRef, {@code spec} is the RouteSpec re-encoded as JSON via {@link
 * JsonFormat} so the storage layer stays proto-agnostic.
 *
 * <p>There is no autonomous rule engine yet (Phase 13): the weights returned
 * here come from {@link RouteConfigService} — an active canary progression for
 * the path if one exists, otherwise the spec's own declared weights, unchanged.
 * {@link #registerRoute} also diffs the *spec's* weights against whatever was
 * previously stored and broadcasts a {@link Decision} for every version whose
 * weight changed, independent of any canary — proving the StreamDecisions
 * plumbing before an autonomous engine drives it.
 */
@Component
public class RouteGrpcService extends RouteServiceGrpc.RouteServiceImplBase {

  private static final Logger log = LoggerFactory.getLogger(RouteGrpcService.class);

  private final PolicyService policies;
  private final RouteConfigService routeConfigs;
  private final DecisionPublisher decisions;
  private final ObjectMapper mapper;

  public RouteGrpcService(
      PolicyService policies, RouteConfigService routeConfigs, DecisionPublisher decisions, ObjectMapper mapper) {
    this.policies = policies;
    this.routeConfigs = routeConfigs;
    this.decisions = decisions;
    this.mapper = mapper;
  }

  @Override
  public void registerRoute(RouteSpec request, StreamObserver<RouteConfig> responseObserver) {
    try {
      String namespace = request.getRef().getNamespace();
      String name = request.getRef().getName();

      Optional<Response> previous = policies.tryGet(namespace, name);
      JsonNode specJson = toJson(request);
      Response saved = policies.upsert(namespace, name, specJson);

      previous.ifPresent(p -> broadcastWeightChanges(request.getRef(), p.spec(), request));

      log.info(
          "registered route {}/{} (generation={}, {} rule(s))",
          namespace, name, saved.generation(), request.getRulesCount());

      RouteConfigDtos.Response config =
          routeConfigs.get(namespace, name).orElseThrow(); // just upserted above; must exist
      responseObserver.onNext(toProto(request.getRef(), config));
      responseObserver.onCompleted();
    } catch (Exception e) {
      log.error("RegisterRoute failed", e);
      responseObserver.onError(
          Status.INTERNAL.withDescription(e.getMessage()).withCause(e).asRuntimeException());
    }
  }

  @Override
  public void deleteRoute(RouteRef request, StreamObserver<Empty> responseObserver) {
    try {
      policies.deleteIfExists(request.getNamespace(), request.getName());
      log.info("deleted route {}/{}", request.getNamespace(), request.getName());
      responseObserver.onNext(Empty.getDefaultInstance());
      responseObserver.onCompleted();
    } catch (Exception e) {
      log.error("DeleteRoute failed", e);
      responseObserver.onError(
          Status.INTERNAL.withDescription(e.getMessage()).withCause(e).asRuntimeException());
    }
  }

  @Override
  public void getRouteConfig(RouteRef request, StreamObserver<RouteConfig> responseObserver) {
    Optional<RouteConfigDtos.Response> config = routeConfigs.get(request.getNamespace(), request.getName());
    if (config.isEmpty()) {
      responseObserver.onError(
          Status.NOT_FOUND
              .withDescription("route %s/%s not found".formatted(request.getNamespace(), request.getName()))
              .asRuntimeException());
      return;
    }
    responseObserver.onNext(toProto(request, config.get()));
    responseObserver.onCompleted();
  }

  private void broadcastWeightChanges(RouteRef ref, JsonNode previousSpec, RouteSpec newSpec) {
    Map<String, Map<String, Integer>> oldWeights = extractWeights(previousSpec);
    for (var rule : newSpec.getRulesList()) {
      Map<String, Integer> oldForPath = oldWeights.getOrDefault(rule.getPath(), Map.of());
      for (var version : rule.getVersionsList()) {
        Integer old = oldForPath.get(version.getName());
        if (old != null && old != version.getWeight()) {
          decisions.publish(
              Decision.newBuilder()
                  .setRef(ref)
                  .setPath(rule.getPath())
                  .setVersion(version.getName())
                  .setOldWeight(old)
                  .setNewWeight(version.getWeight())
                  .setReason("spec-updated")
                  .setTimestampUnixMs(System.currentTimeMillis())
                  .build());
        }
      }
    }
  }

  private static Map<String, Map<String, Integer>> extractWeights(JsonNode spec) {
    Map<String, Map<String, Integer>> out = new HashMap<>();
    for (JsonNode rule : spec.path("rules")) {
      Map<String, Integer> weights = new HashMap<>();
      for (JsonNode version : rule.path("versions")) {
        weights.put(version.path("name").asText(), version.path("weight").asInt());
      }
      out.put(rule.path("path").asText(), weights);
    }
    return out;
  }

  private static RouteConfig toProto(RouteRef ref, RouteConfigDtos.Response dto) {
    RouteConfig.Builder config = RouteConfig.newBuilder().setRef(ref).setGeneration(dto.generation());
    for (var rule : dto.rules()) {
      RuleWeights.Builder weights = RuleWeights.newBuilder().setPath(rule.path());
      for (var v : rule.weights()) {
        weights.addWeights(VersionWeight.newBuilder().setVersion(v.version()).setWeight(v.weight()));
      }
      config.addRules(weights);
    }
    return config.build();
  }

  private JsonNode toJson(RouteSpec spec) {
    try {
      String json = JsonFormat.printer().includingDefaultValueFields().print(spec);
      return mapper.readTree(json);
    } catch (Exception e) {
      throw new IllegalStateException("failed to serialize RouteSpec", e);
    }
  }
}
