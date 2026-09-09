/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ---------------------------------------------------------------------------
// Spec
// ---------------------------------------------------------------------------

// TrafficRouteSpec is the desired L7 routing configuration for a single host.
type TrafficRouteSpec struct {
	// Host is the HTTP authority (Host header / :authority) this route matches.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Host string `json:"host"`

	// VersionLabel is the pod label key used to distinguish backend versions
	// when a RouteRule does not set explicit selector labels.
	// +kubebuilder:default="app.kubernetes.io/version"
	// +optional
	VersionLabel string `json:"versionLabel,omitempty"`

	// Routes is the ordered list of path rules for Host. First match wins.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	Routes []RouteRule `json:"routes"`
}

// RouteRule is one path-based routing rule.
type RouteRule struct {
	// Path is the HTTP path prefix to match (e.g. "/payment").
	// +kubebuilder:validation:Pattern=`^/`
	// +kubebuilder:default="/"
	// +optional
	Path string `json:"path,omitempty"`

	// Backend is the Service this rule sends traffic to.
	Backend BackendRef `json:"backend"`

	// Versions splits traffic across labelled subsets of Backend. When set, the
	// weights must sum to 100. When empty, all Backend endpoints are used.
	// +kubebuilder:validation:MaxItems=16
	// +optional
	Versions []BackendVersion `json:"versions,omitempty"`

	// Strategy selects how traffic is shifted across Versions.
	// +optional
	Strategy RoutingStrategy `json:"strategy,omitempty"`

	// Resilience configures timeouts, retries and the circuit breaker.
	// +optional
	Resilience *ResiliencePolicy `json:"resilience,omitempty"`

	// Security configures rate limiting and JWT authentication.
	// +optional
	Security *SecurityPolicy `json:"security,omitempty"`

	// Health sets the thresholds the decision engine uses to shift traffic.
	// +optional
	Health *HealthPolicy `json:"health,omitempty"`
}

// BackendRef identifies a Service in the TrafficRoute's namespace.
type BackendRef struct {
	// Service is a Service name in the TrafficRoute's namespace.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Service string `json:"service"`

	// Port is the Service port number.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
}

