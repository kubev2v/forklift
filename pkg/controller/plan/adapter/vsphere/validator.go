package vsphere

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	ocpmodel "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/validation"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ToolsNotInstalled = string(types.VirtualMachineToolsStatusToolsNotInstalled)
	ToolsOk           = string(types.VirtualMachineToolsStatusToolsOk)

	GuestToolsNotRunning = string(types.VirtualMachineToolsRunningStatusGuestToolsNotRunning)
	GuestToolsRunning    = string(types.VirtualMachineToolsRunningStatusGuestToolsRunning)

	GuestToolsCurrent   = string(types.VirtualMachineToolsVersionStatusGuestToolsCurrent)
	GuestToolsUnmanaged = string(types.VirtualMachineToolsVersionStatusGuestToolsUnmanaged)
)

// vSphere validator.
type Validator struct {
	*plancontext.Context
}

const (
	namespaceLabelPrimaryUDN = "k8s.ovn.org/primary-user-defined-network"
	nadLabelUDN              = "k8s.ovn.org/user-defined-network"
)

// Validate whether warm migration is supported from this provider type.
func (r *Validator) WarmMigration() (ok bool) {
	ok = true
	return
}

// MigrationType indicates whether the plan's migration type
// is supported by this provider.
func (r *Validator) MigrationType() bool {
	switch r.Plan.Spec.Type {
	case api.MigrationCold, api.MigrationWarm, api.MigrationOnlyConversion, "":
		return true
	default:
		return false
	}
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
		refs = append(refs, ref.Ref{ID: nic.Network.ID})
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
		if !r.Plan.Referenced.Map.Storage.Status.Refs.Find(ref.Ref{ID: disk.Datastore.ID}) {
			return
		}
	}
	ok = true
	return
}

// Validate that the PVC name template is valid
func (r *Validator) PVCNameTemplate(vmRef ref.Ref, pvcNameTemplate string) (ok bool, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	if pvcNameTemplate == "" {
		return true, nil
	}

	// Get target VM name (either from TargetName field or cleaned VM name)
	targetVmName := r.getPlanVMTargetName(vm)

	for i, disk := range vm.Disks {
		testData := api.VSpherePVCNameTemplateData{
			VmName:         vm.Name,
			TargetVmName:   targetVmName,
			PlanName:       r.Plan.Name,
			DiskIndex:      i,
			RootDiskIndex:  1,
			Shared:         false,
			FileName:       extractDiskFileName(disk.File),
			WinDriveLetter: disk.WinDriveLetter,
		}

		// Use shared template validation utility
		_, templateErr := planbase.ValidatePVCNameTemplate(pvcNameTemplate, testData)
		if templateErr != nil {
			return false, templateErr
		}
	}

	if r.Log != nil {
		r.Log.Info("PVC name template is valid", "plan", r.Plan.Name, "namespace", r.Plan.Namespace, "vm", vmRef.String(), "pvcNameTemplate", pvcNameTemplate)
	}

	return true, nil
}

