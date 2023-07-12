package ocp

import (
	"context"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	core "k8s.io/api/core/v1"
	cnv "kubevirt.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Validator
type Validator struct {
	plan      *api.Plan
	inventory web.Client
	client    k8sclient.Client
}

// MaintenanceMode implements base.Validator
func (r *Validator) MaintenanceMode(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// PodNetwork implements base.Validator
func (r *Validator) PodNetwork(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// WarmMigration implements base.Validator
func (r *Validator) WarmMigration() bool {
	return false
}

// Load.
func (r *Validator) Load() (err error) {
	r.inventory, err = web.NewClient(r.plan.Referenced.Provider.Source)
	return
}

func (r *Validator) StorageMapped(vmRef ref.Ref) (ok bool, err error) {
	if r.plan.Referenced.Map.Storage == nil {
		return
	}

	vm := &cnv.VirtualMachine{}
	err = r.client.Get(context.TODO(), k8sclient.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, vm)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM not found.",
			"vm",
			vmRef.String())
		return
	}

	for _, vol := range vm.Spec.Template.Spec.Volumes {
		switch {
		case vol.PersistentVolumeClaim != nil:
			// Get PVC
			pvc := &core.PersistentVolumeClaim{}
			err = r.client.Get(context.TODO(), k8sclient.ObjectKey{
				Namespace: vmRef.Namespace,
				Name:      vol.PersistentVolumeClaim.ClaimName,
			}, pvc)
			if err != nil {
				err = liberr.Wrap(
					err,
					"PVC not found.",
					"pvc",
					vol.PersistentVolumeClaim.ClaimName)
				return
			}

			storageClass := pvc.Spec.StorageClassName
			if storageClass == nil {
				return false, nil
			}

			_, found := r.plan.Referenced.Map.Storage.FindStorageByName(*storageClass)
			if !found {
				err = liberr.Wrap(
					err,
					"StorageClass not found.",
					"StorageClass",
					*storageClass)

				return false, err
			}

		}
	}

	return true, nil
}

// Validate that a VM's networks have been mapped.
func (r *Validator) NetworksMapped(vmRef ref.Ref) (ok bool, err error) {
	return true, nil
}