// BackendVersion is one weighted subset of a backend Service.
type BackendVersion struct {
	// Name identifies the version. Unless Labels is set, endpoints are resolved
	// by matching {spec.versionLabel: Name} on the backing pods.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// Weight is the relative share of traffic for this version (0-100).
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Weight int32 `json:"weight"`

	// Labels overrides the pod selector for this version.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// StrategyType is the traffic-shifting mode for a RouteRule.
// +kubebuilder:validation:Enum=WEIGHTED;CANARY;BLUE_GREEN
type StrategyType string

const (
	// StrategyWeighted holds a fixed split defined by the spec.
	StrategyWeighted StrategyType = "WEIGHTED"
	// StrategyCanary progressively shifts traffic while health stays good.
	StrategyCanary StrategyType = "CANARY"
	// StrategyBlueGreen performs an atomic cutover between two versions.
	StrategyBlueGreen StrategyType = "BLUE_GREEN"
)

// RoutingStrategy selects and tunes the traffic-shifting mode.
type RoutingStrategy struct {
	// +kubebuilder:default=WEIGHTED
	// +optional
	Type StrategyType `json:"type,omitempty"`

	// Canary tunes automatic canary progression (Type=CANARY).
	// +optional
	Canary *CanaryStrategy `json:"canary,omitempty"`

	// BlueGreen tunes blue/green cutover (Type=BLUE_GREEN).
	// +optional
	BlueGreen *BlueGreenStrategy `json:"blueGreen,omitempty"`
}

// CanaryStrategy configures the automatic canary weight ladder.
type CanaryStrategy struct {
	// Steps is the strictly-ascending ladder of canary weight percentages.
	// Defaults to [5,10,20,30,50,100] when empty.
	// +kubebuilder:validation:MaxItems=20
	// +optional
	Steps []int32 `json:"steps,omitempty"`

	// StepInterval is the minimum healthy dwell time before advancing a step.
	// +optional
	StepInterval metav1.Duration `json:"stepInterval,omitempty"`

	// MaxWeight caps automatic promotion until a manual promote (0-100).
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	MaxWeight *int32 `json:"maxWeight,omitempty"`
}

// BlueGreenStrategy configures blue/green cutover behaviour.
type BlueGreenStrategy struct {
	// AutoPromote flips 100% to the green version once it stays healthy for
	// PromoteAfter. When false, promotion requires an API call.
	// +optional
	AutoPromote bool `json:"autoPromote,omitempty"`

	// PromoteAfter is the healthy dwell time required before auto promotion.
	// +optional
	PromoteAfter metav1.Duration `json:"promoteAfter,omitempty"`
}

// ---------------------------------------------------------------------------
// Resilience
// ---------------------------------------------------------------------------

// ResiliencePolicy configures upstream timeouts, retries and circuit breaking.
type ResiliencePolicy struct {
	// Timeout is the overall upstream request timeout.
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty"`

	// +optional
	Retries *RetryPolicy `json:"retries,omitempty"`

	// +optional
	CircuitBreaker *CircuitBreakerPolicy `json:"circuitBreaker,omitempty"`
}

// RetryPolicy configures automatic retries.
type RetryPolicy struct {
	// Attempts is the number of retries after the initial request.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	Attempts int32 `json:"attempts"`

	// PerTryTimeout bounds each individual attempt. Defaults to Timeout.
	// +optional
	PerTryTimeout metav1.Duration `json:"perTryTimeout,omitempty"`

	// RetryOn lists the conditions that trigger a retry.
	// +kubebuilder:validation:items:Enum={"5xx","gateway-error","connect-failure","retriable-status-codes","reset"}
	// +optional
	RetryOn []string `json:"retryOn,omitempty"`
}

// CircuitBreakerPolicy configures the CLOSED -> OPEN -> HALF_OPEN state machine.
type CircuitBreakerPolicy struct {
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// FailureThreshold is the consecutive failure count that opens the circuit.
	// +kubebuilder:validation:Minimum=1
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

	// RecoveryTimeout is how long the circuit stays OPEN before HALF_OPEN.
	// +optional
	RecoveryTimeout metav1.Duration `json:"recoveryTimeout,omitempty"`

	// HalfOpenRequests is the number of trial requests allowed in HALF_OPEN.
	// +kubebuilder:validation:Minimum=0
	// +optional
	HalfOpenRequests int32 `json:"halfOpenRequests,omitempty"`
}

// ---------------------------------------------------------------------------
// Security
// ---------------------------------------------------------------------------

// SecurityPolicy configures request-level security controls.
type SecurityPolicy struct {
	// +optional
	RateLimit *RateLimitPolicy `json:"rateLimit,omitempty"`

	// +optional
	JWT *JWTPolicy `json:"jwt,omitempty"`
}

// RateLimitPolicy configures request rate limiting.
type RateLimitPolicy struct {
	// RequestsPerSecond is the sustained allowed rate.
	// +kubebuilder:validation:Minimum=1
	RequestsPerSecond int32 `json:"requestsPerSecond"`

	// Burst is the token-bucket burst allowance. Defaults to RequestsPerSecond.
	// +kubebuilder:validation:Minimum=0
	// +optional
	Burst int32 `json:"burst,omitempty"`

	// Algorithm selects the limiter implementation.
	// +kubebuilder:validation:Enum=FIXED_WINDOW;TOKEN_BUCKET
	// +kubebuilder:default=TOKEN_BUCKET
	// +optional
	Algorithm string `json:"algorithm,omitempty"`
}

// JWTPolicy configures JWT authentication. Signing keys are always referenced
// from a Secret; they are never inlined in the CRD.
type JWTPolicy struct {
	// Issuer is the expected "iss" claim.
	// +kubebuilder:validation:MinLength=1
	Issuer string `json:"issuer"`

	// Audiences is the set of acceptable "aud" claims.
	// +optional
	Audiences []string `json:"audiences,omitempty"`

	// JWKSSecretRef references a Secret in the TrafficRoute's namespace holding
	// the JSON Web Key Set.
	// +optional
	JWKSSecretRef *SecretKeyRef `json:"jwksSecretRef,omitempty"`

	// ForwardToken passes the validated token to the upstream. Defaults to true.
	// +optional
	ForwardToken *bool `json:"forwardToken,omitempty"`
}

// SecretKeyRef points at one key within a Secret.
type SecretKeyRef struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// +kubebuilder:default="jwks.json"
	// +optional
	Key string `json:"key,omitempty"`
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

// HealthPolicy sets the thresholds the decision engine uses to shift traffic.
type HealthPolicy struct {
	// ErrorRateThreshold is the percent error rate above which traffic to the
	// affected version is reduced.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	ErrorRateThreshold int32 `json:"errorRateThreshold,omitempty"`

	// LatencyThreshold is the p95 latency above which traffic is reduced.
	// +optional
	LatencyThreshold metav1.Duration `json:"latencyThreshold,omitempty"`

	// MinRequests is the sample size required before health rules act.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinRequests int32 `json:"minRequests,omitempty"`

	// StabilizationWindow is the healthy dwell time required before raising
	// canary weight.
	// +optional
	StabilizationWindow metav1.Duration `json:"stabilizationWindow,omitempty"`
}

// ---------------------------------------------------------------------------
// Status
// ---------------------------------------------------------------------------

// TrafficRoutePhase is a coarse lifecycle summary.
// +kubebuilder:validation:Enum=Pending;Ready;Degraded;RollingBack;Invalid
type TrafficRoutePhase string

const (
	PhasePending     TrafficRoutePhase = "Pending"
	PhaseReady       TrafficRoutePhase = "Ready"
	PhaseDegraded    TrafficRoutePhase = "Degraded"
	PhaseRollingBack TrafficRoutePhase = "RollingBack"
	PhaseInvalid     TrafficRoutePhase = "Invalid"
)

// TrafficRouteStatus is the observed state of a TrafficRoute.
type TrafficRouteStatus struct {
	// +optional
	Phase TrafficRoutePhase `json:"phase,omitempty"`

	// ObservedGeneration is the .metadata.generation last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Routes mirrors spec.routes with resolved, live state.
	// +optional
	// +listType=atomic
	Routes []RouteStatus `json:"routes,omitempty"`

	// LastDecision is the most recent traffic decision applied.
	// +optional
	LastDecision *DecisionRef `json:"lastDecision,omitempty"`

	// Conditions follows the standard Kubernetes condition conventions.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// RouteStatus is the resolved state of one RouteRule.
type RouteStatus struct {
	Path    string `json:"path"`
	Backend string `json:"backend"`

	// +optional
	// +listType=atomic
	ResolvedEndpoints []VersionEndpoints `json:"resolvedEndpoints,omitempty"`

	// +optional
	// +listType=atomic
	CurrentWeights []VersionWeight `json:"currentWeights,omitempty"`

	// +optional
	// +listType=atomic
	BackendHealth []VersionHealth `json:"backendHealth,omitempty"`
}

// VersionEndpoints reports discovered endpoints for one version.
type VersionEndpoints struct {
	Version string `json:"version"`
	Ready   int32  `json:"ready"`
	Total   int32  `json:"total"`
	// +optional
	// +listType=atomic
	Addresses []string `json:"addresses,omitempty"`
}

// VersionWeight is the effective weight currently programmed for one version.
type VersionWeight struct {
	Version string `json:"version"`
	Weight  int32  `json:"weight"`
}

// VersionHealth is the last observed health for one version.
type VersionHealth struct {
	Version string `json:"version"`

	// +kubebuilder:validation:Enum=Healthy;Degraded;Unhealthy;Unknown
	Status string `json:"status"`

	// +optional
	ErrorRate string `json:"errorRate,omitempty"`
	// +optional
	P95Latency string `json:"p95Latency,omitempty"`
	// +optional
	// +kubebuilder:validation:Enum=CLOSED;OPEN;HALF_OPEN
	CircuitState string `json:"circuitState,omitempty"`
}

// DecisionRef is a compact reference to a traffic decision recorded by the
// control plane.
type DecisionRef struct {
	// +optional
	ID string `json:"id,omitempty"`
	// +optional
	Version string `json:"version,omitempty"`

	OldWeight int32 `json:"oldWeight"`
	NewWeight int32 `json:"newWeight"`

	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Time metav1.Time `json:"time,omitempty"`
}

// ---------------------------------------------------------------------------
// Root types
// ---------------------------------------------------------------------------

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=trafficroutes,shortName=tr;troute,categories=kubetraffic
// +kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.spec.host`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// TrafficRoute is the Schema for the trafficroutes API.
type TrafficRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TrafficRouteSpec   `json:"spec,omitempty"`
	Status TrafficRouteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TrafficRouteList contains a list of TrafficRoute.
type TrafficRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TrafficRoute `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TrafficRoute{}, &TrafficRouteList{})
}
