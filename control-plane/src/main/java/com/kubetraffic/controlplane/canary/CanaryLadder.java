/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.canary;

import java.util.List;

/** The canary weight ladder: pure validation, independent of storage. */
public final class CanaryLadder {

  /** The default progression named throughout the architecture docs. */
  public static final List<Integer> DEFAULT_STEPS = List.of(5, 10, 20, 30, 50, 100);

  private CanaryLadder() {}

  /**
   * Returns steps if valid (strictly ascending, each in (0,100], ending at 100), or {@link
   * #DEFAULT_STEPS} if steps is null/empty. Throws {@link IllegalArgumentException} otherwise.
   */
  public static List<Integer> validate(List<Integer> steps) {
    if (steps == null || steps.isEmpty()) {
      return DEFAULT_STEPS;
    }
    int prev = 0;
    for (int step : steps) {
      if (step <= prev || step > 100) {
        throw new IllegalArgumentException(
            "canary steps must be strictly ascending and between 1 and 100, got " + steps);
      }
      prev = step;
    }
    if (prev != 100) {
      throw new IllegalArgumentException("canary steps must end at 100, got " + steps);
    }
    return steps;
  }
}
