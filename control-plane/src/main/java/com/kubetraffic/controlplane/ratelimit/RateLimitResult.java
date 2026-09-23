/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.ratelimit;

/** The outcome of a single rate-limit check. */
public record RateLimitResult(boolean allowed, long count, long limit, long remaining) {}
