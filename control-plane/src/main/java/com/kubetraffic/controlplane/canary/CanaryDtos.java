/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.canary;

import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;
import java.time.Instant;
import java.util.List;

/** Request/response payloads for the canary API. */
public final class CanaryDtos {

  private CanaryDtos() {}

  /** Body for POST /api/v1/canary/{name}/start. */
  public record StartRequest(
      @NotBlank String stableVersion,
      @NotBlank String canaryVersion,
      List<@Min(1) @Max(100) Integer> steps) {}

  /** Response for every canary endpoint. */
  public record Response(
      String namespace,
      String name,
      String path,
      CanaryStatus status,
      String stableVersion,
      String canaryVersion,
      List<Integer> steps,
      int stepIndex,
      int currentCanaryWeight,
      Instant startedAt,
      Instant updatedAt) {}
}
