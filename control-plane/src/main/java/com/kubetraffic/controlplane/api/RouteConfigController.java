/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.api;

import com.kubetraffic.controlplane.policy.NotFoundException;
import com.kubetraffic.controlplane.routing.RouteConfigDtos.Response;
import com.kubetraffic.controlplane.routing.RouteConfigService;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

/**
 * The live, canary-aware weights for a route — the same computation the controller consumes over
 * gRPC, exposed as JSON for the console (and anyone else who'd rather not speak gRPC).
 */
@RestController
@RequestMapping("/api/v1/policies")
public class RouteConfigController {

  private final RouteConfigService service;

  public RouteConfigController(RouteConfigService service) {
    this.service = service;
  }

  @GetMapping("/{name}/route-config")
  public Response routeConfig(
      @PathVariable String name, @RequestParam(required = false) String namespace) {
    return service
        .get(namespace, name)
        .orElseThrow(
            () -> new NotFoundException("policy %s/%s not found".formatted(namespace == null ? "default" : namespace, name)));
  }
}
