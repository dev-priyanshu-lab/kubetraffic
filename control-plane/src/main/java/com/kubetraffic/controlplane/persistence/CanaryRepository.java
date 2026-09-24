/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.persistence;

import java.util.Optional;
import java.util.UUID;
import org.springframework.data.jpa.repository.JpaRepository;

public interface CanaryRepository extends JpaRepository<CanaryEntity, Long> {

  Optional<CanaryEntity> findByPolicyIdAndPath(UUID policyId, String path);
}
