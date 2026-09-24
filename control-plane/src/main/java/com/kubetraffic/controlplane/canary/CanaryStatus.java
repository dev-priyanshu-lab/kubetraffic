/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.canary;

/** Lifecycle of one canary progression. */
public enum CanaryStatus {
  /** Walking the weight ladder; {@code stepIndex} names the current rung. */
  PROGRESSING,
  /** Reached 100% — the canary version has fully replaced the stable one. */
  PROMOTED,
  /** Reverted to 0% after a bad rollout. */
  ROLLED_BACK
}
