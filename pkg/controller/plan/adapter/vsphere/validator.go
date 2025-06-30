package vsphere

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/validation"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/vmware/govmomi/vim25/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	ok = true
	return
}

// Validate that a VM's networks have been mapped.
func (r *Validator) NetworksMapped(vmRef ref.Ref) (ok bool, err error) {
	if r.plan.Referenced.Map.Network == nil {
		return
	}
	vm := &model.VM{}
	err = r.inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	for _, net := range vm.Networks {
		if !r.plan.Referenced.Map.Network.Status.Refs.Find(ref.Ref{ID: net.ID}) {
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
		err = liberr.Wrap(err, "vm", vmRef.String())
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
			if nic.Network.ID == network.ID && mapped.Destination.Type == Pod {
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
	vm := &model.VM{}
	err = r.inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	for _, disk := range vm.Disks {
		if !r.plan.Referenced.Map.Storage.Status.Refs.Find(ref.Ref{ID: disk.Datastore.ID}) {
			return
		}
	}
	ok = true
	return
}

// Validate that a VM's Host isn't in maintenance mode.
func (r *Validator) MaintenanceMode(vmRef ref.Ref) (ok bool, err error) {
	vm := &model.VM{}
	err = r.inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	host := &model.Host{}
	hostRef := ref.Ref{ID: vm.Host}
	err = r.inventory.Find(host, hostRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String(), "host", hostRef.String())
		return
	}

	ok = !host.InMaintenanceMode
	return
}

// NO-OP
func (r *Validator) DirectStorage(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// This is inefficient, the best implementation should be done inside the inventory instead of many requests.
// The validations can take a few seconds due to the vddk validation so this should be acceptable.
// If there is problem at scale, we need to move this to the inventory, most likley under separate endpoint
// or expanding the vm list endpoint by supporting disk information in filter.
func (r *Validator) findVmsWithSharedDisk(disk vsphere.Disk) ([]model.VM, error) {
	var allVms []model.VM
	err := r.inventory.List(&allVms, base.Param{
		Key:   base.DetailParam,
		Value: "all",
	})
	if err != nil {
		return nil, liberr.Wrap(err, "disk", disk)
	}
	var resp []model.VM
	for _, vm := range allVms {
		if vm.HasDisk(disk) {
			resp = append(resp, vm)
		}
	}
	return resp, nil
}

func (r *Validator) removeDuplicateVms(vms []model.VM) []model.VM {
	allVms := make(map[string]bool)
	var noDuplicateVms []model.VM
	for _, vm := range vms {
		if _, value := allVms[vm.ID]; !value {
			allVms[vm.ID] = true
			noDuplicateVms = append(noDuplicateVms, vm)
		}
	}
	return noDuplicateVms
}

func (r *Validator) findSharedDisksVms(disks []vsphere.Disk) ([]model.VM, error) {
	var vms []model.VM
	for _, disk := range disks {
		if disk.Shared {
			vmsWithSharedDisks, err := r.findVmsWithSharedDisk(disk)
			if err != nil {
				return nil, err
			}
			vms = append(vms, vmsWithSharedDisks...)
		}
	}
	return r.removeDuplicateVms(vms), nil
}

func (r *Validator) findRunningVms(vms []model.VM) []string {
	var resp []string
	for _, vm := range vms {
		if vm.PowerState != string(types.VirtualMachinePowerStatePoweredOff) {
			resp = append(resp, vm.Name)
		}
	}
	return resp
}

func (r *Validator) sharedDisksRunningVms(vm *model.VM) (runningVms []string, err error) {
	sharedDisksVms, err := r.findSharedDisksVms(vm.Disks)
	if err != nil {
		return nil, liberr.Wrap(err, "vm", vm)
	}
	return r.findRunningVms(sharedDisksVms), nil
}

func (r *Validator) SharedDisks(vmRef ref.Ref, client client.Client) (ok bool, msg string, category string, err error) {
	vm := &model.VM{}
	err = r.inventory.Find(vm, vmRef)
	if err != nil {
		return false, msg, "", liberr.Wrap(err, "vm", vmRef)
	}
	// Warm migration
	if vm.HasSharedDisk() && r.plan.Spec.Warm {
		return false, "The shared disks cannot be used with warm migration", "", nil
	}

	// Running VMs
	runningVms, err := r.sharedDisksRunningVms(vm)
	if err != nil {
		return false, "", "", liberr.Wrap(err, "vm", vm)
	}
	if len(runningVms) > 0 {
		msg = fmt.Sprintf("Virtual Machines %s are running with attached shared disk, please power them off",
			stringifyWithQuotes(runningVms))
		return false, msg, validation.Critical, nil
	}

	// Check existing PVCs
	if !r.plan.Spec.MigrateSharedDisks {
		_, missingDiskPVCs, err := findSharedPVCs(client, vm, r.plan.Spec.TargetNamespace)
		if err != nil {
			return false, "", "", liberr.Wrap(err, "vm", vm)
		}
		if missingDiskPVCs != nil {
			var missingDiskNames []string
			for _, disk := range missingDiskPVCs {
				missingDiskNames = append(missingDiskNames, disk.File)
			}
			msg = fmt.Sprintf("Missing shared disks PVC %s in namespace '%s', the VMs can be migrated but the disk will not be attached",
				stringifyWithQuotes(missingDiskNames), r.plan.Spec.TargetNamespace)
			return false, msg, validation.Warn, nil
		}
	} else {
		// Find duplicate already shared disk
		sharedPVCs, _, err := findSharedPVCs(client, vm, r.plan.Spec.TargetNamespace)
		if err != nil {
			return false, "", "", liberr.Wrap(err, "vm", vm)
		}
		if sharedPVCs != nil {
			var alreadyExistingPvc []string
			for _, pvc := range sharedPVCs {
				alreadyExistingPvc = append(alreadyExistingPvc, pvc.Annotations[planbase.AnnDiskSource])
			}
			msg = fmt.Sprintf("Already existing shared disks PVCs %s in namespace '%s', the VMs can be migrated but the disk will be duplicated",
				stringifyWithQuotes(alreadyExistingPvc), r.plan.Spec.TargetNamespace)
			return false, msg, validation.Warn, nil
		}

		// Find duplicate shared disk in the plan
		sharedDisksDuplicate := make(map[string]int)
		for _, duplicateVmRef := range r.plan.Spec.VMs {
			duplicateVm := &model.VM{}
			err = r.inventory.Find(duplicateVm, duplicateVmRef.Ref)
			if err != nil {
				return false, "", "", liberr.Wrap(err, "vm", vm)
			}
			for _, disk := range duplicateVm.Disks {
				if disk.Shared && vm.HasDisk(disk) {
					sharedDisksDuplicate[disk.File] += 1
				}
			}
		}
		msg := ""
		disksWithMultipleOccurrences := 0
		for diskFile, occurrences := range sharedDisksDuplicate {
			if occurrences > 1 {
				msg = fmt.Sprintf("the shared disk '%s' will be coppied %dx, %s", diskFile, occurrences, msg)
				disksWithMultipleOccurrences += 1
			}
		}
		if msg != "" {
			var diskLabel string
			if disksWithMultipleOccurrences == 1 {
				diskLabel = "disk"
			} else {
				diskLabel = "disks"
			}

			msg = fmt.Sprintf(
				"%s please detach the %s from all but one of the VMs, or remove the VMs from the plan to avoid duplicating the %s",
				msg,
				diskLabel,
				diskLabel,
			)
			return false, msg, validation.Warn, nil
		}
	}
	return true, "", "", nil
}

// Validate that we have information about static IPs for every guest network.
// Virtual nics are not required to have a static IP.
func (r *Validator) StaticIPs(vmRef ref.Ref) (ok bool, err error) {
	if !r.plan.Spec.PreserveStaticIPs {
		return true, nil
	}
	vm := &model.Workload{}
	err = r.inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef)
		return
	}

	if vm.PowerState != string(types.VirtualMachinePowerStatePoweredOn) {
		return
	}
	for _, guestNetwork := range vm.GuestNetworks {
		found := false
		for _, nic := range vm.NICs {
			if nic.MAC == guestNetwork.MAC {
				found = true
				break
			}
		}
		if !found {
			return
		}
	}
	ok = true
	return
}

// Validate that the vm has the change tracking enabled
func (r *Validator) ChangeTrackingEnabled(vmRef ref.Ref) (bool, error) {
	if !r.plan.Spec.Warm {
		return true, nil
	}
	vm := &model.Workload{}
	err := r.inventory.Find(vm, vmRef)
	if err != nil {
		return false, liberr.Wrap(err, "vm", vmRef)
	}
	return vm.ChangeTrackingEnabled, nil
}
