/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.policy;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.kubetraffic.controlplane.audit.AuditService;
import com.kubetraffic.controlplane.persistence.ConfigVersionEntity;
import com.kubetraffic.controlplane.persistence.ConfigVersionRepository;
import com.kubetraffic.controlplane.persistence.PolicyEntity;
import com.kubetraffic.controlplane.persistence.PolicyRepository;
import com.kubetraffic.controlplane.policy.PolicyDtos.CreateRequest;
import com.kubetraffic.controlplane.policy.PolicyDtos.Response;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * Owns policy CRUD, config versioning and the audit hook. Every mutation records
 * a new {@link ConfigVersionEntity} and an audit entry in the same transaction.
 */
@Service
public class PolicyService {

  private static final String DEFAULT_NAMESPACE = "default";

  private final PolicyRepository policies;
  private final ConfigVersionRepository versions;
  private final AuditService audit;
  private final ObjectMapper mapper;

  public PolicyService(
      PolicyRepository policies,
      ConfigVersionRepository versions,
      AuditService audit,
      ObjectMapper mapper) {
    this.policies = policies;
    this.versions = versions;
    this.audit = audit;
    this.mapper = mapper;
  }

  @Transactional
  public Response create(CreateRequest req) {
    String ns = namespaceOrDefault(req.namespace());
    if (policies.existsByNamespaceAndName(ns, req.name())) {
      throw new ConflictException("policy %s/%s already exists".formatted(ns, req.name()));
    }
    String spec = toJson(req.spec());
    PolicyEntity entity = new PolicyEntity(UUID.randomUUID(), ns, req.name(), spec, 1L);
    policies.save(entity);
    versions.save(new ConfigVersionEntity(entity.getId(), 1L, spec));
    audit.record("api", "policy.create", target(ns, req.name()), spec);
    return toResponse(entity);
  }

  @Transactional(readOnly = true)
  public Response get(String namespace, String name) {
    return toResponse(require(namespaceOrDefault(namespace), name));
  }

  @Transactional(readOnly = true)
  public List<Response> list() {
    return policies.findAll().stream().map(this::toResponse).toList();
  }

  /**
   * Create-or-update: used by the gRPC RegisterRoute RPC, which is always an
   * idempotent upsert (the caller doesn't know whether the route already
   * exists on the control plane).
   */
  @Transactional
  public Response upsert(String namespace, String name, JsonNode spec) {
    String ns = namespaceOrDefault(namespace);
    if (policies.existsByNamespaceAndName(ns, name)) {
      return update(ns, name, spec);
    }
    return create(new CreateRequest(ns, name, spec));
  }

  @Transactional
  public Response update(String namespace, String name, JsonNode newSpec) {
    String ns = namespaceOrDefault(namespace);
    PolicyEntity entity = require(ns, name);

    JsonNode current = readTree(entity.getSpec());
    if (!current.equals(newSpec)) {
      String spec = toJson(newSpec);
      entity.setSpec(spec);
      entity.setGeneration(entity.getGeneration() + 1);
      policies.save(entity);
      versions.save(new ConfigVersionEntity(entity.getId(), entity.getGeneration(), spec));
      audit.record("api", "policy.update", target(ns, name), spec);
    }
    return toResponse(entity);
  }

  @Transactional
  public void delete(String namespace, String name) {
    String ns = namespaceOrDefault(namespace);
    PolicyEntity entity = require(ns, name);
    policies.delete(entity); // config_version rows cascade in the DB
    audit.record("api", "policy.delete", target(ns, name), null);
  }

  /** Non-throwing lookup, for callers (e.g. gRPC) that treat "absent" as data, not an error. */
  @Transactional(readOnly = true)
  public Optional<Response> tryGet(String namespace, String name) {
    return policies.findByNamespaceAndName(namespaceOrDefault(namespace), name).map(this::toResponse);
  }

  /** Idempotent delete: a no-op if the policy does not exist. */
  @Transactional
  public void deleteIfExists(String namespace, String name) {
    String ns = namespaceOrDefault(namespace);
    policies
        .findByNamespaceAndName(ns, name)
        .ifPresent(
            entity -> {
              policies.delete(entity);
              audit.record("api", "policy.delete", target(ns, name), null);
            });
  }

  @Transactional(readOnly = true)
  public long versionCount(UUID policyId) {
    return versions.countByPolicyId(policyId);
  }

  private PolicyEntity require(String namespace, String name) {
    return policies
        .findByNamespaceAndName(namespace, name)
        .orElseThrow(
            () -> new NotFoundException("policy %s/%s not found".formatted(namespace, name)));
  }

  private Response toResponse(PolicyEntity e) {
    return new Response(
        e.getId(),
        e.getNamespace(),
        e.getName(),
        e.getGeneration(),
        readTree(e.getSpec()),
        e.getCreatedAt(),
        e.getUpdatedAt());
  }

  private static String namespaceOrDefault(String ns) {
    return (ns == null || ns.isBlank()) ? DEFAULT_NAMESPACE : ns;
  }

  private static String target(String ns, String name) {
    return "policy/" + ns + "/" + name;
  }

  private String toJson(JsonNode node) {
    try {
      return mapper.writeValueAsString(node);
    } catch (Exception e) {
      throw new IllegalArgumentException("spec is not serializable JSON", e);
    }
  }

  private JsonNode readTree(String json) {
    try {
      return mapper.readTree(json);
    } catch (Exception e) {
      throw new IllegalStateException("stored spec is not valid JSON", e);
    }
  }
}
