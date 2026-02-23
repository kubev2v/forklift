package openstack

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/openstack"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Validator
type Validator struct {
	*plancontext.Context
}

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
	for _, volType := range vm.VolumeTypes {
		if !r.Plan.Referenced.Map.Storage.Status.Refs.Find(ref.Ref{ID: volType.ID}) {
			return
		}
	}

	// If vm is image based, we need to see glance in the storage map
	if vm.ImageID != "" && !r.Plan.Referenced.Map.Storage.Status.Refs.Find(ref.Ref{Name: api.GlanceSource}) {
		return
	}

	ok = true
	return
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
	for _, network := range vm.Networks {
		if !r.Plan.Referenced.Map.Network.Status.Refs.Find(ref.Ref{ID: network.ID}) {
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
func (r *Validator) UdnStaticIPs(vmRef ref.Ref, client client.Client) (ok bool, err error) {
	return true, nil
}

func (r *Validator) InvalidDiskSizes(vmRef ref.Ref) ([]string, error) {
	vm := &model.Workload{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return nil, liberr.Wrap(err, "vm", vmRef.String())
	}

	invalidDisks := []string{}
	for _, volume := range vm.Volumes {
		if volume.Size <= 0 {
			invalidDisks = append(invalidDisks, volume.ID)
		}
	}

	return invalidDisks, nil
}

func (r *Validator) MacConflicts(vmRef ref.Ref) ([]planbase.MacConflict, error) {
	// Get source VM using common helper
	vm, err := planbase.FindSourceVM[model.Workload](r.Source.Inventory, vmRef)
	if err != nil {
		return nil, err
	}

	// Get destination VMs and extract their MACs using common helper
	destinationVMs, err := planbase.GetDestinationVMsFromInventory(r.Destination.Inventory, base.Param{
		Key:   base.DetailParam,
		Value: "all",
	})
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	// Extract source VM MACs (OpenStack stores MACs in Addresses map)
	var sourceMacs []string
	for _, vmAddresses := range vm.Addresses {
		if nics, ok := vmAddresses.([]interface{}); ok {
			for _, nic := range nics {
				if m, ok := nic.(map[string]interface{}); ok {
					if macAddress, ok := m["OS-EXT-IPS-MAC:mac_addr"]; ok {
						macStr, ok := macAddress.(string)
						if !ok {
							continue // Skip if MAC address is not a string
						}
						// Include all MACs, even empty ones - the helper function will handle filtering
						sourceMacs = append(sourceMacs, macStr)
					}
				}
			}
		}
	}

	// Use common helper to detect conflicts
	return planbase.CheckMacConflicts(sourceMacs, destinationVMs), nil
}

func (r *Validator) SharedDisks(vmRef ref.Ref, client client.Client) (ok bool, s string, s2 string, err error) {
	ok = true
	return
}

// HasSnapshot - OpenStack doesn't support warm migration, so no snapshot validation needed
func (r *Validator) HasSnapshot(vmRef ref.Ref) (ok bool, msg string, category string, err error) {
	ok = true
	return
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

// NICNetworkRefs returns one source-network ref per VM network attachment.
func (r *Validator) NICNetworkRefs(vmRef ref.Ref) (refs []ref.Ref, err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	refs = make([]ref.Ref, 0, len(vm.Networks))
	for _, network := range vm.Networks {
		refs = append(refs, ref.Ref{ID: network.ID})
	}
	return
}

// NO-OP
func (r *Validator) DirectStorage(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// NO-OP
func (r *Validator) StaticIPs(vmRef ref.Ref) (bool, error) {
	// the guest operating system is not modified during the migration so static IPs should be preserved
	return true, nil
}

// NO-OP
func (r *Validator) ChangeTrackingEnabled(vmRef ref.Ref) (bool, error) {
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
