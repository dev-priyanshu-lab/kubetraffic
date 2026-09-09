/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.api;

import com.kubetraffic.controlplane.policy.PolicyDtos.CreateRequest;
import com.kubetraffic.controlplane.policy.PolicyDtos.Response;
import com.kubetraffic.controlplane.policy.PolicyDtos.UpdateRequest;
import com.kubetraffic.controlplane.policy.PolicyService;
import jakarta.validation.Valid;
import java.net.URI;
import java.util.List;
import org.springframework.http.ResponseEntity;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.ResponseStatus;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1/policies")
public class PolicyController {

  private final PolicyService service;

  public PolicyController(PolicyService service) {
    this.service = service;
  }

  @PostMapping
  public ResponseEntity<Response> create(@Valid @RequestBody CreateRequest request) {
    Response created = service.create(request);
    return ResponseEntity.created(
            URI.create("/api/v1/policies/" + created.name() + "?namespace=" + created.namespace()))
        .body(created);
  }

  @GetMapping
  public List<Response> list() {
    return service.list();
  }

  @GetMapping("/{name}")
  public Response get(
      @PathVariable String name,
      @RequestParam(name = "namespace", required = false) String namespace) {
    return service.get(namespace, name);
  }

  @PutMapping("/{name}")
  public Response update(
      @PathVariable String name,
      @RequestParam(name = "namespace", required = false) String namespace,
      @Valid @RequestBody UpdateRequest request) {
    String ns = request.namespace() != null ? request.namespace() : namespace;
    return service.update(ns, name, request.spec());
  }

  @DeleteMapping("/{name}")
  @ResponseStatus(HttpStatus.NO_CONTENT)
  public void delete(
      @PathVariable String name,
      @RequestParam(name = "namespace", required = false) String namespace) {
    service.delete(namespace, name);
  }
}
