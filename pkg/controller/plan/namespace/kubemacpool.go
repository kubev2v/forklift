package namespace

import (
	"context"

	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// KubemacpoolIgnoreLabelKey is the label key used to exclude namespaces from kubemacpool MAC address management.
// When set to "ignore", kubemacpool will skip MAC address allocation for all VMs in the labeled namespace.
// This is used in OCP-to-OCP migrations to prevent MAC address conflicts.
const KubemacpoolIgnoreLabelKey = "mutatevirtualmachines.kubemacpool.io"

// EnsureKubemacpoolExclusion automatically applies the kubemacpool exclusion label
// to the target namespace for same-cluster OCP migrations to prevent MAC address conflicts.
//
// This is specifically for namespace-to-namespace migrations within the same OpenShift cluster,
// where kubemacpool may detect MAC address conflicts between source and destination VMs.
// Cross-cluster migrations should NOT use this exclusion, as MAC conflicts there would
// indicate real networking problems that should be investigated.
//
// This is the recommended approach for bypassing kubemacpool in OpenShift Virtualization
// environments where MAC address conflicts occur during same-cluster VM migrations. By labeling
// the namespace with 'mutatevirtualmachines.kubemacpool.io=ignore', the kubemacpool admission
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
	// Gate: Only apply for same-cluster OCP migrations (namespace-to-namespace within same cluster)
	if !ctx.Plan.IsSourceProviderOCP() {
		return false, nil // Not applicable - source is not OCP
	}

	// Check if both source and destination are on the same cluster (both are "host" providers)
	sourceProvider := ctx.Plan.Referenced.Provider.Source
	destProvider := ctx.Plan.Referenced.Provider.Destination
	if sourceProvider == nil || destProvider == nil {
		return false, liberr.New("source or destination provider is not available")
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

	// Check if label already has the ignore value
	if namespace.Labels != nil {
		if value, exists := namespace.Labels[KubemacpoolIgnoreLabelKey]; exists && value == "ignore" {
			ctx.Log.Info("Namespace already has kubemacpool exclusion label", "namespace", namespace.Name)
			return true, nil
		}
	}

	// Add the kubemacpool exclusion label
	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}
	namespace.Labels[KubemacpoolIgnoreLabelKey] = "ignore"

	err = ctx.Destination.Client.Update(context.TODO(), namespace)
	if err != nil {
		return false, liberr.Wrap(err, "failed to update namespace with kubemacpool exclusion label")
	}

	ctx.Log.Info("Applied kubemacpool exclusion label to namespace",
		"namespace", namespace.Name,
		"label", KubemacpoolIgnoreLabelKey+"=ignore",
		"approach", "namespace-level bypass per Red Hat OpenShift Virtualization best practices")

	return true, nil
}

// RemoveKubemacpoolExclusion removes the kubemacpool exclusion label from the target namespace
// after migration completion. This should be called during plan cleanup/archival.
//
// Returns true if the label was removed, false if not applicable or not present.
func RemoveKubemacpoolExclusion(ctx *plancontext.Context) (removed bool, err error) {
	// Gate: Only remove for same-cluster OCP migrations (namespace-to-namespace within same cluster)
	if !ctx.Plan.IsSourceProviderOCP() {
		return false, nil // Not applicable - source is not OCP
	}

	// Check if both source and destination are on the same cluster (both are "host" providers)
	sourceProvider := ctx.Plan.Referenced.Provider.Source
	destProvider := ctx.Plan.Referenced.Provider.Destination
	if sourceProvider == nil || destProvider == nil {
		return false, liberr.New("source or destination provider is not available")
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

	// Check if the kubemacpool exclusion label exists and remove it
	if namespace.Labels != nil {
		if _, exists := namespace.Labels[KubemacpoolIgnoreLabelKey]; exists {
			delete(namespace.Labels, KubemacpoolIgnoreLabelKey)

			err = ctx.Destination.Client.Update(context.TODO(), namespace)
			if err != nil {
				return false, liberr.Wrap(err, "failed to remove kubemacpool exclusion label")
			}

			ctx.Log.Info("Removed kubemacpool exclusion label from namespace",
				"namespace", namespace.Name)
			return true, nil
		}
	}

	// Label was not present, nothing to remove
	return false, nil
}
