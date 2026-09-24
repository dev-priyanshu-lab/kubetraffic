/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.canary;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.kubetraffic.controlplane.audit.AuditService;
import com.kubetraffic.controlplane.grpc.DecisionPublisher;
import com.kubetraffic.controlplane.grpc.v1.Decision;
import com.kubetraffic.controlplane.grpc.v1.RouteRef;
import com.kubetraffic.controlplane.persistence.CanaryEntity;
import com.kubetraffic.controlplane.persistence.CanaryRepository;
import com.kubetraffic.controlplane.policy.NotFoundException;
import com.kubetraffic.controlplane.policy.PolicyDtos;
import com.kubetraffic.controlplane.policy.PolicyService;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * The canary progression state machine: walks a stable/canary version pair up a weight ladder
 * (0 implied -&gt; steps[0] -&gt; ... -&gt; 100), or rolls back to 0 at any point. There is no automatic
 * promotion yet (Phase 13's rule engine will call {@link #promote} based on live health signals,
 * the same way it will call {@link com.kubetraffic.controlplane.circuitbreaker.CircuitBreakerService}
 * and {@link com.kubetraffic.controlplane.ratelimit.RateLimiterService}); for now promotion is a
 * REST call.
 *
 * <p>Every transition is audited and broadcasts a {@link Decision} so any connected controller
 * reprograms the data plane immediately, without waiting for its periodic resync.
 */
@Service
public class CanaryService {

  private final CanaryRepository canaries;
  private final PolicyService policies;
  private final AuditService audit;
  private final DecisionPublisher decisions;
  private final ObjectMapper mapper;

  public CanaryService(
      CanaryRepository canaries, PolicyService policies, AuditService audit, DecisionPublisher decisions, ObjectMapper mapper) {
    this.canaries = canaries;
    this.policies = policies;
    this.audit = audit;
    this.decisions = decisions;
    this.mapper = mapper;
  }

  @Transactional
  public CanaryDtos.Response start(
      String namespace, String name, String path, String stableVersion, String canaryVersion, List<Integer> requestedSteps) {
    PolicyDtos.Response policy = requirePolicy(namespace, name);
    List<Integer> steps = CanaryLadder.validate(requestedSteps);
    String stepsJson = writeSteps(steps);

    CanaryEntity entity =
        canaries
            .findByPolicyIdAndPath(policy.id(), path)
            .map(
                existing -> {
                  existing.reinitialize(stableVersion, canaryVersion, stepsJson, CanaryStatus.PROGRESSING.name());
                  return existing;
                })
            .orElseGet(
                () ->
                    new CanaryEntity(
                        policy.id(), path, stableVersion, canaryVersion, stepsJson, CanaryStatus.PROGRESSING.name()));
    canaries.save(entity);

    int firstWeight = steps.get(0);
    audit.record(
        "api",
        "canary.start",
        target(namespace, name, path),
        "{\"stableVersion\":\"%s\",\"canaryVersion\":\"%s\",\"weight\":%d}"
            .formatted(stableVersion, canaryVersion, firstWeight));
    publish(namespace, name, path, canaryVersion, 0, firstWeight, "canary-start");

    return toResponse(namespace, name, entity);
  }

  @Transactional
  public CanaryDtos.Response promote(String namespace, String name, String path) {
    PolicyDtos.Response policy = requirePolicy(namespace, name);
    CanaryEntity entity = requireCanary(policy.id(), name, path);
    CanaryStatus status = CanaryStatus.valueOf(entity.getStatus());

    if (status == CanaryStatus.PROMOTED || status == CanaryStatus.ROLLED_BACK) {
      return toResponse(namespace, name, entity); // terminal states: promote is a no-op
    }

    List<Integer> steps = readSteps(entity.getSteps());
    int oldWeight = steps.get(entity.getStepIndex());
    int nextIndex = Math.min(entity.getStepIndex() + 1, steps.size() - 1);
    int newWeight = steps.get(nextIndex);

    entity.setStepIndex(nextIndex);
    if (newWeight >= 100) {
      entity.setStatus(CanaryStatus.PROMOTED.name());
    }
    canaries.save(entity);

    audit.record(
        "api",
        "canary.promote",
        target(namespace, name, path),
        "{\"oldWeight\":%d,\"newWeight\":%d}".formatted(oldWeight, newWeight));
    publish(namespace, name, path, entity.getCanaryVersion(), oldWeight, newWeight, "canary-promote");

    return toResponse(namespace, name, entity);
  }

