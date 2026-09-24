/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.api;

import com.kubetraffic.controlplane.canary.CanaryDtos.Response;
import com.kubetraffic.controlplane.canary.CanaryDtos.StartRequest;
import com.kubetraffic.controlplane.canary.CanaryService;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

/**
 * Canary progression. {@code path} defaults to "/" (the common single-rule-route case); routes
 * with multiple rules pass {@code ?path=} to address a specific one.
 */
@RestController
@RequestMapping("/api/v1/canary")
public class CanaryController {

  private final CanaryService service;

  public CanaryController(CanaryService service) {
    this.service = service;
  }

  @PostMapping("/{name}/start")
  public Response start(
      @PathVariable String name,
      @RequestParam(required = false) String namespace,
      @RequestParam(defaultValue = "/") String path,
      @Valid @RequestBody StartRequest request) {
    return service.start(namespace, name, path, request.stableVersion(), request.canaryVersion(), request.steps());
  }

  @PostMapping("/{name}/promote")
  public Response promote(
      @PathVariable String name,
      @RequestParam(required = false) String namespace,
      @RequestParam(defaultValue = "/") String path) {
    return service.promote(namespace, name, path);
  }

  @PostMapping("/{name}/rollback")
  public Response rollback(
      @PathVariable String name,
      @RequestParam(required = false) String namespace,
      @RequestParam(defaultValue = "/") String path) {
    return service.rollback(namespace, name, path);
  }

  @GetMapping("/{name}")
  public Response get(
      @PathVariable String name,
      @RequestParam(required = false) String namespace,
      @RequestParam(defaultValue = "/") String path) {
    return service.get(namespace, name, path);
  }
}
