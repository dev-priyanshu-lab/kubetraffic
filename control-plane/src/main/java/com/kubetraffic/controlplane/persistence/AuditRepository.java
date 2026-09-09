/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.persistence;

import java.util.List;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;

public interface AuditRepository extends JpaRepository<AuditEntity, Long> {

  List<AuditEntity> findAllByOrderByCreatedAtDesc(Pageable pageable);

  List<AuditEntity> findByTargetOrderByCreatedAtDesc(String target, Pageable pageable);
}
