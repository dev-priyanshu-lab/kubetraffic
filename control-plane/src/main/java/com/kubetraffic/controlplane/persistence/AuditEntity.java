/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.persistence;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import java.time.Instant;
import org.hibernate.annotations.CreationTimestamp;
import org.hibernate.annotations.JdbcTypeCode;
import org.hibernate.type.SqlTypes;

/** An append-only audit record. */
@Entity
@Table(name = "audit_log")
public class AuditEntity {

  @Id
  @GeneratedValue(strategy = GenerationType.IDENTITY)
  private Long id;

  @Column(nullable = false)
  private String actor;

  @Column(nullable = false)
  private String action;

  @Column(nullable = false)
  private String target;

  @JdbcTypeCode(SqlTypes.JSON)
  @Column(columnDefinition = "jsonb")
  private String detail;

  @CreationTimestamp
  @Column(name = "created_at", updatable = false)
  private Instant createdAt;

  protected AuditEntity() {}

  public AuditEntity(String actor, String action, String target, String detail) {
    this.actor = actor;
    this.action = action;
    this.target = target;
    this.detail = detail;
  }

  public Long getId() {
    return id;
  }

  public String getActor() {
    return actor;
  }

  public String getAction() {
    return action;
  }

  public String getTarget() {
    return target;
  }

  public String getDetail() {
    return detail;
  }

  public Instant getCreatedAt() {
    return createdAt;
  }
}
