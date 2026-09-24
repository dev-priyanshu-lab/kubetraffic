import { apiBaseUrl } from "./config";
import type {
  AuditRecord,
  CanaryResponse,
  CanaryStartRequest,
  CircuitBreakerRecord,
  PolicyResponse,
  RateLimitResult,
  RouteConfigResponse,
} from "./types";

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${apiBaseUrl()}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!res.ok) {
    let detail = res.statusText;
    try {
      const problem = await res.json();
      detail = problem.detail ?? problem.title ?? detail;
    } catch {
      // response body wasn't JSON — fall back to statusText
    }
    throw new ApiError(res.status, detail);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json();
}

function qs(params: Record<string, string | number | undefined>): string {
  const entries = Object.entries(params).filter(([, v]) => v !== undefined && v !== "");
  if (entries.length === 0) return "";
  return "?" + new URLSearchParams(entries as [string, string][]).toString();
}

export const api = {
  policies: {
    list: () => request<PolicyResponse[]>("/api/v1/policies"),
    get: (name: string, namespace?: string) =>
      request<PolicyResponse>(`/api/v1/policies/${encodeURIComponent(name)}${qs({ namespace })}`),
    routeConfig: (name: string, namespace?: string) =>
      request<RouteConfigResponse>(
        `/api/v1/policies/${encodeURIComponent(name)}/route-config${qs({ namespace })}`,
      ),
    delete: (name: string, namespace?: string) =>
      request<void>(`/api/v1/policies/${encodeURIComponent(name)}${qs({ namespace })}`, {
        method: "DELETE",
      }),
  },
  canary: {
    get: (name: string, namespace?: string, path = "/") =>
      request<CanaryResponse>(`/api/v1/canary/${encodeURIComponent(name)}${qs({ namespace, path })}`),
    start: (name: string, body: CanaryStartRequest, namespace?: string, path = "/") =>
      request<CanaryResponse>(`/api/v1/canary/${encodeURIComponent(name)}/start${qs({ namespace, path })}`, {
        method: "POST",
        body: JSON.stringify(body),
      }),
    promote: (name: string, namespace?: string, path = "/") =>
      request<CanaryResponse>(`/api/v1/canary/${encodeURIComponent(name)}/promote${qs({ namespace, path })}`, {
        method: "POST",
      }),
    rollback: (name: string, namespace?: string, path = "/") =>
      request<CanaryResponse>(`/api/v1/canary/${encodeURIComponent(name)}/rollback${qs({ namespace, path })}`, {
        method: "POST",
      }),
  },
  audit: {
    query: (target?: string, limit = 100) =>
      request<AuditRecord[]>(`/api/v1/audit${qs({ target, limit })}`),
  },
  rateLimit: {
    check: (key: string, limit = 100, windowSeconds = 60) =>
      request<RateLimitResult>(
        `/api/v1/ratelimit/${encodeURIComponent(key)}/check${qs({ limit, windowSeconds })}`,
        { method: "POST" },
      ),
  },
  circuitBreaker: {
    get: (key: string, recoverySeconds = 30) =>
      request<CircuitBreakerRecord>(
        `/api/v1/circuit-breaker/${encodeURIComponent(key)}${qs({ recoverySeconds })}`,
      ),
    recordFailure: (key: string, threshold = 5, recoverySeconds = 30) =>
      request<CircuitBreakerRecord>(
        `/api/v1/circuit-breaker/${encodeURIComponent(key)}/failure${qs({ threshold, recoverySeconds })}`,
        { method: "POST" },
      ),
    recordSuccess: (key: string) =>
      request<CircuitBreakerRecord>(`/api/v1/circuit-breaker/${encodeURIComponent(key)}/success`, {
        method: "POST",
      }),
  },
};
