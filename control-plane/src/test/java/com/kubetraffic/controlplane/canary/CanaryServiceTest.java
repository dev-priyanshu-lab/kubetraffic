/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.canary;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.kubetraffic.controlplane.audit.AuditService;
import com.kubetraffic.controlplane.grpc.DecisionPublisher;
import com.kubetraffic.controlplane.grpc.v1.Decision;
import com.kubetraffic.controlplane.persistence.CanaryEntity;
import com.kubetraffic.controlplane.persistence.CanaryRepository;
import com.kubetraffic.controlplane.policy.NotFoundException;
import com.kubetraffic.controlplane.policy.PolicyDtos;
import com.kubetraffic.controlplane.policy.PolicyService;
import java.time.Instant;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class CanaryServiceTest {

  private final CanaryRepository canaries = mock(CanaryRepository.class);
  private final PolicyService policies = mock(PolicyService.class);
  private final AuditService audit = mock(AuditService.class);
  private final DecisionPublisher decisions = mock(DecisionPublisher.class);
  private final ObjectMapper mapper = new ObjectMapper();
  private final CanaryService service = new CanaryService(canaries, policies, audit, decisions, mapper);

  private final UUID policyId = UUID.randomUUID();
  private final AtomicReference<CanaryEntity> stored = new AtomicReference<>();

  @BeforeEach
  void wireStatefulRepository() {
    // findByPolicyIdAndPath/save behave like a real single-row table, so a
    // start-then-promote-then-rollback sequence in one test carries state
    // exactly like it would against Postgres.
    when(canaries.save(any())).thenAnswer(inv -> {
      stored.set(inv.getArgument(0));
      return stored.get();
    });
    when(canaries.findByPolicyIdAndPath(any(), any())).thenAnswer(inv -> Optional.ofNullable(stored.get()));

    when(policies.tryGet(anyString(), eq("payment-route")))
        .thenReturn(
            Optional.of(
                new PolicyDtos.Response(
                    policyId, "demo", "payment-route", 1L, mapper.createObjectNode(), Instant.now(), Instant.now())));
  }

  @Test
  void start_beginsProgressingAtFirstStep() {
    var resp = service.start("demo", "payment-route", "/payment", "v1", "v2", null);

    assertThat(resp.status()).isEqualTo(CanaryStatus.PROGRESSING);
    assertThat(resp.stepIndex()).isZero();
    assertThat(resp.currentCanaryWeight()).isEqualTo(5); // CanaryLadder.DEFAULT_STEPS[0]
    assertThat(resp.steps()).isEqualTo(CanaryLadder.DEFAULT_STEPS);

    verify(audit).record(eq("api"), eq("canary.start"), anyString(), anyString());
    verify(decisions).publish(any(Decision.class));
  }

  @Test
  void promote_walksTheLadderThenPromotes() {
    service.start("demo", "payment-route", "/payment", "v1", "v2", null);

    var afterFirst = service.promote("demo", "payment-route", "/payment");
    assertThat(afterFirst.currentCanaryWeight()).isEqualTo(10);
    assertThat(afterFirst.status()).isEqualTo(CanaryStatus.PROGRESSING);

    // walk the rest of the default ladder: 20, 30, 50, 100
    service.promote("demo", "payment-route", "/payment");
    service.promote("demo", "payment-route", "/payment");
    service.promote("demo", "payment-route", "/payment");
    var atEnd = service.promote("demo", "payment-route", "/payment");

    assertThat(atEnd.currentCanaryWeight()).isEqualTo(100);
    assertThat(atEnd.status()).isEqualTo(CanaryStatus.PROMOTED);

    var again = service.promote("demo", "payment-route", "/payment");
    assertThat(again.status()).isEqualTo(CanaryStatus.PROMOTED);
    assertThat(again.currentCanaryWeight()).isEqualTo(100); // idempotent at the terminal state
  }

  @Test
  void rollback_zeroesTheCanaryImmediately() {
    service.start("demo", "payment-route", "/payment", "v1", "v2", null);
    service.promote("demo", "payment-route", "/payment"); // now at 10%

    var rolledBack = service.rollback("demo", "payment-route", "/payment");

    assertThat(rolledBack.status()).isEqualTo(CanaryStatus.ROLLED_BACK);
    assertThat(rolledBack.currentCanaryWeight()).isZero();
    verify(audit).record(eq("api"), eq("canary.rollback"), anyString(), anyString());
  }

  @Test
  void effectiveWeights_reflectProgressingState() {
    service.start("demo", "payment-route", "/payment", "v1", "v2", null);

    var weights = service.effectiveWeights(policyId, "/payment").orElseThrow();
    assertThat(weights).containsEntry("v2", 5).containsEntry("v1", 95);
  }

  @Test
  void effectiveWeights_emptyWhenNoCanaryActive() {
    assertThat(service.effectiveWeights(policyId, "/payment")).isEmpty();
  }

  @Test
  void promote_withoutActiveCanary_throwsNotFound() {
    assertThatThrownBy(() -> service.promote("demo", "payment-route", "/payment"))
        .isInstanceOf(NotFoundException.class);
  }

  @Test
  void start_withUnknownPolicy_throwsNotFound() {
    when(policies.tryGet(anyString(), eq("missing"))).thenReturn(Optional.empty());
    assertThatThrownBy(() -> service.start("demo", "missing", "/", "v1", "v2", null))
        .isInstanceOf(NotFoundException.class);
  }

  @Test
  void start_rejectsInvalidSteps() {
    assertThatThrownBy(
            () -> service.start("demo", "payment-route", "/payment", "v1", "v2", java.util.List.of(10, 5, 100)))
        .isInstanceOf(IllegalArgumentException.class);
  }

  @Test
  void decisionPublished_reflectsWeightTransition() {
    service.start("demo", "payment-route", "/payment", "v1", "v2", null);
    service.promote("demo", "payment-route", "/payment");

    var captor = org.mockito.ArgumentCaptor.forClass(Decision.class);
    verify(decisions, times(2)).publish(captor.capture());
    Decision promoteDecision = captor.getAllValues().get(1);
    assertThat(promoteDecision.getVersion()).isEqualTo("v2");
    assertThat(promoteDecision.getOldWeight()).isEqualTo(5);
    assertThat(promoteDecision.getNewWeight()).isEqualTo(10);
    assertThat(promoteDecision.getReason()).isEqualTo("canary-promote");
  }
}
