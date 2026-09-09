/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.policy;

import com.fasterxml.jackson.databind.JsonNode;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;
import java.time.Instant;
import java.util.UUID;

/** Request/response payloads for the policy API. */
public final class PolicyDtos {

  private PolicyDtos() {}

  /** Body for POST /api/v1/policies. */
  public record CreateRequest(
      @Size(max = 253) String namespace,
      @NotBlank @Size(max = 253) String name,
      @NotNull JsonNode spec) {}

  /** Body for PUT /api/v1/policies/{name}. */
  public record UpdateRequest(@Size(max = 253) String namespace, @NotNull JsonNode spec) {}

  /** Response for all policy endpoints. */
  public record Response(
      UUID id,
      String namespace,
      String name,
      long generation,
      JsonNode spec,
      Instant createdAt,
      Instant updatedAt) {}
}
