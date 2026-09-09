/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.persistence;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import java.time.Instant;
import java.util.UUID;
import org.hibernate.annotations.CreationTimestamp;
import org.hibernate.annotations.JdbcTypeCode;
import org.hibernate.annotations.UpdateTimestamp;
import org.hibernate.type.SqlTypes;

/** A stored TrafficRoute policy, addressed by (namespace, name). */
@Entity
@Table(name = "policy")
public class PolicyEntity {

  @Id private UUID id;

  @Column(nullable = false)
  private String namespace;

  @Column(nullable = false)
  private String name;

  /** The policy spec as raw JSON. */
  @JdbcTypeCode(SqlTypes.JSON)
  @Column(columnDefinition = "jsonb", nullable = false)
  private String spec;

  @Column(nullable = false)
  private long generation;

  @CreationTimestamp
  @Column(name = "created_at", updatable = false)
  private Instant createdAt;

  @UpdateTimestamp
  @Column(name = "updated_at")
  private Instant updatedAt;

  protected PolicyEntity() {}

  public PolicyEntity(UUID id, String namespace, String name, String spec, long generation) {
    this.id = id;
    this.namespace = namespace;
    this.name = name;
    this.spec = spec;
    this.generation = generation;
  }

  public UUID getId() {
    return id;
  }

  public String getNamespace() {
    return namespace;
  }

  public String getName() {
    return name;
  }

  public String getSpec() {
    return spec;
  }

  public void setSpec(String spec) {
    this.spec = spec;
  }

  public long getGeneration() {
    return generation;
  }

  public void setGeneration(long generation) {
    this.generation = generation;
  }

  public Instant getCreatedAt() {
    return createdAt;
  }

  public Instant getUpdatedAt() {
    return updatedAt;
  }
}
