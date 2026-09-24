/*
 * Copyright 2026 The KubeTraffic Authors.
 * SPDX-License-Identifier: Apache-2.0
 */
package com.kubetraffic.controlplane.config;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Configuration;
import org.springframework.web.servlet.config.annotation.CorsRegistry;
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer;

/**
 * Lets the console (typically served from a different origin — a different port in dev, a
 * different Service/Ingress host in production) call the REST API directly. No cookies/credentials
 * are used, so a permissive default is safe; set {@code CORS_ALLOWED_ORIGINS} to lock it down.
 */
@Configuration
public class CorsConfig implements WebMvcConfigurer {

  @Value("${kubetraffic.cors.allowed-origins:*}")
  private String[] allowedOrigins;

  @Override
  public void addCorsMappings(CorsRegistry registry) {
    registry
        .addMapping("/api/**")
        .allowedOrigins(allowedOrigins)
        .allowedMethods("GET", "POST", "PUT", "DELETE", "OPTIONS")
        .allowedHeaders("*")
        .maxAge(3600);
  }
}
