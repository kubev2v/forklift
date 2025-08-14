package namespace

import (
	"context"
	"strings"

	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// KubemacpoolIgnoreLabelKey is the label key used to exclude namespaces from kubemacpool MAC address management.
// When set to KubemacpoolIgnoreLabelValue, kubemacpool will skip MAC address allocation for all VMs in the labeled namespace.
// This is used in OCP-to-OCP migrations to prevent MAC address conflicts.
const KubemacpoolIgnoreLabelKey = "mutatevirtualmachines.kubemacpool.io"

// KubemacpoolIgnoreLabelValue is the value that tells kubemacpool to ignore a namespace.
const KubemacpoolIgnoreLabelValue = "ignore"

// KubemacpoolOwnersAnnotationKey tracks which plans are currently using kubemacpool exclusion
// for a namespace. The value is a comma-separated list of plan UIDs.
const KubemacpoolOwnersAnnotationKey = "forklift.konveyor.io/kubemacpool-owners"

// Helper functions for managing plan UID sets in annotations

// addPlanToOwners adds a plan UID to the kubemacpool owners annotation.
// Returns the updated owner set and whether any change was made.
func addPlanToOwners(namespace *core.Namespace, planUID types.UID) (owners []string, changed bool) {
	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}

	existingOwners := getPlanOwners(namespace)

	// Check if plan is already in the set
	for _, owner := range existingOwners {
		if owner == string(planUID) {
			return existingOwners, false // No change needed
		}
	}

	// Add the new plan UID
	updatedOwners := append(existingOwners, string(planUID))
	namespace.Annotations[KubemacpoolOwnersAnnotationKey] = strings.Join(updatedOwners, ",")

	return updatedOwners, true
}

// removePlanFromOwners removes a plan UID from the kubemacpool owners annotation.
// Returns the updated owner set and whether any change was made.
func removePlanFromOwners(namespace *core.Namespace, planUID types.UID) (owners []string, changed bool) {
	existingOwners := getPlanOwners(namespace)

	// Find and remove the plan UID
	var updatedOwners []string
	found := false
	for _, owner := range existingOwners {
		if owner != string(planUID) {
			updatedOwners = append(updatedOwners, owner)
		} else {
			found = true
		}
	}

	if !found {
		return existingOwners, false // Plan was not in the set
	}

	// Update or remove the annotation
	if len(updatedOwners) == 0 {
		if namespace.Annotations != nil {
			delete(namespace.Annotations, KubemacpoolOwnersAnnotationKey)
		}
	} else {
		if namespace.Annotations == nil {
			namespace.Annotations = make(map[string]string)
		}
		namespace.Annotations[KubemacpoolOwnersAnnotationKey] = strings.Join(updatedOwners, ",")
	}

	return updatedOwners, true
}

// getPlanOwners extracts the list of plan UIDs from the kubemacpool owners annotation.
func getPlanOwners(namespace *core.Namespace) []string {
	if namespace.Annotations == nil {
		return nil
	}

	owners := namespace.Annotations[KubemacpoolOwnersAnnotationKey]
	if owners == "" {
		return nil
	}

	// Split by comma and filter out empty strings
	parts := strings.Split(owners, ",")
	var result []string
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// EnsureKubemacpoolExclusion automatically applies the kubemacpool exclusion label
// to the target namespace for same-cluster OCP migrations to prevent MAC address conflicts.
//
// Uses reference counting to coordinate multiple concurrent plans targeting the same namespace.
// Each plan is tracked in the kubemacpool owners annotation, and the exclusion label is only
// removed when the last plan completes.
//
// This is specifically for namespace-to-namespace migrations within the same OpenShift cluster,
// where kubemacpool may detect MAC address conflicts between source and destination VMs.
// Cross-cluster migrations should NOT use this exclusion, as MAC conflicts there would
// indicate real networking problems that should be investigated.
//
// In OCP same-cluster migrations with MAC conflicts, label the target namespace with
// 'mutatevirtualmachines.kubemacpool.io=ignore' so that the kubemacpool admission
// webhook completely bypasses MAC address management for all VMs in that namespace.
//
// This method is shared between cold migrations (kubevirt.EnsureVM) and live migrations
// (ensurer.VirtualMachine) to provide consistent behavior across all migration types.
//
// Reference: Red Hat OpenShift Virtualization Documentation
// https://docs.redhat.com/en/documentation/openshift_container_platform/4.8/html-single/openshift_virtualization/index#virt-4-8-changes
//
// Returns true if exclusion was applied or already existed, false if not applicable for this migration type.
func EnsureKubemacpoolExclusion(ctx *plancontext.Context) (applied bool, err error) {
	// Check if both source and destination are available first (before calling any methods on them)
	sourceProvider := ctx.Plan.Referenced.Provider.Source
	destProvider := ctx.Plan.Referenced.Provider.Destination
	if sourceProvider == nil || destProvider == nil {
		return false, liberr.New("source or destination provider is not available")
	}

	// Gate: Only apply for same-cluster OCP migrations (namespace-to-namespace within same cluster)
	if !ctx.Plan.IsSourceProviderOCP() {
		return false, nil // Not applicable - source is not OCP
	}

	if !sourceProvider.IsHost() || !destProvider.IsHost() {
		return false, nil // Not applicable - cross-cluster migration, MAC conflicts should be investigated
	}

	if ctx.Plan.Spec.TargetNamespace == "" {
		return false, liberr.New("target namespace is empty")
	}

	namespace := &core.Namespace{}
	err = ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: ctx.Plan.Spec.TargetNamespace}, namespace)
	if err != nil {
		return false, liberr.Wrap(err, "failed to get target namespace")
	}

	// Create a copy before making changes for patch semantics
	orig := namespace.DeepCopy()

	// Add this plan to the owners list using reference counting
	planUID := ctx.Plan.GetUID()
	var owners []string
	changed := false
	if planUID != "" {
		owners, changed = addPlanToOwners(namespace, planUID)
	}
	// If planUID is empty (e.g., in some test contexts), proceed without owners annotation changes.

	labelAlreadyExists := false
	if namespace.Labels != nil {
		if value, exists := namespace.Labels[KubemacpoolIgnoreLabelKey]; exists && value == KubemacpoolIgnoreLabelValue {
			labelAlreadyExists = true
		}
	}

	// Add the kubemacpool exclusion label if not already present
	if !labelAlreadyExists {
		if namespace.Labels == nil {
			namespace.Labels = make(map[string]string)
		}
		namespace.Labels[KubemacpoolIgnoreLabelKey] = KubemacpoolIgnoreLabelValue
		changed = true
	}

	// Update the namespace if any changes were made
	if changed {
		err = ctx.Destination.Client.Patch(context.TODO(), namespace, k8sclient.MergeFrom(orig))
		if err != nil {
			return false, liberr.Wrap(err, "failed to patch namespace with kubemacpool exclusion")
		}

		if !labelAlreadyExists {
			ctx.Log.V(1).Info("Applied kubemacpool exclusion label to namespace",
				"namespace", namespace.Name,
				"owners", len(owners))
		} else {
			ctx.Log.V(1).Info("Added plan to kubemacpool exclusion owners",
				"namespace", namespace.Name,
				"owners", len(owners))
		}
	} else {
		ctx.Log.V(1).Info("Plan already in kubemacpool exclusion owners",
			"namespace", namespace.Name,
			"owners", len(owners))
	}

	// Let callers provide context-aware logging
	return true, nil
}

