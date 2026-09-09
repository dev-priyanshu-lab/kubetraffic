/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.audit;

import com.fasterxml.jackson.databind.JsonNode;
import java.time.Instant;

/** A single audit-log entry as returned by the API. */
public record AuditRecord(
    long id, String actor, String action, String target, JsonNode detail, Instant createdAt) {}
