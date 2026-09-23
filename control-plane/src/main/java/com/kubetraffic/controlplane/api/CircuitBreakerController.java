/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.api;

import com.kubetraffic.controlplane.circuitbreaker.CircuitBreakerRecord;
import com.kubetraffic.controlplane.circuitbreaker.CircuitBreakerService;
import java.time.Duration;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

/**
 * Shared CLOSED/OPEN/HALF_OPEN circuit-breaker state, backed by {@link CircuitBreakerService}.
 * {@code key} is a caller-chosen circuit id (use {@code :} as a separator, e.g.
 * {@code demo:payment-route:v2}) — Phase 13 wires this to real health signals.
 */
@RestController
@RequestMapping("/api/v1/circuit-breaker")
public class CircuitBreakerController {

  private final CircuitBreakerService service;

  public CircuitBreakerController(CircuitBreakerService service) {
    this.service = service;
  }

  @GetMapping("/{key}")
  public CircuitBreakerRecord get(
      @PathVariable String key,
      @RequestParam(name = "recoverySeconds", defaultValue = "30") long recoverySeconds) {
    return service.get(key, Duration.ofSeconds(recoverySeconds));
  }

  @PostMapping("/{key}/failure")
  public CircuitBreakerRecord recordFailure(
      @PathVariable String key,
      @RequestParam(defaultValue = "5") int threshold,
      @RequestParam(name = "recoverySeconds", defaultValue = "30") long recoverySeconds) {
    return service.recordFailure(key, threshold, Duration.ofSeconds(recoverySeconds));
  }

  @PostMapping("/{key}/success")
  public CircuitBreakerRecord recordSuccess(@PathVariable String key) {
    return service.recordSuccess(key);
  }
}
