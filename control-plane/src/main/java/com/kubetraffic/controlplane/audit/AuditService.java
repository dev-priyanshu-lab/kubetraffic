/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.audit;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.kubetraffic.controlplane.persistence.AuditEntity;
import com.kubetraffic.controlplane.persistence.AuditRepository;
import java.util.List;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.data.domain.PageRequest;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/** Append-only audit trail for every mutating control-plane action. */
@Service
public class AuditService {

  private static final Logger log = LoggerFactory.getLogger(AuditService.class);
  private static final int MAX_LIMIT = 500;

  private final AuditRepository repository;
  private final ObjectMapper mapper;

  public AuditService(AuditRepository repository, ObjectMapper mapper) {
    this.repository = repository;
    this.mapper = mapper;
  }

  @Transactional
  public void record(String actor, String action, String target, String detailJson) {
    repository.save(new AuditEntity(actor, action, target, detailJson));
    log.info("audit actor={} action={} target={}", actor, action, target);
  }

  @Transactional(readOnly = true)
  public List<AuditRecord> query(String target, int limit) {
    int capped = Math.min(Math.max(limit, 1), MAX_LIMIT);
    var page = PageRequest.of(0, capped);
    var rows =
        (target == null || target.isBlank())
            ? repository.findAllByOrderByCreatedAtDesc(page)
            : repository.findByTargetOrderByCreatedAtDesc(target, page);
    return rows.stream().map(this::toRecord).toList();
  }

  private AuditRecord toRecord(AuditEntity e) {
    return new AuditRecord(
        e.getId(), e.getActor(), e.getAction(), e.getTarget(), parse(e.getDetail()), e.getCreatedAt());
  }

  private JsonNode parse(String json) {
    if (json == null) {
      return null;
    }
    try {
      return mapper.readTree(json);
    } catch (Exception ex) {
      return null;
    }
  }
}
