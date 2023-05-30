package ovirt

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/ovirt"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"github.com/konveyor/forklift-controller/pkg/settings"
)

// vSphere validator.
type Validator struct {
	plan      *api.Plan
	inventory web.Client
}

// Load.
func (r *Validator) Load() (err error) {
	r.inventory, err = web.NewClient(r.plan.Referenced.Provider.Source)
	return
}

// Validate whether warm migration is supported from this provider type.
func (r *Validator) WarmMigration() (ok bool) {
	ok = settings.Settings.Features.OvirtWarmMigration
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

	for _, nic := range vm.NICs {
		if !r.plan.Referenced.Map.Network.Status.Refs.Find(ref.Ref{ID: nic.Profile.Network}) {
			return
		}
	}
	ok = true
	return
}

// Validate that no more than one of a VM's networks is mapped to the pod network.
func (r *Validator) PodNetwork(vmRef ref.Ref) (ok bool, err error) {
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

	mapping := r.plan.Referenced.Map.Network.Spec.Map
	podMapped := 0
	for i := range mapping {
		mapped := &mapping[i]
		ref := mapped.Source
		network := &model.Network{}
		fErr := r.inventory.Find(network, ref)
		if fErr != nil {
			err = fErr
			return
		}
		for _, nic := range vm.NICs {
			if nic.Profile.Network == network.ID && mapped.Destination.Type == Pod {
				podMapped++
			}
		}
	}

	ok = podMapped <= 1
	return
}

// Validate that a VM's disk backing storage has been mapped.
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
	for _, da := range vm.DiskAttachments {
		if da.Disk.StorageType != "lun" {
			if !r.plan.Referenced.Map.Storage.Status.Refs.Find(ref.Ref{ID: da.Disk.StorageDomain}) {
				return
			}
		} else if len(da.Disk.Lun.LogicalUnits.LogicalUnit) > 0 && da.Disk.Lun.LogicalUnits.LogicalUnit[0].Address == "" {
			// Have LUN disk but without the relevant data. This might happen with older oVirt versions.
			return
		}
	}
	ok = true
	return
}

// Validate that a VM's Host isn't in maintenance mode. No-op for oVirt.
func (r *Validator) MaintenanceMode(_ ref.Ref) (ok bool, err error) {
	ok = true
	return
}
