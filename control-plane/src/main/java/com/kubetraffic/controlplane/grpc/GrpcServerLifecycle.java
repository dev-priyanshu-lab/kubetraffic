/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.grpc;

import io.grpc.Server;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import java.io.IOException;
import java.util.concurrent.TimeUnit;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.SmartLifecycle;
import org.springframework.stereotype.Component;

/** Starts and stops the gRPC server alongside the Spring application context. */
@Component
public class GrpcServerLifecycle implements SmartLifecycle {

  private static final Logger log = LoggerFactory.getLogger(GrpcServerLifecycle.class);

  private final int port;
  private final RouteGrpcService routeService;
  private final DecisionGrpcService decisionService;

  private Server server;
  private volatile boolean running;

  public GrpcServerLifecycle(
      @Value("${grpc.port:9090}") int port,
      RouteGrpcService routeService,
      DecisionGrpcService decisionService) {
    this.port = port;
    this.routeService = routeService;
    this.decisionService = decisionService;
  }

  @Override
  public void start() {
    try {
      server =
          NettyServerBuilder.forPort(port)
              .addService(routeService)
              .addService(decisionService)
              .build()
              .start();
      running = true;
      log.info("gRPC server listening on :{}", port);
    } catch (IOException e) {
      throw new IllegalStateException("failed to start gRPC server on port " + port, e);
    }
  }

  @Override
  public void stop() {
    if (server == null) {
      return;
    }
    server.shutdown();
    try {
      if (!server.awaitTermination(10, TimeUnit.SECONDS)) {
        server.shutdownNow();
      }
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
      server.shutdownNow();
    } finally {
      running = false;
    }
  }

  @Override
  public boolean isRunning() {
    return running;
  }

  @Override
  public int getPhase() {
    // Start after the web server/actuator infrastructure is up.
    return Integer.MAX_VALUE;
  }
}
