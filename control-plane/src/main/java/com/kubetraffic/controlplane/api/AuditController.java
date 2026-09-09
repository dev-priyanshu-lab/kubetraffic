/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.api;

import com.kubetraffic.controlplane.audit.AuditRecord;
import com.kubetraffic.controlplane.audit.AuditService;
import java.util.List;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1/audit")
public class AuditController {

  private final AuditService service;

  public AuditController(AuditService service) {
    this.service = service;
  }

  @GetMapping
  public List<AuditRecord> query(
      @RequestParam(name = "target", required = false) String target,
      @RequestParam(name = "limit", defaultValue = "100") int limit) {
    return service.query(target, limit);
  }
}
