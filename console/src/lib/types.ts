export interface PolicyResponse {
  id: string;
  namespace: string;
  name: string;
  generation: number;
  spec: PolicySpec;
  createdAt: string;
  updatedAt: string;
}

export interface PolicySpec {
  host?: string;
  rules?: PolicyRule[];
  [key: string]: unknown;
}

export interface PolicyRule {
  path: string;
  backendService?: string;
  backendPort?: number;
  versions?: { name: string; weight: number }[];
  [key: string]: unknown;
}

export interface VersionWeight {
  version: string;
  weight: number;
}

export interface RuleWeights {
  path: string;
  weights: VersionWeight[];
}

export interface RouteConfigResponse {
  namespace: string;
  name: string;
  generation: number;
  rules: RuleWeights[];
}

export type CanaryStatus = "PROGRESSING" | "PROMOTED" | "ROLLED_BACK";

export interface CanaryResponse {
  namespace: string;
  name: string;
  path: string;
  status: CanaryStatus;
  stableVersion: string;
  canaryVersion: string;
  steps: number[];
  stepIndex: number;
  currentCanaryWeight: number;
  startedAt: string;
  updatedAt: string;
}

export interface CanaryStartRequest {
  stableVersion: string;
  canaryVersion: string;
  steps?: number[];
}

export interface AuditRecord {
  id: number;
  actor: string;
  action: string;
  target: string;
  detail: unknown;
  createdAt: string;
}

export type CircuitState = "CLOSED" | "OPEN" | "HALF_OPEN";

export interface CircuitBreakerRecord {
  state: CircuitState;
  consecutiveFailures: number;
  openedAt: string | null;
}

export interface RateLimitResult {
  allowed: boolean;
  count: number;
  limit: number;
  remaining: number;
}
