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
import java.util.UUID;
import org.hibernate.annotations.CreationTimestamp;
import org.hibernate.annotations.JdbcTypeCode;
import org.hibernate.type.SqlTypes;

/** An immutable snapshot of a policy spec at a given generation. */
@Entity
@Table(name = "config_version")
public class ConfigVersionEntity {

  @Id
  @GeneratedValue(strategy = GenerationType.IDENTITY)
  private Long id;

  @Column(name = "policy_id", nullable = false)
  private UUID policyId;

  @Column(nullable = false)
  private long version;

  @JdbcTypeCode(SqlTypes.JSON)
  @Column(columnDefinition = "jsonb", nullable = false)
  private String spec;

  @CreationTimestamp
  @Column(name = "created_at", updatable = false)
  private Instant createdAt;

  protected ConfigVersionEntity() {}

  public ConfigVersionEntity(UUID policyId, long version, String spec) {
    this.policyId = policyId;
    this.version = version;
    this.spec = spec;
  }

  public Long getId() {
    return id;
  }

  public UUID getPolicyId() {
    return policyId;
  }

  public long getVersion() {
    return version;
  }

  public String getSpec() {
    return spec;
  }

  public Instant getCreatedAt() {
    return createdAt;
  }
}
