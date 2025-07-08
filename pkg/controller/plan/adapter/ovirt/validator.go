package ovirt

import (
	"strconv"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/container"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ovirt"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/settings"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// oVirt validator.
type Validator struct {
	*plancontext.Context
}

// NOOP
func (r *Validator) SharedDisks(vmRef ref.Ref, client client.Client) (ok bool, s string, s2 string, err error) {
	ok = true
	return
}

// Validate whether warm migration is supported from this provider type.
func (r *Validator) WarmMigration() (ok bool) {
	ok = settings.Settings.Features.OvirtWarmMigration
	return
}

// MigrationType indicates whether the plan's migration type
// is supported by this provider.
func (r *Validator) MigrationType() bool {
	switch r.Plan.Spec.Type {
	case api.MigrationCold, "":
		return true
	case api.MigrationWarm:
		return settings.Settings.Features.OvirtWarmMigration
	default:
		return false
	}
}

// Validate that a VM's networks have been mapped.
func (r *Validator) NetworksMapped(vmRef ref.Ref) (ok bool, err error) {
	if r.Plan.Referenced.Map.Network == nil {
		return
	}
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	for _, nic := range vm.NICs {
		if !r.Plan.Referenced.Map.Network.Status.Refs.Find(ref.Ref{ID: nic.Profile.Network}) {
			return
		}
	}
	ok = true
	return
}

// Validate that no more than one of a VM's networks is mapped to the pod network.
func (r *Validator) PodNetwork(vmRef ref.Ref) (ok bool, err error) {
	if r.Plan.Referenced.Map.Network == nil {
		return
	}
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	mapping := r.Plan.Referenced.Map.Network.Spec.Map
	podMapped := 0
	for i := range mapping {
		mapped := &mapping[i]
		ref := mapped.Source
		network := &model.Network{}
		fErr := r.Source.Inventory.Find(network, ref)
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
	if r.Plan.Referenced.Map.Storage == nil {
		return
	}
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	for _, da := range vm.DiskAttachments {
		if da.Disk.StorageType != "lun" && !r.Plan.Referenced.Map.Storage.Status.Refs.Find(ref.Ref{ID: da.Disk.StorageDomain}) {
			return
		}
	}
	ok = true
	return
}

// Validates oVirt version in case we use direct LUN/FC storage
func (r *Validator) DirectStorage(vmRef ref.Ref) (ok bool, err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	for _, da := range vm.DiskAttachments {
		if da.Disk.StorageType == "lun" {
			if len(da.Disk.Lun.LogicalUnits.LogicalUnit) > 0 {
				if ok, err := r.canImportDirectDisksFromProvider(); !ok {
					return ok, err
				}
			}
		}
	}
	ok = true
	return
}

// Checks the version for ovirt direct LUN/FC
func (r *Validator) canImportDirectDisksFromProvider() (bool, error) {
	// validate ovirt version > ovirt-engine-4.5.2.1 (https://github.com/oVirt/ovirt-engine/commit/e7c1f585863a332bcecfc8c3d909c9a3a56eb922)
	rl := container.Build(nil, r.Plan.Referenced.Provider.Source, r.Plan.Referenced.Secret)
	major, minor, build, revision, err := rl.Version()
	if err != nil {
		return false, err
	}
	majorVal, err := strconv.Atoi(major)
	if err != nil {
		return false, err
	}
	minorVal, err := strconv.Atoi(minor)
	if err != nil {
		return false, err
	}
	buildVal, err := strconv.Atoi(build)
	if err != nil {
		return false, err
	}
	revisionVal, err := strconv.Atoi(revision)
	if err != nil {
		return false, err
	}

	currentVersion := majorVal*1000 + minorVal*100 + buildVal*10 + revisionVal

	const minVersion = 4521

	return currentVersion >= minVersion, nil
}

// Validate that a VM's Host isn't in maintenance mode. No-op for oVirt.
func (r *Validator) MaintenanceMode(_ ref.Ref) (ok bool, err error) {
	ok = true
	return
}

// NO-OP
func (r *Validator) StaticIPs(vmRef ref.Ref) (bool, error) {
	// the guest operating system is not modified during the migration so static IPs should be preserved
	return true, nil
}

// NO-OP
func (r *Validator) ChangeTrackingEnabled(vmRef ref.Ref) (bool, error) {
	// Validate that the vm has the change tracking enabled
	return true, nil
}

func (r *Validator) PowerState(vmRef ref.Ref) (ok bool, err error) {
	ok = true
	return
}

func (r *Validator) VMMigrationType(vmRef ref.Ref) (ok bool, err error) {
	ok = true
	return
}
