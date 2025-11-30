package adapter

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DestinationClient manages EC2VolumePopulator CR ownership and lifecycle.
//
// Owner Reference Strategy:
// EC2VolumePopulator CRs cannot be owned by the Plan CR due to cross-namespace restrictions
// (Plan is in openshift-mtv, populators are in user's target namespace). Instead, each
// populator is owned by its corresponding PVC. When the PVC is deleted, Kubernetes garbage
// collection automatically cleans up the populator CR.
type DestinationClient struct {
	*plancontext.Context
}

// DeletePopulatorDataSource is a no-op for EC2.
// Ec2VolumePopulator CRs are owned by their PVCs via owner references, so Kubernetes
// garbage collection handles cleanup automatically when PVCs are deleted.
func (r *DestinationClient) DeletePopulatorDataSource(vm *planapi.VMStatus) error {
	// No manual deletion needed - garbage collection via PVC ownership handles this
	return nil
}

// SetPopulatorCrOwnership sets PVC owner references on Ec2VolumePopulator CRs.
// This ensures populators are automatically deleted when their corresponding PVCs are removed.
// Called once per VM in the CreateVM phase, but only processes populators without owner references.
func (r *DestinationClient) SetPopulatorCrOwnership() error {
	// Get only populators that don't have owner references yet
	// This makes the method efficient when called multiple times (once per VM)
	populatorCrList, err := r.getPopulatorCrListWithoutOwners()
	if err != nil {
		return liberr.Wrap(err)
	}

	if len(populatorCrList.Items) == 0 {
		r.Log.V(2).Info("No populators without owner references found")
		return nil
	}

	r.Log.Info("Setting owner references on populators", "count", len(populatorCrList.Items))

	for _, populatorCr := range populatorCrList.Items {
		// Find the PVC that references this populator
		pvc, err := r.getPVCForPopulator(&populatorCr)
		if err != nil {
			r.Log.Error(err, "Failed to find PVC for populator", "populator", populatorCr.Name)
			continue
		}

		// Set PVC as owner of the populator CR
		populatorCrCopy := populatorCr.DeepCopy()
		err = k8sutil.SetOwnerReference(pvc, populatorCrCopy, r.Scheme())
		if err != nil {
			r.Log.Error(err, "Failed to set owner reference",
				"populator", populatorCr.Name,
				"pvc", pvc.Name)
			continue
		}

		// Update the populator CR with owner reference
		err = r.Destination.Client.Patch(context.TODO(), populatorCrCopy, client.MergeFrom(&populatorCr))
		if err != nil {
			r.Log.Error(err, "Failed to patch populator with owner reference",
				"populator", populatorCr.Name,
				"pvc", pvc.Name)
			return liberr.Wrap(err)
		}

		r.Log.Info("Set PVC owner reference on populator",
			"populator", populatorCr.Name,
			"pvc", pvc.Name)
	}

	return nil
}

// getPopulatorCrListWithoutOwners retrieves Ec2VolumePopulator CRs that don't have owner references yet.
func (r *DestinationClient) getPopulatorCrListWithoutOwners() (*api.Ec2VolumePopulatorList, error) {
	// First get all populators for this migration
	allList := &api.Ec2VolumePopulatorList{}
	err := r.Destination.Client.List(
		context.TODO(),
		allList,
		&client.ListOptions{
			Namespace: r.Plan.Spec.TargetNamespace,
			LabelSelector: labels.SelectorFromSet(map[string]string{
				"migration": string(r.Migration.UID),
			}),
		},
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	// Filter to only include populators without owner references
	filteredList := &api.Ec2VolumePopulatorList{}
	for i := range allList.Items {
		populator := &allList.Items[i]
		if len(populator.OwnerReferences) == 0 {
			filteredList.Items = append(filteredList.Items, *populator)
		}
	}

	return filteredList, nil
}

// getPVCForPopulator finds the PVC that references the given Ec2VolumePopulator via dataSourceRef.
func (r *DestinationClient) getPVCForPopulator(populator *api.Ec2VolumePopulator) (*core.PersistentVolumeClaim, error) {
	pvcList := &core.PersistentVolumeClaimList{}
	err := r.Destination.Client.List(
		context.TODO(),
		pvcList,
		&client.ListOptions{
			Namespace:     populator.Namespace,
			LabelSelector: labels.SelectorFromSet(populator.Labels),
		},
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	// Find PVC with dataSourceRef pointing to this populator
	for i := range pvcList.Items {
		pvc := &pvcList.Items[i]
		if pvc.Spec.DataSourceRef != nil &&
			pvc.Spec.DataSourceRef.Kind == "Ec2VolumePopulator" &&
			pvc.Spec.DataSourceRef.Name == populator.Name {
			return pvc, nil
		}
	}

	return nil, liberr.New("PVC not found for populator", "populator", populator.Name)
}
