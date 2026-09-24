/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.routing;

import com.fasterxml.jackson.databind.JsonNode;
import com.kubetraffic.controlplane.canary.CanaryService;
import com.kubetraffic.controlplane.policy.PolicyDtos;
import com.kubetraffic.controlplane.policy.PolicyService;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * Computes the effective (canary-aware) routing config for a stored route — the single source of
 * truth both {@code RouteGrpcService} (the controller's view, as protobuf) and the REST API/console
 * (a human's view, as JSON) read from, so the two can never disagree.
 */
@Service
public class RouteConfigService {

  private final PolicyService policies;
  private final CanaryService canaries;

  public RouteConfigService(PolicyService policies, CanaryService canaries) {
    this.policies = policies;
    this.canaries = canaries;
  }

  @Transactional(readOnly = true)
  public Optional<RouteConfigDtos.Response> get(String namespace, String name) {
    return policies.tryGet(namespace, name).map(this::build);
  }

  private RouteConfigDtos.Response build(PolicyDtos.Response policy) {
    List<RouteConfigDtos.RuleWeights> rules = new ArrayList<>();
    for (JsonNode rule : policy.spec().path("rules")) {
      String path = rule.path("path").asText();
      Map<String, Integer> override = canaries.effectiveWeights(policy.id(), path).orElse(null);

      List<RouteConfigDtos.VersionWeight> weights = new ArrayList<>();
      for (JsonNode version : rule.path("versions")) {
        String name = version.path("name").asText();
        int specWeight = version.path("weight").asInt();
        int weight = override != null ? override.getOrDefault(name, specWeight) : specWeight;
        weights.add(new RouteConfigDtos.VersionWeight(name, weight));
      }
      rules.add(new RouteConfigDtos.RuleWeights(path, weights));
    }
    return new RouteConfigDtos.Response(policy.namespace(), policy.name(), policy.generation(), rules);
  }
}
