package health

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/pkg/util/client"
)

// ProviderCheckResult contains the results of provider health checks
type ProviderCheckResult struct {
	Providers                  []ProviderHealth
	HasVSphereProvider         bool
	HasRemoteOpenShiftProvider bool
}

// CheckProvidersHealth checks the health of all providers
func CheckProvidersHealth(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string, allNamespaces bool) (ProviderCheckResult, error) {
	result := ProviderCheckResult{
		Providers: []ProviderHealth{},
	}

	dynamicClient, err := client.GetDynamicClient(configFlags)
	if err != nil {
		return result, err
	}

	var providers *unstructured.UnstructuredList
	if allNamespaces {
		providers, err = dynamicClient.Resource(client.ProvidersGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	} else {
		ns := namespace
		if ns == "" {
			ns = client.OpenShiftMTVNamespace
		}
		providers, err = dynamicClient.Resource(client.ProvidersGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return result, err
	}

	for _, provider := range providers.Items {
		health := analyzeProvider(&provider)
		result.Providers = append(result.Providers, health)

		// Check if this is a vSphere provider
		if health.Type == "vsphere" {
			result.HasVSphereProvider = true
		}

		// Check if this is a remote OpenShift provider (has URL set)
		if health.Type == "openshift" {
			url, _, _ := unstructured.NestedString(provider.Object, "spec", "url")
			if url != "" {
				result.HasRemoteOpenShiftProvider = true
			}
		}
	}

	return result, nil
}

// analyzeProvider analyzes a single provider and returns its health status
func analyzeProvider(provider *unstructured.Unstructured) ProviderHealth {
	health := ProviderHealth{
		Name:      provider.GetName(),
		Namespace: provider.GetNamespace(),
	}

	// Get provider type
	providerType, _, _ := unstructured.NestedString(provider.Object, "spec", "type")
	health.Type = providerType

	// Get phase
	phase, _, _ := unstructured.NestedString(provider.Object, "status", "phase")
	health.Phase = phase

	// Extract conditions
	conditions, exists, _ := unstructured.NestedSlice(provider.Object, "status", "conditions")
	if exists {
		for _, c := range conditions {
			condition, ok := c.(map[string]interface{})
			if !ok {
				continue
			}

			condType, _, _ := unstructured.NestedString(condition, "type")
			condStatus, _, _ := unstructured.NestedString(condition, "status")
			message, _, _ := unstructured.NestedString(condition, "message")

			isTrue := condStatus == "True"

			switch condType {
			case "Ready":
				health.Ready = isTrue
				if !isTrue && message != "" {
					health.Message = message
				}
			case "ConnectionTestSucceeded":
				health.Connected = isTrue
				if !isTrue && message != "" && health.Message == "" {
					health.Message = message
				}
			case "Validated":
				health.Validated = isTrue
			case "InventoryCreated":
				health.InventoryCreated = isTrue
			}
		}
	}

	return health
}

// AnalyzeProvidersHealth analyzes provider health and adds issues to the report
func AnalyzeProvidersHealth(providers []ProviderHealth, report *HealthReport) {
	for _, provider := range providers {
		// Skip the "host" provider (local OpenShift)
		if provider.Name == "host" && provider.Type == "openshift" {
			continue
		}

		// Check for not ready
		if !provider.Ready {
			severity := SeverityWarning
			message := "Provider is not ready"
			suggestion := "Check provider configuration and credentials"

			if !provider.Connected {
				severity = SeverityCritical
				message = "Provider connection failed"
				suggestion = "Verify provider URL and network connectivity"
			}

			if provider.Message != "" {
				message += ": " + provider.Message
			}

			report.AddIssue(
				severity,
				"Provider",
				provider.Name,
				message,
				suggestion,
			)
		}

		// Check for connection issues
		if !provider.Connected && provider.Ready {
			report.AddIssue(
				SeverityWarning,
				"Provider",
				provider.Name,
				"Provider connection test not succeeded",
				"Check provider credentials and network access",
			)
		}

		// Check for validation issues
		if !provider.Validated && provider.Ready {
			report.AddIssue(
				SeverityWarning,
				"Provider",
				provider.Name,
				"Provider validation not completed",
				"Review provider configuration",
			)
		}

		// Check for inventory issues
		if !provider.InventoryCreated && provider.Ready && provider.Connected {
			report.AddIssue(
				SeverityWarning,
				"Provider",
				provider.Name,
				"Provider inventory not created",
				"Wait for inventory sync or check for inventory service issues",
			)
		}
	}
}