  @Transactional
  public CanaryDtos.Response rollback(String namespace, String name, String path) {
    PolicyDtos.Response policy = requirePolicy(namespace, name);
    CanaryEntity entity = requireCanary(policy.id(), name, path);
    List<Integer> steps = readSteps(entity.getSteps());
    int oldWeight = steps.get(entity.getStepIndex());

    entity.setStatus(CanaryStatus.ROLLED_BACK.name());
    canaries.save(entity);

    audit.record(
        "api",
        "canary.rollback",
        target(namespace, name, path),
        "{\"oldWeight\":%d,\"newWeight\":0}".formatted(oldWeight));
    publish(namespace, name, path, entity.getCanaryVersion(), oldWeight, 0, "canary-rollback");

    return toResponse(namespace, name, entity);
  }

  @Transactional(readOnly = true)
  public CanaryDtos.Response get(String namespace, String name, String path) {
    PolicyDtos.Response policy = requirePolicy(namespace, name);
    return toResponse(namespace, name, requireCanary(policy.id(), name, path));
  }

  /**
   * The canary-overridden {@code {version: weight}} pair for path, if a canary is active. Used by
   * {@code RouteGrpcService} to override the spec's declared weights when building a RouteConfig.
   */
  @Transactional(readOnly = true)
  public Optional<Map<String, Integer>> effectiveWeights(UUID policyId, String path) {
    return canaries
        .findByPolicyIdAndPath(policyId, path)
        .map(
            entity -> {
              List<Integer> steps = readSteps(entity.getSteps());
              int canaryWeight =
                  switch (CanaryStatus.valueOf(entity.getStatus())) {
                    case ROLLED_BACK -> 0;
                    case PROMOTED -> 100;
                    case PROGRESSING -> steps.get(entity.getStepIndex());
                  };
              Map<String, Integer> weights = new HashMap<>();
              weights.put(entity.getCanaryVersion(), canaryWeight);
              weights.put(entity.getStableVersion(), 100 - canaryWeight);
              return weights;
            });
  }

  private void publish(
      String namespace, String name, String path, String version, int oldWeight, int newWeight, String reason) {
    decisions.publish(
        Decision.newBuilder()
            .setRef(RouteRef.newBuilder().setNamespace(namespace).setName(name))
            .setPath(path)
            .setVersion(version)
            .setOldWeight(oldWeight)
            .setNewWeight(newWeight)
            .setReason(reason)
            .setTimestampUnixMs(System.currentTimeMillis())
            .build());
  }

  private PolicyDtos.Response requirePolicy(String namespace, String name) {
    return policies
        .tryGet(namespace, name)
        .orElseThrow(() -> new NotFoundException("policy %s/%s not found".formatted(displayNs(namespace), name)));
  }

  private CanaryEntity requireCanary(UUID policyId, String name, String path) {
    return canaries
        .findByPolicyIdAndPath(policyId, path)
        .orElseThrow(
            () -> new NotFoundException("no canary in progress for %s path %s".formatted(name, path)));
  }

  private CanaryDtos.Response toResponse(String namespace, String name, CanaryEntity e) {
    List<Integer> steps = readSteps(e.getSteps());
    int currentWeight =
        switch (CanaryStatus.valueOf(e.getStatus())) {
          case ROLLED_BACK -> 0;
          case PROMOTED -> 100;
          case PROGRESSING -> steps.get(e.getStepIndex());
        };
    return new CanaryDtos.Response(
        namespace,
        name,
        e.getPath(),
        CanaryStatus.valueOf(e.getStatus()),
        e.getStableVersion(),
        e.getCanaryVersion(),
        steps,
        e.getStepIndex(),
        currentWeight,
        e.getStartedAt(),
        e.getUpdatedAt());
  }

  private static String displayNs(String ns) {
    return (ns == null || ns.isBlank()) ? "default" : ns;
  }

  private static String target(String namespace, String name, String path) {
    return "policy/" + displayNs(namespace) + "/" + name + path;
  }

  private String writeSteps(List<Integer> steps) {
    try {
      return mapper.writeValueAsString(steps);
    } catch (Exception e) {
      throw new IllegalStateException("failed to serialize canary steps", e);
    }
  }

  private List<Integer> readSteps(String json) {
    try {
      return mapper.readValue(json, new TypeReference<List<Integer>>() {});
    } catch (Exception e) {
      throw new IllegalStateException("stored canary steps are not valid JSON", e);
    }
  }
}
