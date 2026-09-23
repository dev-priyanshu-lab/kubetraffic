/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.api;

import com.kubetraffic.controlplane.ratelimit.RateLimitResult;
import com.kubetraffic.controlplane.ratelimit.RateLimiterService;
import java.time.Duration;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

/**
 * Distributed rate-limit checks, backed by {@link RateLimiterService}. {@code key} is a caller-
 * chosen bucket id (use {@code :} as a separator, e.g. {@code demo:payment-route}) — Phase 13
 * wires this to real route/version keys read from {@code spec.security.rateLimit}.
 */
@RestController
@RequestMapping("/api/v1/ratelimit")
public class RateLimitController {

  private final RateLimiterService service;

  public RateLimitController(RateLimiterService service) {
    this.service = service;
  }

  @PostMapping("/{key}/check")
  public ResponseEntity<RateLimitResult> check(
      @PathVariable String key,
      @RequestParam(defaultValue = "100") long limit,
      @RequestParam(name = "windowSeconds", defaultValue = "60") long windowSeconds) {
    RateLimitResult result = service.tryAcquire(key, limit, Duration.ofSeconds(windowSeconds));
    return ResponseEntity.status(result.allowed() ? HttpStatus.OK : HttpStatus.TOO_MANY_REQUESTS)
        .body(result);
  }
}
