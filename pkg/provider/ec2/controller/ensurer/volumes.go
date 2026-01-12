package ensurer

import (
	"context"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// EnsurePersistentVolumes creates PVs with CSI volume sources and returns the created PV names.
// PVs are created with ClaimRef pre-binding to their corresponding PVCs.
// Returns a map of original volume ID -> created PV name.
func (r *Ensurer) EnsurePersistentVolumes(ctx context.Context, vm *planapi.VMStatus, pvs []*core.PersistentVolume) (map[string]string, error) {
	pvNames := make(map[string]string)

	// List existing PVs by label
	existingPVList := &core.PersistentVolumeList{}
	vmLabels := r.Labeler.VMLabels(vm.Ref)
	err := r.Client.List(ctx, existingPVList, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(vmLabels),
	})
	if err != nil {
		r.log.Error(err, "Failed to list existing PVs", "vm", vm.Name)
		return nil, liberr.Wrap(err)
	}

	// Index existing PVs by EBS volume ID
	existingPVsByEBSVolume := make(map[string]*core.PersistentVolume)
	for i := range existingPVList.Items {
		pv := &existingPVList.Items[i]
		if ebsVolumeID, ok := pv.Labels["forklift.konveyor.io/ebs-volume-id"]; ok {
			existingPVsByEBSVolume[ebsVolumeID] = pv
		}
	}

	for _, pv := range pvs {
		ebsVolumeID := pv.Labels["forklift.konveyor.io/ebs-volume-id"]
		originalVolumeID := pv.Annotations["forklift.konveyor.io/original-volume-id"]

		if existingPV, exists := existingPVsByEBSVolume[ebsVolumeID]; exists {
			// PV already exists
			pvNames[originalVolumeID] = existingPV.Name
			r.log.V(1).Info("PV already exists",
				"vm", vm.Name,
				"ebsVolumeID", ebsVolumeID,
				"pvName", existingPV.Name)
			continue
		}

		// Create new PV
		err = r.Client.Create(ctx, pv)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				// Race condition: another reconcile created it
				r.log.V(1).Info("PV created by another reconcile",
					"vm", vm.Name,
					"ebsVolumeID", ebsVolumeID)
				// Fetch the created PV to get its name
				freshPVList := &core.PersistentVolumeList{}
				listErr := r.Client.List(ctx, freshPVList, &client.ListOptions{
					LabelSelector: k8slabels.SelectorFromSet(map[string]string{
						"forklift.konveyor.io/ebs-volume-id": ebsVolumeID,
					}),
				})
				if listErr == nil && len(freshPVList.Items) > 0 {
					pvNames[originalVolumeID] = freshPVList.Items[0].Name
				}
				continue
			}
			r.log.Error(err, "Failed to create PV",
				"vm", vm.Name,
				"ebsVolumeID", ebsVolumeID)
			return nil, liberr.Wrap(err)
		}

		// After creation, pv.Name contains the generated name
		pvNames[originalVolumeID] = pv.Name

		r.log.Info("Created PV for EBS volume",
			"vm", vm.Name,
			"pvName", pv.Name,
			"ebsVolumeID", ebsVolumeID,
			"originalVolumeID", originalVolumeID)
	}

	return pvNames, nil
}

// EnsureDirectPVCs creates PVCs that will bind to pre-created PVs.
// Returns a map of original volume ID -> created PVC name.
func (r *Ensurer) EnsureDirectPVCs(ctx context.Context, vm *planapi.VMStatus, pvcs []*core.PersistentVolumeClaim) (map[string]string, error) {
	pvcNames := make(map[string]string)

	// List existing PVCs by label
	existingPVCList := &core.PersistentVolumeClaimList{}
	vmLabels := r.Labeler.VMLabels(vm.Ref)
	err := r.Client.List(ctx, existingPVCList, &client.ListOptions{
		Namespace:     r.Plan.Spec.TargetNamespace,
		LabelSelector: k8slabels.SelectorFromSet(vmLabels),
	})
	if err != nil {
		r.log.Error(err, "Failed to list existing PVCs", "vm", vm.Name)
		return nil, liberr.Wrap(err)
	}

	// Index existing PVCs by source volume ID (volume-id label stores the original EC2 volume ID)
	existingPVCsByVolume := make(map[string]*core.PersistentVolumeClaim)
	for i := range existingPVCList.Items {
		pvc := &existingPVCList.Items[i]
		if volumeID, ok := pvc.Labels["forklift.konveyor.io/volume-id"]; ok {
			existingPVCsByVolume[volumeID] = pvc
		}
	}

	for _, pvc := range pvcs {
		originalVolumeID := pvc.Labels["forklift.konveyor.io/volume-id"]

		if existingPVC, exists := existingPVCsByVolume[originalVolumeID]; exists {
			// PVC already exists
			pvcNames[originalVolumeID] = existingPVC.Name
			r.log.V(1).Info("PVC already exists",
				"vm", vm.Name,
				"originalVolumeID", originalVolumeID,
				"pvcName", existingPVC.Name)
			continue
		}

		// Set owner reference to the plan for cleanup
		err = controllerutil.SetOwnerReference(r.Plan, pvc, r.Client.Scheme())
		if err != nil {
			r.log.Error(err, "Failed to set owner reference on PVC", "vm", vm.Name)
		}

		// Create new PVC
		err = r.Client.Create(ctx, pvc)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				r.log.V(1).Info("PVC created by another reconcile",
					"vm", vm.Name,
					"originalVolumeID", originalVolumeID)
				continue
			}
			r.log.Error(err, "Failed to create PVC",
				"vm", vm.Name,
				"originalVolumeID", originalVolumeID)
			return nil, liberr.Wrap(err)
		}

		// After creation, pvc.Name contains the generated name
		pvcNames[originalVolumeID] = pvc.Name

		r.log.Info("Created direct PVC",
			"vm", vm.Name,
			"pvcName", pvc.Name,
			"originalVolumeID", originalVolumeID,
			"namespace", pvc.Namespace)
	}

	return pvcNames, nil
}

// CheckDirectPVCsBound checks if all PVCs for a VM are bound.
func (r *Ensurer) CheckDirectPVCsBound(ctx context.Context, vm *planapi.VMStatus) (allBound bool, err error) {
	// List existing PVCs by label
	existingPVCList := &core.PersistentVolumeClaimList{}
	vmLabels := r.Labeler.VMLabels(vm.Ref)
	err = r.Client.List(ctx, existingPVCList, &client.ListOptions{
		Namespace:     r.Plan.Spec.TargetNamespace,
		LabelSelector: k8slabels.SelectorFromSet(vmLabels),
	})
	if err != nil {
		r.log.Error(err, "Failed to list existing PVCs", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if len(existingPVCList.Items) == 0 {
		r.log.Info("No PVCs found for VM", "vm", vm.Name)
		return false, nil
	}

	allBound = true
	for _, pvc := range existingPVCList.Items {
		if pvc.Status.Phase != core.ClaimBound {
			r.log.Info("PVC not yet bound",
				"vm", vm.Name,
				"pvc", pvc.Name,
				"phase", pvc.Status.Phase)
			allBound = false
		}
	}

	if allBound {
		r.log.Info("All PVCs are bound", "vm", vm.Name, "count", len(existingPVCList.Items))
	}

	return allBound, nil
}
