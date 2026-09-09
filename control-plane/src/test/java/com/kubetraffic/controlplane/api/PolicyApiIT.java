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

@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
class PolicyApiIT extends AbstractPostgresIT {

  @Autowired TestRestTemplate rest;

  private static Map<String, Object> body(String ns, String name, Object spec) {
    return Map.of("namespace", ns, "name", name, "spec", spec);
  }

  @Test
  void fullPolicyLifecycle() {
    var create =
        rest.postForEntity(
            "/api/v1/policies",
            body("demo", "payment-route", Map.of("host", "api.example.com", "weight", 90)),
            JsonNode.class);
    assertThat(create.getStatusCode()).isEqualTo(HttpStatus.CREATED);
    assertThat(create.getBody().get("generation").asLong()).isEqualTo(1L);
    assertThat(create.getBody().get("spec").get("host").asText()).isEqualTo("api.example.com");

    // duplicate -> 409
    var dup =
        rest.postForEntity(
            "/api/v1/policies", body("demo", "payment-route", Map.of("host", "x")), JsonNode.class);
    assertThat(dup.getStatusCode()).isEqualTo(HttpStatus.CONFLICT);

    // get -> 200
    var get =
        rest.getForEntity(
            "/api/v1/policies/payment-route?namespace=demo", JsonNode.class);
    assertThat(get.getStatusCode()).isEqualTo(HttpStatus.OK);
    assertThat(get.getBody().get("spec").get("weight").asInt()).isEqualTo(90);

    // put with changed spec -> generation 2
    var put =
        rest.exchange(
            "/api/v1/policies/payment-route?namespace=demo",
            org.springframework.http.HttpMethod.PUT,
            new org.springframework.http.HttpEntity<>(Map.of("spec", Map.of("host", "api.example.com", "weight", 50))),
            JsonNode.class);
    assertThat(put.getStatusCode()).isEqualTo(HttpStatus.OK);
    assertThat(put.getBody().get("generation").asLong()).isEqualTo(2L);

    // audit has both actions for this target
    var audit =
        rest.getForEntity("/api/v1/audit?target=policy/demo/payment-route", JsonNode.class);
    assertThat(audit.getStatusCode()).isEqualTo(HttpStatus.OK);
    assertThat(audit.getBody().size()).isGreaterThanOrEqualTo(2);

    // delete -> 204, then GET -> 404
    var del =
        rest.exchange(
            "/api/v1/policies/payment-route?namespace=demo",
            org.springframework.http.HttpMethod.DELETE,
            null,
            Void.class);
    assertThat(del.getStatusCode()).isEqualTo(HttpStatus.NO_CONTENT);
    assertThat(
            rest.getForEntity("/api/v1/policies/payment-route?namespace=demo", JsonNode.class)
                .getStatusCode())
        .isEqualTo(HttpStatus.NOT_FOUND);
  }

  @Test
  void get_missing_returns404() {
    var resp = rest.getForEntity("/api/v1/policies/nope?namespace=demo", JsonNode.class);
    assertThat(resp.getStatusCode()).isEqualTo(HttpStatus.NOT_FOUND);
  }

  @Test
  void create_invalidBody_returns400() {
    var resp =
        rest.postForEntity(
            "/api/v1/policies", Map.of("namespace", "demo", "spec", Map.of()), JsonNode.class);
    assertThat(resp.getStatusCode()).isEqualTo(HttpStatus.BAD_REQUEST);
  }

  @Test
  void actuatorHealthIsUp() {
    var resp = rest.getForEntity("/actuator/health", JsonNode.class);
    assertThat(resp.getStatusCode()).isEqualTo(HttpStatus.OK);
    assertThat(resp.getBody().get("status").asText()).isEqualTo("UP");
  }
}
