// Package flags provides CLI flag utilities including provider-specific hints.
package flags

import (
	"fmt"
	"strings"
)

// Provider hint constants for consistent formatting in help descriptions.
// These should be appended to flag descriptions to indicate which providers support the flag.
const (
	// Single provider hints
	ProvidersAll       = "[providers: all]"
	ProvidersVSphere   = "[providers: vsphere]"
	ProvidersOVirt     = "[providers: ovirt]"
	ProvidersOpenStack = "[providers: openstack]"
	ProvidersOpenShift = "[providers: openshift]"
	ProvidersOVA       = "[providers: ova]"
	ProvidersEC2       = "[providers: ec2]"
	ProvidersHyperV    = "[providers: hyperv]"

	// Common provider combinations
	ProvidersVSphereOVirt     = "[providers: vsphere, ovirt]"
	ProvidersVSphereEC2       = "[providers: vsphere, ec2]"
	ProvidersVSphereOpenShift = "[providers: vsphere, openshift]"

	// Providers that require guest conversion (virt-v2v)
	ProvidersConversion = "[providers: vsphere, ova, ec2, hyperv]"

	// Migration type hints
	MigrationWarm       = "[migration: warm]"
	MigrationCold       = "[migration: cold]"
	MigrationLive       = "[migration: live]"
	MigrationConversion = "[migration: conversion]"

	// Combined migration type support per provider
	MigrationTypeSupport = "[cold: all; warm: vsphere, ovirt; live: openshift; conversion: vsphere]"
)

// ProviderHint returns a formatted provider hint string for the given providers.
// Example: ProviderHint("vsphere", "ovirt") returns "[providers: vsphere, ovirt]"
func ProviderHint(providers ...string) string {
	if len(providers) == 0 {
		return ""
	}
	return fmt.Sprintf("[providers: %s]", strings.Join(providers, ", "))
}

// MigrationHint returns a formatted migration type hint string.
// Example: MigrationHint("warm") returns "[migration: warm]"
func MigrationHint(migrationTypes ...string) string {
	if len(migrationTypes) == 0 {
		return ""
	}
	return fmt.Sprintf("[migration: %s]", strings.Join(migrationTypes, ", "))
}

// CombineHints combines multiple hint strings with a space separator.
// Example: CombineHints(ProvidersVSphere, MigrationWarm) returns "[providers: vsphere] [migration: warm]"
func CombineHints(hints ...string) string {
	nonEmpty := make([]string, 0, len(hints))
	for _, h := range hints {
		if h != "" {
			nonEmpty = append(nonEmpty, h)
		}
	}
	return strings.Join(nonEmpty, " ")
}

// AppendHint appends a provider hint to a description string.
// Example: AppendHint("Preserve static IPs", ProvidersVSphere) returns "Preserve static IPs [providers: vsphere]"
func AppendHint(description string, hint string) string {
	if hint == "" {
		return description
	}
	return fmt.Sprintf("%s %s", description, hint)
}
