/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

// Package v1alpha1 holds the admission webhooks for traffic.kubetraffic.io.
//
// The CRD's OpenAPI schema already enforces structural rules (required fields,
// enums, numeric ranges, list bounds). This webhook adds the cross-field
// semantic checks a structural schema cannot express, e.g. "version weights sum
// to 100" or "CANARY needs at least two versions".
//
// It is wired into the manager in Phase 3; Phase 2 ships and unit-tests the
// validation logic so it is proven before it is deployed.
package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	trafficv1alpha1 "github.com/kubetraffic/controller/api/v1alpha1"
)

var trafficRouteGK = schema.GroupKind{Group: "traffic.kubetraffic.io", Kind: "TrafficRoute"}

// SetupTrafficRouteWebhookWithManager registers the validating webhook.
func SetupTrafficRouteWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&trafficv1alpha1.TrafficRoute{}).
		WithValidator(&TrafficRouteValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-traffic-kubetraffic-io-v1alpha1-trafficroute,mutating=false,failurePolicy=fail,sideEffects=None,groups=traffic.kubetraffic.io,resources=trafficroutes,verbs=create;update,versions=v1alpha1,name=vtrafficroute.kb.io,admissionReviewVersions=v1

// TrafficRouteValidator implements the CustomValidator interface.
type TrafficRouteValidator struct{}

var _ webhook.CustomValidator = &TrafficRouteValidator{}

// ValidateCreate validates a new TrafficRoute.
func (v *TrafficRouteValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return validate(obj)
}

// ValidateUpdate validates a change to an existing TrafficRoute.
func (v *TrafficRouteValidator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	return validate(newObj)
}

// ValidateDelete performs no validation on delete.
func (v *TrafficRouteValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validate(obj runtime.Object) (admission.Warnings, error) {
	tr, ok := obj.(*trafficv1alpha1.TrafficRoute)
	if !ok {
		return nil, fmt.Errorf("expected a TrafficRoute, got %T", obj)
	}
	logf.Log.WithName("trafficroute-webhook").V(1).Info("validating", "name", tr.Name, "namespace", tr.Namespace)

	warnings := Warnings(tr)
	if errs := ValidateTrafficRoute(tr); len(errs) > 0 {
		return warnings, apierrors.NewInvalid(trafficRouteGK, tr.Name, errs)
	}
	return warnings, nil
}

// Warnings returns non-blocking advisories for an otherwise valid TrafficRoute.
func Warnings(tr *trafficv1alpha1.TrafficRoute) admission.Warnings {
	var w admission.Warnings
	for i := range tr.Spec.Routes {
		r := &tr.Spec.Routes[i]
		if r.Strategy.Type == trafficv1alpha1.StrategyCanary && (r.Health == nil || r.Health.LatencyThreshold.Duration == 0) {
			w = append(w, fmt.Sprintf("routes[%d]: CANARY strategy without health.latencyThreshold; the decision engine can only act on error rate", i))
		}
	}
	return w
}

// ValidateTrafficRoute runs every semantic (cross-field) check and returns all
// violations. It is pure and exported so it can be unit-tested and reused by the
// reconciler as a defence-in-depth backstop.
func ValidateTrafficRoute(tr *trafficv1alpha1.TrafficRoute) field.ErrorList {
	var errs field.ErrorList
	spec := field.NewPath("spec")

	if strings.TrimSpace(tr.Spec.Host) == "" {
		errs = append(errs, field.Required(spec.Child("host"), "host is required"))
	}
	if len(tr.Spec.Routes) == 0 {
		errs = append(errs, field.Required(spec.Child("routes"), "at least one route is required"))
	}

	paths := map[string]bool{}
	for i := range tr.Spec.Routes {
		r := &tr.Spec.Routes[i]
		rp := spec.Child("routes").Index(i)

		p := r.Path
		if p == "" {
			p = "/"
		}
		if !strings.HasPrefix(p, "/") {
			errs = append(errs, field.Invalid(rp.Child("path"), r.Path, "path must start with '/'"))
		}
		if paths[p] {
			errs = append(errs, field.Duplicate(rp.Child("path"), p))
		}
		paths[p] = true

		if strings.TrimSpace(r.Backend.Service) == "" {
			errs = append(errs, field.Required(rp.Child("backend", "service"), "backend service is required"))
		}
		if r.Backend.Port < 1 || r.Backend.Port > 65535 {
			errs = append(errs, field.Invalid(rp.Child("backend", "port"), r.Backend.Port, "port must be between 1 and 65535"))
		}

		errs = append(errs, validateVersions(rp, r)...)
		errs = append(errs, validateStrategy(rp, r)...)
		errs = append(errs, validateResilience(rp, r)...)
		errs = append(errs, validateSecurity(rp, r)...)
	}
	return errs
}

func validateVersions(rp *field.Path, r *trafficv1alpha1.RouteRule) field.ErrorList {
	var errs field.ErrorList
	if len(r.Versions) == 0 {
		return errs
	}
	vp := rp.Child("versions")

	seen := map[string]bool{}
	var sum int32
	for i := range r.Versions {
		ver := &r.Versions[i]
		switch {
		case ver.Name == "":
			errs = append(errs, field.Required(vp.Index(i).Child("name"), "version name is required"))
		case seen[ver.Name]:
			errs = append(errs, field.Duplicate(vp.Index(i).Child("name"), ver.Name))
		}
		seen[ver.Name] = true

		if ver.Weight < 0 || ver.Weight > 100 {
			errs = append(errs, field.Invalid(vp.Index(i).Child("weight"), ver.Weight, "weight must be between 0 and 100"))
		}
		sum += ver.Weight
	}
	if sum != 100 {
		errs = append(errs, field.Invalid(vp, sum, "version weights must sum to 100"))
	}
	return errs
}

func validateStrategy(rp *field.Path, r *trafficv1alpha1.RouteRule) field.ErrorList {
	var errs field.ErrorList
	sp := rp.Child("strategy")

	switch r.Strategy.Type {
	case "", trafficv1alpha1.StrategyWeighted:
		// no additional constraints
	case trafficv1alpha1.StrategyCanary:
		if len(r.Versions) < 2 {
			errs = append(errs, field.Invalid(rp.Child("versions"), len(r.Versions),
				"CANARY strategy requires at least two versions"))
		}
		if r.Strategy.Canary != nil {
			errs = append(errs, validateAscending(sp.Child("canary", "steps"), r.Strategy.Canary.Steps)...)
		}
	case trafficv1alpha1.StrategyBlueGreen:
		if len(r.Versions) != 2 {
			errs = append(errs, field.Invalid(rp.Child("versions"), len(r.Versions),
				"BLUE_GREEN strategy requires exactly two versions"))
		}
	default:
		errs = append(errs, field.NotSupported(sp.Child("type"), r.Strategy.Type,
			[]string{string(trafficv1alpha1.StrategyWeighted), string(trafficv1alpha1.StrategyCanary), string(trafficv1alpha1.StrategyBlueGreen)}))
	}
	return errs
}

func validateAscending(p *field.Path, steps []int32) field.ErrorList {
	var errs field.ErrorList
	prev := int32(-1)
	for i, s := range steps {
		if s < 0 || s > 100 {
			errs = append(errs, field.Invalid(p.Index(i), s, "step must be between 0 and 100"))
		}
		if s <= prev {
			errs = append(errs, field.Invalid(p.Index(i), s, "canary steps must be strictly ascending"))
		}
		prev = s
	}
	return errs
}

func validateResilience(rp *field.Path, r *trafficv1alpha1.RouteRule) field.ErrorList {
	var errs field.ErrorList
	if r.Resilience == nil {
		return errs
	}
	resp := rp.Child("resilience")

	if rt := r.Resilience.Retries; rt != nil {
		if rt.Attempts < 0 || rt.Attempts > 10 {
			errs = append(errs, field.Invalid(resp.Child("retries", "attempts"), rt.Attempts, "attempts must be between 0 and 10"))
		}
	}
	if cb := r.Resilience.CircuitBreaker; cb != nil && cb.Enabled {
		if cb.FailureThreshold < 1 {
			errs = append(errs, field.Invalid(resp.Child("circuitBreaker", "failureThreshold"), cb.FailureThreshold,
				"failureThreshold must be >= 1 when the circuit breaker is enabled"))
		}
		if cb.HalfOpenRequests < 0 {
			errs = append(errs, field.Invalid(resp.Child("circuitBreaker", "halfOpenRequests"), cb.HalfOpenRequests, "halfOpenRequests must be >= 0"))
		}
	}
	return errs
}

func validateSecurity(rp *field.Path, r *trafficv1alpha1.RouteRule) field.ErrorList {
	var errs field.ErrorList
	if r.Security == nil {
		return errs
	}
	secp := rp.Child("security")

	if rl := r.Security.RateLimit; rl != nil {
		if rl.RequestsPerSecond < 1 {
			errs = append(errs, field.Invalid(secp.Child("rateLimit", "requestsPerSecond"), rl.RequestsPerSecond, "requestsPerSecond must be >= 1"))
		}
		if rl.Burst < 0 {
			errs = append(errs, field.Invalid(secp.Child("rateLimit", "burst"), rl.Burst, "burst must be >= 0"))
		}
	}
	if jwt := r.Security.JWT; jwt != nil {
		if strings.TrimSpace(jwt.Issuer) == "" {
			errs = append(errs, field.Required(secp.Child("jwt", "issuer"), "issuer is required when jwt is set"))
		}
		if jwt.JWKSSecretRef != nil && strings.TrimSpace(jwt.JWKSSecretRef.Name) == "" {
			errs = append(errs, field.Required(secp.Child("jwt", "jwksSecretRef", "name"), "secret name is required"))
		}
	}
	return errs
}