// Validate that a VM's Host isn't in maintenance mode.
func (r *Validator) MaintenanceMode(vmRef ref.Ref) (ok bool, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	host := &model.Host{}
	hostRef := ref.Ref{ID: vm.Host}
	err = r.Source.Inventory.Find(host, hostRef)
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
	err := r.Source.Inventory.List(&allVms, base.Param{
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

func (r *Validator) InvalidDiskSizes(vmRef ref.Ref) ([]string, error) {
	vm := &model.VM{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return nil, liberr.Wrap(err, "vm", vmRef)
	}

	invalidDisks := []string{}
	for _, disk := range vm.Disks {
		if disk.Capacity <= 0 {
			invalidDisks = append(invalidDisks, disk.File)
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
	destinationVMs, err := planbase.GetDestinationVMsFromInventory(r.Destination.Inventory, base.Param{
		Key:   base.DetailParam,
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

func (r *Validator) sharedDisksRunningVms(vm *model.VM) (runningVms []string, err error) {
	sharedDisksVms, err := r.findSharedDisksVms(vm.Disks)
	if err != nil {
		return nil, liberr.Wrap(err, "vm", vm)
	}
	return r.findRunningVms(sharedDisksVms), nil
}

func (r *Validator) SharedDisks(vmRef ref.Ref, client client.Client) (ok bool, msg string, category string, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return false, msg, "", liberr.Wrap(err, "vm", vmRef)
	}
	// Warm migration
	if vm.HasSharedDisk() && r.Plan.IsWarm() {
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
	if !r.Plan.Spec.MigrateSharedDisks {
		_, missingDiskPVCs, err := findSharedPVCs(client, vm, r.Plan.Spec.TargetNamespace)
		if err != nil {
			return false, "", "", liberr.Wrap(err, "vm", vm)
		}
		if missingDiskPVCs != nil {
			var missingDiskNames []string
			for _, disk := range missingDiskPVCs {
				missingDiskNames = append(missingDiskNames, disk.File)
			}
			msg = fmt.Sprintf("Missing shared disks PVC %s in namespace '%s', the VMs can be migrated but the disk will not be attached",
				stringifyWithQuotes(missingDiskNames), r.Plan.Spec.TargetNamespace)
			return false, msg, validation.Warn, nil
		}
	} else {
		// Find duplicate already shared disk
		sharedPVCs, _, err := findSharedPVCs(client, vm, r.Plan.Spec.TargetNamespace)
		if err != nil {
			return false, "", "", liberr.Wrap(err, "vm", vm)
		}
		if sharedPVCs != nil {
			var alreadyExistingPvc []string
			for _, pvc := range sharedPVCs {
				alreadyExistingPvc = append(alreadyExistingPvc, pvc.Annotations[planbase.AnnDiskSource])
			}
			msg = fmt.Sprintf("Already existing shared disks PVCs %s in namespace '%s', the VMs can be migrated but the disk will be duplicated",
				stringifyWithQuotes(alreadyExistingPvc), r.Plan.Spec.TargetNamespace)
			return false, msg, validation.Warn, nil
		}

		// Find duplicate shared disk in the plan
		sharedDisksDuplicate := make(map[string]int)
		for _, duplicateVmRef := range r.Plan.Spec.VMs {
			duplicateVm := &model.VM{}
			err = r.Source.Inventory.Find(duplicateVm, duplicateVmRef.Ref)
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

func (r *Validator) getUdnSubnet(client client.Client) (string, error) {
	key := k8sclient.ObjectKey{
		Name: r.Plan.Spec.TargetNamespace,
	}
	namespace := &core.Namespace{}
	err := client.Get(context.TODO(), key, namespace)
	if err != nil {
		return "", err
	}
	_, hasUdnLabel := namespace.ObjectMeta.Labels[namespaceLabelPrimaryUDN]
	if !hasUdnLabel {
		return "", nil
	}

	nadList := &k8snet.NetworkAttachmentDefinitionList{}
	listOpts := []k8sclient.ListOption{
		k8sclient.InNamespace(r.Plan.Spec.TargetNamespace),
		k8sclient.MatchingLabels{nadLabelUDN: ""},
	}

	err = client.List(context.TODO(), nadList, listOpts...)
	if err != nil {
		return "", err
	}
	for _, nad := range nadList.Items {
		var networkConfig ocpmodel.NetworkConfig
		err = json.Unmarshal([]byte(nad.Spec.Config), &networkConfig)
		if err != nil {
			r.Log.Info("Skipping NAD: failed to parse network config", "namespace", nad.Namespace, "name", nad.Name, "error", err.Error())
			continue
		}
		if networkConfig.IsUnsupportedUdn() && networkConfig.AllowPersistentIPs {
			return networkConfig.Subnets, nil
		}
	}
	return "", nil
}
func (r *Validator) getSourceNetworkForPodNetworkTarget(vmRef ref.Ref) (net *model.Network, err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef)
		return
	}

	mapping := r.Plan.Referenced.Map.Network.Spec.Map
	for i := range mapping {
		mapped := &mapping[i]
		ref := mapped.Source
		network := &model.Network{}
		fErr := r.Source.Inventory.Find(network, ref)
		if fErr != nil {
			err = fErr
			return
		}
		if mapped.Destination.Type == Pod {
			return network, nil
		}
	}
	return
}

func (r *Validator) UdnStaticIPs(vmRef ref.Ref, client client.Client) (ok bool, err error) {
	// Check static IPs
	if !r.Plan.DestinationHasUdnNetwork(client) {
		return true, nil
	}
	if ok, err = r.StaticIPs(vmRef); err != nil {
		return false, liberr.Wrap(err, "vm", vmRef)
	} else if !ok {
		return false, nil
	}
	sourceNetwork, err := r.getSourceNetworkForPodNetworkTarget(vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef)
		return
	}
	if sourceNetwork == nil {
		// No Pod network mapping found, validation passes
		return true, nil
	}
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef)
		return
	}

	udnSubnet, err := r.getUdnSubnet(client)
	if udnSubnet == "" {
		// No UDN subnet configured, validation passes
		return true, nil
	}
	if err != nil {
		return false, liberr.Wrap(err, "vm", vmRef)
	}
	for _, guestNetwork := range vm.GuestNetworks {
		if guestNetwork.Network == sourceNetwork.Name {
			// Validate the NAD
			_, ipNet, err := net.ParseCIDR(udnSubnet)
			if err != nil {
				return false, liberr.Wrap(err, "udnSubnet", udnSubnet)
			}
			ip := net.ParseIP(guestNetwork.IP)
			if ip == nil {
				// Invalid IP in guest network
				r.Log.V(4).Info("Invalid IP in guest network", "vm", vmRef.String(), "ip", guestNetwork.IP)
				return false, nil
			}
			return ipNet.Contains(ip), nil
		}
	}
	return true, nil
}

// Validate that we have information about static IPs for every guest network.
// Virtual nics are not required to have a static IP.
func (r *Validator) StaticIPs(vmRef ref.Ref) (ok bool, err error) {
	if !r.Plan.Spec.PreserveStaticIPs {
		return true, nil
	}
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef)
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
	// Check if this is a warm migration
	if !r.Plan.IsWarm() {
		return true, nil
	}
	vm := &model.Workload{}
	err := r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return false, liberr.Wrap(err, "vm", vmRef)
	}
	return vm.ChangeTrackingEnabled, nil
}

// getPlanVM get the plan VM for the given vsphere VM by looping over plan.Spec.VMs
func (r *Validator) getPlanVM(vm *model.VM) *plan.VM {
	for i := range r.Plan.Spec.VMs {
		if r.Plan.Spec.VMs[i].ID == vm.ID {
			return &r.Plan.Spec.VMs[i]
		}
	}
	return nil
}

// getPlanVMTargetName returns the target VM name, either by using the TargetName field if present,
// or by cleaning the VM name to make it DNS1123 compatible
func (r *Validator) getPlanVMTargetName(vm *model.VM) string {
	// Get plan VM from spec.vms and use the TargetName field if present
	planVM := r.getPlanVM(vm)
	if planVM != nil {
		if name := strings.TrimSpace(planVM.TargetName); name != "" {
			return name
		}
	}

	// Otherwise, clean the VM name
	return util.ChangeVmName(vm.Name)
}

// Validate that VM has no pre-existing snapshots for warm migration
func (r *Validator) HasSnapshot(vmRef ref.Ref) (ok bool, msg string, category string, err error) {
	// Check if this is a warm migration
	if !r.Plan.IsWarm() {
		return true, "", "", nil
	}
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		return false, "", "", liberr.Wrap(err, "vm", vmRef)
	}

	// Check if VM has pre-existing snapshots
	if vm.Snapshot.ID != "" {
		return false, "VM has pre-existing snapshots which are incompatible with warm migration", "", nil
	}

	return true, "", "", nil
}

func (r *Validator) PowerState(vmRef ref.Ref) (ok bool, err error) {
	ok = true
	return
}

func (r *Validator) VMMigrationType(vmRef ref.Ref) (ok bool, err error) {
	ok = true
	return
}

// Validate guest tools (VMware Tools) status for the VM.
func (r *Validator) GuestToolsInstalled(vmRef ref.Ref) (ok bool, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	// Only check VMware Tools status if VM is powered on
	if vm.PowerState == string(types.VirtualMachinePowerStatePoweredOn) {
		// Check critical VMware Tools issues: unknown status, not installed, or not running
		if isUnknownToolsStatus(vm.ToolsStatus) || vm.ToolsStatus == ToolsNotInstalled ||
			vm.ToolsRunningStatus == GuestToolsNotRunning {
			return false, nil
		}
	}

	return true, nil
}

// isUnknownToolsStatus normalizes how we treat unreported/unknown statuses.
func isUnknownToolsStatus(s string) bool {
	switch s {
	case "", "null", "<nil>":
		return true
	default:
		return false
	}
}
