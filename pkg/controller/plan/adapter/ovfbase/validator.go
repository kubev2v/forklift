package ovfbase

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	webbase "github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ova"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Validator for OVF-based providers (OVA and HyperV).
type Validator struct {
	*plancontext.Context
}

// Validate whether warm migration is supported from this provider type.
func (r *Validator) WarmMigration() (ok bool) {
	ok = false
	return
}

// MigrationType indicates whether the plan's migration type
// is supported by this provider.
func (r *Validator) MigrationType() bool {
	switch r.Plan.Spec.Type {
	case api.MigrationCold, "":
		return true
	default:
		return false
	}
}

// NO-OP
func (r *Validator) UdnStaticIPs(vmRef ref.Ref, client client.Client) (ok bool, err error) {
	return true, nil
}

func (r *Validator) InvalidDiskSizes(vmRef ref.Ref) ([]string, error) {
	vm := &model.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return nil, liberr.Wrap(err, "vm", vmRef.String())
	}

	invalidDisks := []string{}
	for _, disk := range vm.Disks {
		if disk.Capacity <= 0 {
			invalidDisks = append(invalidDisks, disk.FilePath)
		}
	}

	return invalidDisks, nil
}

func (r *Validator) MacConflicts(vmRef ref.Ref) ([]planbase.MacConflict, error) {
	// Get source VM using common helper
	vm, err := planbase.FindSourceVM[model.VM](r.Source.Inventory, vmRef)
	if err != nil {
		return nil, err
	}

	// Get destination VMs and extract their MACs using common helper
	destinationVMs, err := planbase.GetDestinationVMsFromInventory(r.Destination.Inventory, webbase.Param{
		Key:   webbase.DetailParam,
		Value: "all",
	})
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	// Extract source VM MACs
	var sourceMacs []string
	for _, nic := range vm.NICs {
		// Include all MACs, even empty ones - the helper function will handle filtering
		sourceMacs = append(sourceMacs, nic.MAC)
	}

	// Use common helper to detect conflicts
	return planbase.CheckMacConflicts(sourceMacs, destinationVMs), nil
}

func (r *Validator) SharedDisks(vmRef ref.Ref, client client.Client) (ok bool, s string, s2 string, err error) {
	ok = true
	return
}

// HasSnapshot - OVF-based providers don't support warm migration, so no snapshot validation needed
func (r *Validator) HasSnapshot(vmRef ref.Ref) (ok bool, msg string, category string, err error) {
	ok = true
	return
}

// Validate that a VM's networks have been mapped.
func (r *Validator) NetworksMapped(vmRef ref.Ref) (ok bool, err error) {
	if r.Plan.Referenced.Map.Network == nil {
		return
	}
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	for _, net := range vm.Networks {
		if !r.Plan.Referenced.Map.Network.Status.Refs.Find(ref.Ref{ID: net.ID}) {
			return
		}
	}
	ok = true
	return
}

// NICNetworkRefs returns one source-network ref per VM NIC.
func (r *Validator) NICNetworkRefs(vmRef ref.Ref) (refs []ref.Ref, err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	refs = make([]ref.Ref, 0, len(vm.NICs))
	for _, nic := range vm.NICs {
		refs = append(refs, ref.Ref{Name: nic.Network})
	}
	return
}

// Validate that a VM's disk backing storage has been mapped.
func (r *Validator) StorageMapped(vmRef ref.Ref) (ok bool, err error) {
	if r.Plan.Referenced.Map.Storage == nil {
		return
	}
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	for _, disk := range vm.Disks {
		if !r.Plan.Referenced.Map.Storage.Status.Refs.Find(ref.Ref{ID: disk.ID}) {
			return
		}
	}
	ok = true
	return
}

// Validate that a VM's Host isn't in maintenance mode.
func (r *Validator) MaintenanceMode(vmRef ref.Ref) (ok bool, err error) {
	ok = true
	return
}

// NO-OP
func (r *Validator) DirectStorage(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// NO-OP
func (r *Validator) StaticIPs(vmRef ref.Ref) (bool, error) {
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

// NO-OP
func (r *Validator) PVCNameTemplate(vmRef ref.Ref, pvcNameTemplate string) (ok bool, err error) {
	ok = true
	return
}

// NO-OP
func (r *Validator) GuestToolsInstalled(vmRef ref.Ref) (ok bool, err error) {
	ok = true
	return
}
