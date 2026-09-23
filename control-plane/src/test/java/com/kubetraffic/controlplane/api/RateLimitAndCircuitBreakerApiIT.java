/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.api;

import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.databind.JsonNode;
import com.kubetraffic.controlplane.AbstractRedisIT;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.client.TestRestTemplate;
import org.springframework.http.HttpStatus;

/** End-to-end against a real Redis: exercises the same counters two "replicas" would share. */
@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
class RateLimitAndCircuitBreakerApiIT extends AbstractRedisIT {

  @Autowired TestRestTemplate rest;

  @Test
  void rateLimitAllowsUpToLimitThenReturns429() {
    String key = "it-test-" + System.nanoTime();
    for (int i = 1; i <= 3; i++) {
      var resp =
          rest.postForEntity(
              "/api/v1/ratelimit/" + key + "/check?limit=3&windowSeconds=60", null, JsonNode.class);
      assertThat(resp.getStatusCode()).isEqualTo(HttpStatus.OK);
      assertThat(resp.getBody().get("count").asInt()).isEqualTo(i);
    }
    var denied =
        rest.postForEntity(
            "/api/v1/ratelimit/" + key + "/check?limit=3&windowSeconds=60", null, JsonNode.class);
    assertThat(denied.getStatusCode()).isEqualTo(HttpStatus.TOO_MANY_REQUESTS);
    assertThat(denied.getBody().get("allowed").asBoolean()).isFalse();
  }

  @Test
  void circuitBreakerOpensAndResets() {
    String key = "it-cb-" + System.nanoTime();
    JsonNode last = null;
    for (int i = 0; i < 5; i++) {
      last =
          rest.postForObject(
              "/api/v1/circuit-breaker/" + key + "/failure?threshold=5&recoverySeconds=30",
              null,
              JsonNode.class);
    }
    assertThat(last.get("state").asText()).isEqualTo("OPEN");

    var reset =
        rest.postForObject("/api/v1/circuit-breaker/" + key + "/success", null, JsonNode.class);
    assertThat(reset.get("state").asText()).isEqualTo("CLOSED");
    assertThat(reset.get("consecutiveFailures").asInt()).isZero();
  }
}
