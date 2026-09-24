/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.grpc;

import com.kubetraffic.controlplane.grpc.v1.Decision;

/**
 * Publishes a {@link Decision} to every controller currently subscribed via {@code
 * StreamDecisions}. Kept as an interface so business services (route registration, canary
 * progression) depend on "publish a decision", not on the gRPC service implementation.
 */
public interface DecisionPublisher {

  void publish(Decision decision);
}
