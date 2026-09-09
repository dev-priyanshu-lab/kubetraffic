/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

// Package v1alpha1 contains the API schema for the traffic.kubetraffic.io group.
//
// +kubebuilder:object:generate=true
// +groupName=traffic.kubetraffic.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is the group/version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "traffic.kubetraffic.io", Version: "v1alpha1"}

	// SchemeBuilder collects the types for this group/version.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group/version to a scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
