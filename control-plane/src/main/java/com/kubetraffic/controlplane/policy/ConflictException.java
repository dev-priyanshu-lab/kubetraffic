/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.policy;

/** Thrown when a create would violate uniqueness. Mapped to HTTP 409. */
public class ConflictException extends RuntimeException {
  public ConflictException(String message) {
    super(message);
  }
}
