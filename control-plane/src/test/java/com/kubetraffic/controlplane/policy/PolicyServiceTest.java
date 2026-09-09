/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.policy;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.kubetraffic.controlplane.audit.AuditService;
import com.kubetraffic.controlplane.persistence.ConfigVersionEntity;
import com.kubetraffic.controlplane.persistence.ConfigVersionRepository;
import com.kubetraffic.controlplane.persistence.PolicyEntity;
import com.kubetraffic.controlplane.persistence.PolicyRepository;
import com.kubetraffic.controlplane.policy.PolicyDtos.CreateRequest;
import java.util.Optional;
import java.util.UUID;
import org.junit.jupiter.api.Test;

class PolicyServiceTest {

  private final PolicyRepository policies = mock(PolicyRepository.class);
  private final ConfigVersionRepository versions = mock(ConfigVersionRepository.class);
  private final AuditService audit = mock(AuditService.class);
  private final ObjectMapper mapper = new ObjectMapper();
  private final PolicyService service = new PolicyService(policies, versions, audit, mapper);

  private CreateRequest create(String ns, String name, String json) throws Exception {
    return new CreateRequest(ns, name, mapper.readTree(json));
  }

  @Test
  void create_persistsPolicyVersionAndAudit() throws Exception {
    when(policies.existsByNamespaceAndName("demo", "p")).thenReturn(false);

    var resp = service.create(create("demo", "p", "{\"host\":\"api.example.com\"}"));

    assertThat(resp.generation()).isEqualTo(1L);
    assertThat(resp.namespace()).isEqualTo("demo");
    verify(policies).save(any(PolicyEntity.class));
    verify(versions).save(any(ConfigVersionEntity.class));
    verify(audit).record(eq("api"), eq("policy.create"), eq("policy/demo/p"), anyString());
  }

  @Test
  void create_defaultsNamespace() throws Exception {
    when(policies.existsByNamespaceAndName("default", "p")).thenReturn(false);
    var resp = service.create(create(null, "p", "{}"));
    assertThat(resp.namespace()).isEqualTo("default");
  }

  @Test
  void create_conflictWhenExists() throws Exception {
    when(policies.existsByNamespaceAndName("demo", "p")).thenReturn(true);
    assertThatThrownBy(() -> service.create(create("demo", "p", "{}")))
        .isInstanceOf(ConflictException.class);
  }

  @Test
  void get_notFound() {
    when(policies.findByNamespaceAndName("demo", "missing")).thenReturn(Optional.empty());
    assertThatThrownBy(() -> service.get("demo", "missing")).isInstanceOf(NotFoundException.class);
  }

  @Test
  void update_bumpsGenerationWhenSpecChanges() throws Exception {
    var entity = new PolicyEntity(UUID.randomUUID(), "demo", "p", "{\"a\":1}", 1L);
    when(policies.findByNamespaceAndName("demo", "p")).thenReturn(Optional.of(entity));

    var resp = service.update("demo", "p", mapper.readTree("{\"a\":2}"));

    assertThat(resp.generation()).isEqualTo(2L);
    assertThat(entity.getGeneration()).isEqualTo(2L);
    verify(versions).save(any(ConfigVersionEntity.class));
    verify(audit).record(eq("api"), eq("policy.update"), eq("policy/demo/p"), anyString());
  }

  @Test
  void delete_removesPolicyAndAudits() {
    var entity = new PolicyEntity(UUID.randomUUID(), "demo", "p", "{}", 1L);
    when(policies.findByNamespaceAndName("demo", "p")).thenReturn(Optional.of(entity));

    service.delete("demo", "p");

    verify(policies).delete(entity);
    verify(audit).record(eq("api"), eq("policy.delete"), eq("policy/demo/p"), eq(null));
  }

  @Test
  void delete_notFound() {
    when(policies.findByNamespaceAndName("demo", "missing")).thenReturn(Optional.empty());
    assertThatThrownBy(() -> service.delete("demo", "missing")).isInstanceOf(NotFoundException.class);
  }

  @Test
  void update_noopWhenSpecUnchanged() throws Exception {
    var entity = new PolicyEntity(UUID.randomUUID(), "demo", "p", "{\"a\":1}", 3L);
    when(policies.findByNamespaceAndName("demo", "p")).thenReturn(Optional.of(entity));

    var resp = service.update("demo", "p", mapper.readTree("{ \"a\" : 1 }")); // same, reformatted

    assertThat(resp.generation()).isEqualTo(3L);
    verify(versions, never()).save(any(ConfigVersionEntity.class));
    verify(audit, never()).record(anyString(), anyString(), anyString(), anyString());
  }
}