// RemoveKubemacpoolExclusion removes the kubemacpool exclusion label from the target namespace
// after migration completion. Uses reference counting to ensure the label is only removed
// when the last plan targeting this namespace completes.
//
// Returns true if the label was removed, false if not applicable or other plans still need it.
func RemoveKubemacpoolExclusion(ctx *plancontext.Context) (removed bool, err error) {
	// Check if both source and destination are available first (before calling any methods on them)
	sourceProvider := ctx.Plan.Referenced.Provider.Source
	destProvider := ctx.Plan.Referenced.Provider.Destination
	if sourceProvider == nil || destProvider == nil {
		return false, liberr.New("source or destination provider is not available")
	}
	if ctx.Destination.Client == nil {
		return false, liberr.New("destination client is not configured")
	}

	// Gate: Only remove for same-cluster OCP migrations (namespace-to-namespace within same cluster)
	if !ctx.Plan.IsSourceProviderOCP() {
		return false, nil // Not applicable - source is not OCP
	}

	if !sourceProvider.IsHost() || !destProvider.IsHost() {
		return false, nil // Not applicable - cross-cluster migration, nothing to remove
	}

	if ctx.Plan.Spec.TargetNamespace == "" {
		return false, liberr.New("target namespace is empty")
	}

	namespace := &core.Namespace{}
	err = ctx.Destination.Client.Get(context.TODO(), k8sclient.ObjectKey{Name: ctx.Plan.Spec.TargetNamespace}, namespace)
	if err != nil {
		return false, liberr.Wrap(err, "failed to get target namespace")
	}

	// Create a copy before making changes for patch semantics
	orig := namespace.DeepCopy()

	// Remove this plan from the owners list using reference counting
	planUID := ctx.Plan.GetUID()
	var remainingOwners []string
	changed := false
	if planUID != "" {
		remainingOwners, changed = removePlanFromOwners(namespace, planUID)
	} else {
		// No UID available (e.g., in some test contexts); proceed without owners annotation changes.
		remainingOwners = getPlanOwners(namespace)
	}

	labelRemoved := false

	// Only remove the kubemacpool exclusion label if no plans are using it anymore
	if changed && len(remainingOwners) == 0 {
		if namespace.Labels != nil {
			if _, exists := namespace.Labels[KubemacpoolIgnoreLabelKey]; exists {
				delete(namespace.Labels, KubemacpoolIgnoreLabelKey)
				labelRemoved = true
			}
		}
	}

	// Update the namespace if any changes were made
	if changed {
		err = ctx.Destination.Client.Patch(context.TODO(), namespace, k8sclient.MergeFrom(orig))
		if err != nil {
			return false, liberr.Wrap(err, "failed to patch namespace kubemacpool exclusion")
		}

		if labelRemoved {
			ctx.Log.V(1).Info("Removed kubemacpool exclusion label from namespace",
				"namespace", namespace.Name,
				"remainingOwners", len(remainingOwners))
		} else {
			ctx.Log.V(1).Info("Removed plan from kubemacpool exclusion owners",
				"namespace", namespace.Name,
				"remainingOwners", len(remainingOwners))
		}

		return labelRemoved, nil
	}

	// Plan was not in the owners list
	ctx.Log.V(1).Info("Plan was not in kubemacpool exclusion owners",
		"namespace", namespace.Name,
		"currentOwners", len(getPlanOwners(namespace)))
	return false, nil
}
