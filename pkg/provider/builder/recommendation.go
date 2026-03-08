package builder

import (
	"context"
	"fmt"
	"math"
	"sort"

	template "github.com/openshift/api/template/v1"
	instancetypeapi "kubevirt.io/api/instancetype"
	instancetype "kubevirt.io/api/instancetype/v1beta1"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolvePreference looks up a VirtualMachinePreference by name,
// first in the given namespace, then cluster-wide.
// Returns a RecommendedResource with name and kind, or an empty resource if not found.
func ResolvePreference(k8sClient client.Client, namespace string, preferenceName string) (RecommendedResource, error) {
	if preferenceName == "" {
		return RecommendedResource{}, nil
	}

	// Try namespace-scoped preference first
	pref := &instancetype.VirtualMachinePreference{}
	err := k8sClient.Get(
		context.TODO(),
		client.ObjectKey{Name: preferenceName, Namespace: namespace},
		pref,
	)
	if err == nil {
		return RecommendedResource{
			Name: preferenceName,
			Kind: instancetypeapi.SingularPreferenceResourceName,
		}, nil
	}
	if !k8serr.IsNotFound(err) {
		return RecommendedResource{}, fmt.Errorf("failed to get VirtualMachinePreference %s/%s: %w", namespace, preferenceName, err)
	}

	// Try cluster-scoped preference
	clusterPref := &instancetype.VirtualMachineClusterPreference{}
	err = k8sClient.Get(
		context.TODO(),
		client.ObjectKey{Name: preferenceName},
		clusterPref,
	)
	if err == nil {
		return RecommendedResource{
			Name: preferenceName,
			Kind: instancetypeapi.ClusterSingularPreferenceResourceName,
		}, nil
	}
	if k8serr.IsNotFound(err) {
		return RecommendedResource{}, nil
	}

	return RecommendedResource{}, fmt.Errorf("failed to get VirtualMachineClusterPreference %s: %w", preferenceName, err)
}

// ResolveInstanceType looks up a VirtualMachineInstancetype by name,
// first in the given namespace, then cluster-wide.
// Returns a RecommendedResource with name and kind, or an empty resource if not found.
func ResolveInstanceType(k8sClient client.Client, namespace string, instanceTypeName string) (RecommendedResource, error) {
	if instanceTypeName == "" {
		return RecommendedResource{}, nil
	}

	// Try namespace-scoped instancetype first
	it := &instancetype.VirtualMachineInstancetype{}
	err := k8sClient.Get(
		context.TODO(),
		client.ObjectKey{Name: instanceTypeName, Namespace: namespace},
		it,
	)
	if err == nil {
		return RecommendedResource{
			Name: instanceTypeName,
			Kind: instancetypeapi.SingularResourceName,
		}, nil
	}
	if !k8serr.IsNotFound(err) {
		return RecommendedResource{}, fmt.Errorf("failed to get VirtualMachineInstancetype %s/%s: %w", namespace, instanceTypeName, err)
	}

	// Try cluster-scoped instancetype
	clusterIT := &instancetype.VirtualMachineClusterInstancetype{}
	err = k8sClient.Get(
		context.TODO(),
		client.ObjectKey{Name: instanceTypeName},
		clusterIT,
	)
	if err == nil {
		return RecommendedResource{
			Name: instanceTypeName,
			Kind: instancetypeapi.ClusterSingularResourceName,
		}, nil
	}
	if k8serr.IsNotFound(err) {
		return RecommendedResource{}, nil
	}

	return RecommendedResource{}, fmt.Errorf("failed to get VirtualMachineClusterInstancetype %s: %w", instanceTypeName, err)
}

// ResolveTemplate queries OpenShift templates in the "openshift" namespace
// by label selector and returns the newest matching template.
// Returns an empty resource if no matching template is found or if the cluster
// does not support OpenShift templates.
func ResolveTemplate(k8sClient client.Client, labels map[string]string) (RecommendedResource, error) {
	if len(labels) == 0 {
		return RecommendedResource{}, nil
	}

	templateList := &template.TemplateList{}
	err := k8sClient.List(
		context.TODO(),
		templateList,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(labels),
			Namespace:     "openshift",
		},
	)
	if err != nil {
		// If the CRD doesn't exist (not on OpenShift), return empty
		if k8serr.IsNotFound(err) || isNoMatchError(err) {
			return RecommendedResource{}, nil
		}
		return RecommendedResource{}, fmt.Errorf("failed to list OpenShift templates: %w", err)
	}

	if len(templateList.Items) == 0 {
		return RecommendedResource{}, nil
	}

	// Sort by creation timestamp, newest first
	if len(templateList.Items) > 1 {
		sort.Slice(templateList.Items, func(i, j int) bool {
			return templateList.Items[j].CreationTimestamp.Before(&templateList.Items[i].CreationTimestamp)
		})
	}

	return RecommendedResource{
		Name: templateList.Items[0].Name,
		Kind: "Template",
	}, nil
}

