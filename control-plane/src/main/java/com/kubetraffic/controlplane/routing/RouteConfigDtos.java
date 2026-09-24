/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.routing;

import java.util.List;

/** The effective (canary-aware) routing config for a stored route — plain JSON, no protobuf. */
public final class RouteConfigDtos {

  private RouteConfigDtos() {}

  public record VersionWeight(String version, int weight) {}

  public record RuleWeights(String path, List<VersionWeight> weights) {}

  public record Response(String namespace, String name, long generation, List<RuleWeights> rules) {}
}
