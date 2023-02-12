package openstack

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
)

// Validator
type Validator struct {
	plan      *api.Plan
	inventory web.Client
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
	vm := &model.Workload{}
	err = r.inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM not found in inventory.",
			"vm",
			vmRef.String())
		return
	}
	for _, av := range vm.AttachedVolumes {
		volType := &model.VolumeType{}
		err = r.inventory.Find(volType, ref.Ref{Name: av.VolumeType})
		if err != nil {
			err = liberr.Wrap(
				err,
				"VM not found in inventory.",
				"vm",
				vmRef.String())
			return
		}
		if !r.plan.Referenced.Map.Storage.Status.Refs.Find(ref.Ref{ID: volType.ID}) {
			return
		}
	}
	ok = true
	return
}

// Validate that a VM's networks have been mapped.
func (r *Validator) NetworksMapped(vmRef ref.Ref) (ok bool, err error) {
	if r.plan.Referenced.Map.Network == nil {
		return
	}
	vm := &model.Workload{}
	err = r.inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM not found in inventory.",
			"vm",
			vmRef.String())
		return
	}

	ok = true
	return
}

// Validate that a VM's Host isn't in maintenance mode.
func (r *Validator) MaintenanceMode(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// Validate whether warm migration is supported from this provider type.
func (r *Validator) WarmMigration() bool {
	return true
}

// Validate that no more than one of a VM's networks is mapped to the pod network.
func (r *Validator) PodNetwork(vmRef ref.Ref) (bool, error) {
	return true, nil
}