// instanceTypeCandidate holds information about an instancetype for closest-match comparison.
type instanceTypeCandidate struct {
	Name     string
	Kind     string
	VCPUs    uint32
	MemoryMi int64
}

// FindClosestInstanceType queries available VirtualMachineInstancetypes and
// VirtualMachineClusterInstancetypes, then finds the one closest to the given
// vCPU and memory values. "Closest" is defined as minimum normalized Euclidean distance
// in (vCPU, memoryMiB) space.
// Returns an empty resource if no instancetypes are available.
func FindClosestInstanceType(k8sClient client.Client, namespace string, vcpus uint32, memoryMiB int64) (RecommendedResource, error) {
	var candidates []instanceTypeCandidate

	// List namespace-scoped instancetypes
	nsList := &instancetype.VirtualMachineInstancetypeList{}
	if err := k8sClient.List(context.TODO(), nsList, &client.ListOptions{Namespace: namespace}); err != nil {
		if !k8serr.IsNotFound(err) && !isNoMatchError(err) {
			return RecommendedResource{}, fmt.Errorf("failed to list VirtualMachineInstancetypes: %w", err)
		}
	} else {
		for _, it := range nsList.Items {
			cpuVal := it.Spec.CPU.Guest
			memVal := it.Spec.Memory.Guest.Value() / (1024 * 1024) // convert to MiB
			candidates = append(candidates, instanceTypeCandidate{
				Name:     it.Name,
				Kind:     instancetypeapi.SingularResourceName,
				VCPUs:    cpuVal,
				MemoryMi: memVal,
			})
		}
	}

	// List cluster-scoped instancetypes
	clusterList := &instancetype.VirtualMachineClusterInstancetypeList{}
	if err := k8sClient.List(context.TODO(), clusterList); err != nil {
		if !k8serr.IsNotFound(err) && !isNoMatchError(err) {
			return RecommendedResource{}, fmt.Errorf("failed to list VirtualMachineClusterInstancetypes: %w", err)
		}
	} else {
		for _, it := range clusterList.Items {
			cpuVal := it.Spec.CPU.Guest
			memVal := it.Spec.Memory.Guest.Value() / (1024 * 1024)
			candidates = append(candidates, instanceTypeCandidate{
				Name:     it.Name,
				Kind:     instancetypeapi.ClusterSingularResourceName,
				VCPUs:    cpuVal,
				MemoryMi: memVal,
			})
		}
	}

	if len(candidates) == 0 {
		return RecommendedResource{}, nil
	}

	best := findClosest(candidates, vcpus, memoryMiB)
	return RecommendedResource{
		Name: best.Name,
		Kind: best.Kind,
	}, nil
}

// findClosest selects the candidate with the minimum normalized Euclidean distance
// from (vcpus, memoryMiB). CPU and memory are normalized by the target values
// so that neither dimension dominates the distance calculation.
func findClosest(candidates []instanceTypeCandidate, vcpus uint32, memoryMiB int64) instanceTypeCandidate {
	bestIdx := 0
	bestDist := math.MaxFloat64

	// Normalize: treat the requested values as 1.0 in each dimension
	cpuNorm := float64(vcpus)
	if cpuNorm == 0 {
		cpuNorm = 1
	}
	memNorm := float64(memoryMiB)
	if memNorm == 0 {
		memNorm = 1
	}

	for i, c := range candidates {
		cpuDiff := (float64(c.VCPUs) - float64(vcpus)) / cpuNorm
		memDiff := (float64(c.MemoryMi) - float64(memoryMiB)) / memNorm
		dist := math.Sqrt(cpuDiff*cpuDiff + memDiff*memDiff)
		if dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}

	return candidates[bestIdx]
}

// isNoMatchError checks if an error indicates that the resource type is not
// registered (e.g., OpenShift Template CRD not present on vanilla Kubernetes).
func isNoMatchError(err error) bool {
	// The controller-runtime client returns a *meta.NoKindMatchError when the CRD is not installed.
	// We check the error string as a fallback since the exact type may vary.
	if err == nil {
		return false
	}
	// Check for common "no matches" or "no kind" patterns in the error string
	errStr := err.Error()
	return contains(errStr, "no matches for kind") || contains(errStr, "no match for kind") || contains(errStr, "the server could not find the requested resource")
}

// contains is a simple helper to check if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
