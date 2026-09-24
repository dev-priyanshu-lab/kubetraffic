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
import org.hibernate.annotations.UpdateTimestamp;
import org.hibernate.type.SqlTypes;

/** Canary progression state for one (policy, path). */
@Entity
@Table(name = "canary_state")
public class CanaryEntity {

  @Id
  @GeneratedValue(strategy = GenerationType.IDENTITY)
  private Long id;

  @Column(name = "policy_id", nullable = false)
  private UUID policyId;

  @Column(nullable = false)
  private String path;

  @Column(name = "stable_version", nullable = false)
  private String stableVersion;

  @Column(name = "canary_version", nullable = false)
  private String canaryVersion;

  /** JSON array of ascending weight percentages, e.g. "[5,10,20,30,50,100]". */
  @JdbcTypeCode(SqlTypes.JSON)
  @Column(columnDefinition = "jsonb", nullable = false)
  private String steps;

  @Column(name = "step_index", nullable = false)
  private int stepIndex;

  /** Name of {@link com.kubetraffic.controlplane.canary.CanaryStatus}. */
  @Column(nullable = false)
  private String status;

  @CreationTimestamp
  @Column(name = "started_at", updatable = false)
  private Instant startedAt;

  @UpdateTimestamp
  @Column(name = "updated_at")
  private Instant updatedAt;

  protected CanaryEntity() {}

  public CanaryEntity(
      UUID policyId, String path, String stableVersion, String canaryVersion, String steps, String status) {
    this.policyId = policyId;
    this.path = path;
    this.stableVersion = stableVersion;
    this.canaryVersion = canaryVersion;
    this.steps = steps;
    this.stepIndex = 0;
    this.status = status;
  }

  /** Re-initializes an existing row for a fresh `start` (upsert semantics). */
  public void reinitialize(String stableVersion, String canaryVersion, String steps, String status) {
    this.stableVersion = stableVersion;
    this.canaryVersion = canaryVersion;
    this.steps = steps;
    this.stepIndex = 0;
    this.status = status;
  }

  public Long getId() {
    return id;
  }

  public UUID getPolicyId() {
    return policyId;
  }

  public String getPath() {
    return path;
  }

  public String getStableVersion() {
    return stableVersion;
  }

  public String getCanaryVersion() {
    return canaryVersion;
  }

  public String getSteps() {
    return steps;
  }

  public int getStepIndex() {
    return stepIndex;
  }

  public void setStepIndex(int stepIndex) {
    this.stepIndex = stepIndex;
  }

  public String getStatus() {
    return status;
  }

  public void setStatus(String status) {
    this.status = status;
  }

  public Instant getStartedAt() {
    return startedAt;
  }

  public Instant getUpdatedAt() {
    return updatedAt;
  }
}
