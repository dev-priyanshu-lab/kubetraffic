/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.api;

import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.databind.JsonNode;
import com.kubetraffic.controlplane.AbstractPostgresIT;
import java.util.Map;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.client.TestRestTemplate;
import org.springframework.http.HttpStatus;

/** End-to-end: create a policy, start a canary against it, walk it to promotion. */
@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
class CanaryApiIT extends AbstractPostgresIT {

  @Autowired TestRestTemplate rest;

  @Test
  void canaryLifecycle() {
    String name = "canary-it-" + System.nanoTime();
    var create =
        rest.postForEntity(
            "/api/v1/policies",
            Map.of("namespace", "demo", "name", name, "spec", Map.of("host", "api.example.com")),
            JsonNode.class);
    assertThat(create.getStatusCode()).isEqualTo(HttpStatus.CREATED);

    var start =
        rest.postForEntity(
            "/api/v1/canary/" + name + "/start?namespace=demo&path=/payment",
            Map.of("stableVersion", "v1", "canaryVersion", "v2", "steps", java.util.List.of(10, 100)),
            JsonNode.class);
    assertThat(start.getStatusCode()).isEqualTo(HttpStatus.OK);
    assertThat(start.getBody().get("status").asText()).isEqualTo("PROGRESSING");
    assertThat(start.getBody().get("currentCanaryWeight").asInt()).isEqualTo(10);

    var promoted =
        rest.postForObject(
            "/api/v1/canary/" + name + "/promote?namespace=demo&path=/payment", null, JsonNode.class);
    assertThat(promoted.get("status").asText()).isEqualTo("PROMOTED");
    assertThat(promoted.get("currentCanaryWeight").asInt()).isEqualTo(100);

    var get =
        rest.getForObject("/api/v1/canary/" + name + "?namespace=demo&path=/payment", JsonNode.class);
    assertThat(get.get("status").asText()).isEqualTo("PROMOTED");
  }

  @Test
  void rollbackReturnsToZero() {
    String name = "canary-rb-it-" + System.nanoTime();
    rest.postForEntity(
        "/api/v1/policies",
        Map.of("namespace", "demo", "name", name, "spec", Map.of("host", "api.example.com")),
        JsonNode.class);
    rest.postForEntity(
        "/api/v1/canary/" + name + "/start?namespace=demo&path=/",
        Map.of("stableVersion", "v1", "canaryVersion", "v2"),
        JsonNode.class);

    var rolledBack =
        rest.postForObject("/api/v1/canary/" + name + "/rollback?namespace=demo&path=/", null, JsonNode.class);
    assertThat(rolledBack.get("status").asText()).isEqualTo("ROLLED_BACK");
    assertThat(rolledBack.get("currentCanaryWeight").asInt()).isZero();
  }
}
