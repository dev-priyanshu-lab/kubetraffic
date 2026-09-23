/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane;

import org.springframework.boot.testcontainers.service.connection.ServiceConnection;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;

/**
 * Base class for integration tests that need a real Redis, on top of {@link AbstractPostgresIT}'s
 * PostgreSQL. Like the rest of this module's ITs, this needs a Docker daemon whose API version the
 * bundled docker-java understands — see the note in {@code AbstractPostgresIT}'s neighbours; on
 * this dev machine that's exercised via {@code make control-plane-verify-full}, not the default
 * {@code mvn test}.
 */
@Testcontainers
public abstract class AbstractRedisIT extends AbstractPostgresIT {

  @Container
  @ServiceConnection("redis")
  static final GenericContainer<?> REDIS =
      new GenericContainer<>(DockerImageName.parse("redis:7-alpine")).withExposedPorts(6379);
}
