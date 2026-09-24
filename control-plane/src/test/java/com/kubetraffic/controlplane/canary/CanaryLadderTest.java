/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.canary;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.util.List;
import org.junit.jupiter.api.Test;

class CanaryLadderTest {

  @Test
  void nullOrEmptyReturnsDefault() {
    assertThat(CanaryLadder.validate(null)).isEqualTo(CanaryLadder.DEFAULT_STEPS);
    assertThat(CanaryLadder.validate(List.of())).isEqualTo(CanaryLadder.DEFAULT_STEPS);
  }

  @Test
  void acceptsAscendingStepsEndingAt100() {
    var steps = List.of(10, 50, 100);
    assertThat(CanaryLadder.validate(steps)).isEqualTo(steps);
  }

  @Test
  void rejectsNonAscending() {
    assertThatThrownBy(() -> CanaryLadder.validate(List.of(10, 10, 100)))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("ascending");
  }

  @Test
  void rejectsNotEndingAt100() {
    assertThatThrownBy(() -> CanaryLadder.validate(List.of(10, 50)))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("end at 100");
  }

  @Test
  void rejectsOutOfRange() {
    assertThatThrownBy(() -> CanaryLadder.validate(List.of(0, 100)))
        .isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> CanaryLadder.validate(List.of(50, 150)))
        .isInstanceOf(IllegalArgumentException.class);
  }
}
