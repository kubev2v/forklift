package vsphere

import (
	"context"
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
	calicoclient "github.com/kubev2v/forklift/pkg/lib/client/calico"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	k8stypes "k8s.io/apimachinery/pkg/types"
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

// shouldMigrateSharedDisks returns whether shared disks should be migrated for the given model VM.
// VM-level setting takes precedence; falls back to plan-level setting.
func (r *Validator) shouldMigrateSharedDisks(vm *model.VM) bool {
	if planVM := r.getPlanVM(vm); planVM != nil && planVM.MigrateSharedDisks != nil {
		return *planVM.MigrateSharedDisks
	}
	return r.Plan.Spec.MigrateSharedDisks
}

func (r *Validator) SharedDisks(vmRef ref.Ref, client k8sclient.Client) (ok bool, msg string, category string, err error) {
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
	if !r.shouldMigrateSharedDisks(vm) {
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

		// Find duplicate shared disk in the plan (only count VMs that will also migrate shared disks)
		sharedDisksDuplicate := make(map[string]int)
		for _, duplicateVmRef := range r.Plan.Spec.VMs {
			migrate := r.Plan.Spec.MigrateSharedDisks
			if duplicateVmRef.MigrateSharedDisks != nil {
				migrate = *duplicateVmRef.MigrateSharedDisks
			}
			if !migrate {
				continue
			}
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

func (r *Validator) getUdnSubnet(client k8sclient.Client) (string, error) {
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
		networkConfig, err := ocpmodel.ParseNAD(&nad)
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
		fErr := r.Source.Inventory.Find(network, ref.Ref)
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

func (r *Validator) UdnStaticIPs(vmRef ref.Ref, client k8sclient.Client) (ok bool, err error) {
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

	if len(vm.NICs) > 0 && len(vm.GuestNetworks) == 0 {
		return false, nil
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

func (r *Validator) ConsolidationNeeded(vmRef ref.Ref) (needed bool, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	return vm.ConsolidationNeeded, nil
}

// ValidateCalicoNADs walks every Multus destination in the plan's network
// map, fetches each Calico-referencing NAD, and validates the NAD/Network/
// IPPool resources.
func (r *Validator) ValidateCalicoNADs(c k8sclient.Client) (planbase.CalicoValidationResult, error) {
	result := planbase.CalicoValidationResult{
		Cache: &planbase.CalicoValidationCache{
			NADs: map[k8stypes.NamespacedName]*planbase.ResolvedCalicoNAD{},
		},
	}
	if r.Plan.Referenced.Map.Network == nil {
		return result, nil
	}

	nadCfgs := map[k8stypes.NamespacedName]*ocpmodel.NetworkConfig{}
	var pools []calicoclient.IPPool
	poolsLoaded := false
	bpfChecked := false

	vrfChecked := map[string]bool{}
	vrfDataplaneChecked := false
	var vrfNetworks []calicoclient.Network
	vrfNetworksLoaded := false
	var bgpPeerNetworks map[string]bool
	bgpPeerNetworksLoaded := false
	var felixFacts *calicoclient.FelixConfig

	// Felix is the per-node daemon which programs the dataplane
	// based on k8s config.
	loadFelix := func(key k8stypes.NamespacedName) (*calicoclient.FelixConfig, error) {
		if felixFacts != nil {
			return felixFacts, nil
		}
		fc, err := calicoclient.GetFelixConfig(context.TODO(), c)
		if err != nil {
			return nil, liberr.Wrap(err, "nad", key.String())
		}
		felixFacts = fc
		return fc, nil
	}

	// IPPools are the Calico CRD describing available IP CIDRs
	// and their permitted uses.
	loadPools := func(key k8stypes.NamespacedName) (err error) {
		if poolsLoaded {
			return
		}
		pools, err = calicoclient.ListIPPools(context.TODO(), c)
		if err != nil {
			if !meta.IsNoMatchError(err) {
				return liberr.Wrap(err, "nad", key.String())
			}
			pools = nil
			err = nil
		}
		poolsLoaded = true
		return
	}

	// For each map item, try fetching a corresponding NAD.
	// Search for NAD's whose config has `type:calico`, ignore the rest.
	// The presence of a Calico `Network` CR implies per-NIC addressing
	// is possible for L2 and L3.
	for _, pair := range r.Plan.Referenced.Map.Network.Spec.Map {
		if pair.Destination.Type != planbase.Multus {
			continue
		}
		key := k8stypes.NamespacedName{
			Namespace: pair.Destination.Namespace,
			Name:      pair.Destination.Name,
		}
		if _, dup := nadCfgs[key]; dup {
			continue
		}

		cfg, err := planbase.FetchAndParseNAD(context.TODO(), c, key.Namespace, key.Name)
		if err != nil {
			nadCfgs[key] = nil
			if r.Log != nil {
				r.Log.Error(err, "Calico NAD: failed to fetch/parse",
					"namespace", key.Namespace, "name", key.Name)
			}
			result.Issues = append(result.Issues, planbase.CalicoNADIssue{
				NAD:  key,
				Kind: planbase.CalicoIssueNADUnreadable,
			})
			continue
		}
		nadCfgs[key] = cfg

		// type:calico without a "network" field is Calico's legacy L3 IPAM
		// mode. Identity preservation on secondary NICs relies on the
		// network-scoped annotations that ship with the Network resource, so
		// it applies only to NADs that reference one (l2Bridge or VRF); warn
		// that MAC/IP annotations will not be emitted for NICs mapped here.
		if cfg.Type == ocpmodel.CalicoCNIType && cfg.Network == "" {
			result.Warnings = append(result.Warnings, planbase.CalicoNADIssue{
				NAD:  key,
				Kind: planbase.CalicoIssueNADMissingNetwork,
			})
			continue
		}
		if !cfg.ReferencesCalicoNetwork() {
			continue
		}

		issueBase := planbase.CalicoNADIssue{NAD: key, Network: cfg.Network, VLAN: cfg.VLAN}

		nw, err := calicoclient.GetNetwork(context.TODO(), c, cfg.Network)
		if err != nil {
			switch {
			case meta.IsNoMatchError(err):
				issueBase.Kind = planbase.CalicoIssueNetworkCRDAbsent
				result.Issues = append(result.Issues, issueBase)
				continue
			case k8serr.IsNotFound(err):
				issueBase.Kind = planbase.CalicoIssueNetworkNotFound
				result.Issues = append(result.Issues, issueBase)
				continue
			default:
				return planbase.CalicoValidationResult{}, liberr.Wrap(err, "nad", key.String(), "network", cfg.Network)
			}
		}

		// Classify the Network before any VLAN handling (VRF / l2Bridge).
		if nw.IsVRF {
			if cfg.VLAN != 0 {
				ib := issueBase
				ib.Kind = planbase.CalicoIssueVRFVlanIgnored
				result.Warnings = append(result.Warnings, ib)
			}

			if len(cfg.IPv4Pools) == 0 && !r.Plan.Spec.PreserveStaticIPs {
				ib := issueBase
				ib.Kind = planbase.CalicoIssueVRFPoolNotPinned
				result.Warnings = append(result.Warnings, ib)
			}
			if !vrfChecked[cfg.Network] {
				vrfChecked[cfg.Network] = true

				// An empty nodeSelector is the canonical all-nodes form, so
				// all-scoped hostConfig means a node subset (Calico selectors
				// are not parsed). A VM scheduled onto an uncovered node
				// fails at CNI ADD, so with no placement constraints on the
				// plan the outcome is a scheduling lottery: Critical. When
				// the plan pins placement (targetNodeSelector/targetAffinity)
				// the user has taken control; Warn that the pin cannot be
				// verified against the network's selectors.
				if !vrfHasAllNodesEntry(nw.VRFHostConfig) {
					ib := issueBase
					if len(r.Plan.Spec.TargetNodeSelector) > 0 || r.Plan.Spec.TargetAffinity != nil {
						ib.Kind = planbase.CalicoIssueVRFPlacementUnverified
						result.Warnings = append(result.Warnings, ib)
					} else {
						ib.Kind = planbase.CalicoIssueVRFNodeScoped
						result.Issues = append(result.Issues, ib)
					}
				}

				if vrfHasEntryWithoutHostInterfaces(nw.VRFHostConfig) {
					ib := issueBase
					ib.Kind = planbase.CalicoIssueVRFNoHostInterfaces
					result.Warnings = append(result.Warnings, ib)
				}

				if !bgpPeerNetworksLoaded {
					bgpPeerNetworksLoaded = true
					bgpPeerNetworks, err = calicoclient.ListBGPPeerNetworks(context.TODO(), c)
					if err != nil {
						return planbase.CalicoValidationResult{}, liberr.Wrap(err, "nad", key.String())
					}
				}
				if !bgpPeerNetworks[cfg.Network] {
					ib := issueBase
					ib.Kind = planbase.CalicoIssueVRFNoBGPPeer
					result.Warnings = append(result.Warnings, ib)
				}

				if !vrfNetworksLoaded {
					vrfNetworksLoaded = true
					vrfNetworks, err = calicoclient.ListNetworks(context.TODO(), c)
					if err != nil {
						return planbase.CalicoValidationResult{}, liberr.Wrap(err, "nad", key.String())
					}
				}
				felix, ferr := loadFelix(key)
				if ferr != nil {
					return planbase.CalicoValidationResult{}, ferr
				}
				criticals, warns := vrfRouteTableIssues(issueBase, nw, vrfNetworks, felix)
				result.Issues = append(result.Issues, criticals...)
				result.Warnings = append(result.Warnings, warns...)

				// VRF networks only supported on NFTables dataplane.
				if !vrfDataplaneChecked {
					vrfDataplaneChecked = true
					if felix.BPFEnabled || felix.NftablesMode != calicoclient.NftablesModeEnabled {
						ib := issueBase
						ib.Kind = planbase.CalicoIssueVRFDataplaneNotNftables
						result.Issues = append(result.Issues, ib)
					}
				}
			}
			if err = loadPools(key); err != nil {
				return planbase.CalicoValidationResult{}, err
			}
			result.Cache.NADs[key] = &planbase.ResolvedCalicoNAD{
				Network:       cfg.Network,
				IsVRF:         true,
				EligiblePools: calicoclient.L3EligiblePools(pools),
			}
			continue
		}
		if nw.L2Bridge == nil {
			issueBase.Kind = planbase.CalicoIssueNetworkHasNoL2Bridge
			result.Issues = append(result.Issues, issueBase)
			continue
		}

		// L2 networks require the BPF dataplane.
		if !bpfChecked {
			bpfChecked = true
			bpfEnabled, err := calicoclient.GetBPFEnabled(context.TODO(), c)
			if err != nil {
				return planbase.CalicoValidationResult{}, liberr.Wrap(err, "nad", key.String())
			}
			if !bpfEnabled {
				ib := issueBase
				ib.Kind = planbase.CalicoIssueDataplaneNotBPF
				result.Issues = append(result.Issues, ib)
			}
		}

		if cfg.VLAN == 0 {
			issueBase.Kind = planbase.CalicoIssueVLANRequired
			result.Issues = append(result.Issues, issueBase)
			continue
		}

		entry, vlanIssueKind := resolveVLANEntry(nw.L2Bridge.VLANs, cfg.VLAN)
		if vlanIssueKind != "" {
			issueBase.Kind = vlanIssueKind
			result.Issues = append(result.Issues, issueBase)
			continue
		}

		if err = loadPools(key); err != nil {
			return planbase.CalicoValidationResult{}, err
		}
		eligible := calicoclient.L2WorkloadEligiblePools(pools, entry.Subnets)
		if len(eligible) == 0 {
			issueBase.Kind = planbase.CalicoIssueVLANHasNoIPPool
			result.Issues = append(result.Issues, issueBase)
			continue
		}

		result.Cache.NADs[key] = &planbase.ResolvedCalicoNAD{
			Network:       cfg.Network,
			VLAN:          *entry,
			EligiblePools: eligible,
		}
	}
	return result, nil
}

// CalicoVMIssues returns per-VM Calico issues for vmRef using the cache
// from ValidateCalicoNADs.
//
// Issues are deduplicated by {Kind, Network, VLAN, IP}, so two NICs
// hitting the same failure mode yield a single issue.
func (r *Validator) CalicoVMIssues(vmRef ref.Ref, cache *planbase.CalicoValidationCache) ([]planbase.CalicoIssue, error) {
	if !r.Plan.Spec.PreserveStaticIPs {
		return nil, nil
	}
	if cache == nil || len(cache.NADs) == 0 {
		return nil, nil
	}
	if r.Plan.Referenced.Map.Network == nil {
		return nil, nil
	}
	vm := &model.VM{}
	if err := r.Source.Inventory.Find(vm, vmRef); err != nil {
		return nil, liberr.Wrap(err, "vm", vmRef.String())
	}

	var issues []planbase.CalicoIssue
	seen := map[planbase.CalicoIssue]struct{}{}
	emit := func(i planbase.CalicoIssue) {
		if _, ok := seen[i]; ok {
			return
		}
		seen[i] = struct{}{}
		issues = append(issues, i)
	}
	nadPool := planbase.NewNADPool()
	nicKeys, pairsBySource, err := r.buildNICResolver(vm.NICs)
	if err != nil {
		return nil, liberr.Wrap(err, "vm", vmRef)
	}

	for i, nic := range vm.NICs {
		pair, allocated := planbase.AllocateNetwork(nadPool, pairsBySource[nicKeys[i]])
		if !allocated || pair.Destination.Type != planbase.Multus {
			continue
		}
		key := k8stypes.NamespacedName{
			Namespace: pair.Destination.Namespace,
			Name:      pair.Destination.Name,
		}
		resolved, ok := cache.NADs[key]
		if !ok {
			continue
		}
		issueBase := planbase.CalicoIssue{Network: resolved.Network, VLAN: resolved.VLAN.VID}
		ips := findInterfaceIps(vm, nic)
		if len(ips) > 1 {
			multi := issueBase
			multi.Kind = planbase.CalicoIssueTooManyIPs
			multi.IP = strings.Join(ips, ",")
			emit(multi)
			continue
		}
		for _, ip := range ips {
			perIP := issueBase
			perIP.IP = ip
			if resolved.IsVRF {
				if calicoclient.L3EligiblePoolForIP(resolved.EligiblePools, ip) == nil {
					perIP.Kind = planbase.CalicoIssueIPNotInIPPool
					emit(perIP)
				}
				continue
			}
			if !ipInAnySubnet(ip, resolved.VLAN.Subnets) {
				perIP.Kind = planbase.CalicoIssueIPNotInSubnet
				emit(perIP)
				continue
			}
			if calicoclient.L2WorkloadEligiblePoolForIP(resolved.EligiblePools, ip, resolved.VLAN.Subnets) == nil {
				perIP.Kind = planbase.CalicoIssueIPNotInIPPool
				emit(perIP)
			}
		}
	}
	return issues, nil
}

// ValidateCalicoPrimary validates the (at most one) calico-flagged
// NetworkMap entry — a type: pod destination carrying the calico field.
//
// Precondition: Plan.Referenced.Map.Network is populated by the dispatcher
// before this is called.
func (r *Validator) ValidateCalicoPrimary(c k8sclient.Client) (planbase.CalicoPrimaryValidationResult, error) {
	result := planbase.CalicoPrimaryValidationResult{
		Cache: &planbase.CalicoPrimaryValidationCache{},
	}
	if r.Plan.Referenced.Map.Network == nil {
		return result, nil
	}

	// Classify entries (VRF or L2Bridge), surface field-misplacement issues.
	var calicoEntries []api.NetworkPair
	for _, pair := range r.Plan.Referenced.Map.Network.Spec.Map {
		dest := pair.Destination
		if dest.Calico == nil {
			continue
		}
		if dest.Type != planbase.Pod {
			result.Issues = append(result.Issues, planbase.CalicoPrimaryIssue{
				Kind:    planbase.CalicoIssuePrimaryFieldsMisplaced,
				Network: dest.Calico.Network,
				VLAN:    dest.Calico.Vlan,
			})
			continue
		}
		calicoEntries = append(calicoEntries, pair)
		// vlan-without-network is a field-placement error within the block.
		if dest.Calico.Network == "" && dest.Calico.Vlan != 0 {
			result.Issues = append(result.Issues, planbase.CalicoPrimaryIssue{
				Kind: planbase.CalicoIssuePrimaryFieldsMisplaced,
				VLAN: dest.Calico.Vlan,
			})
		}
	}

	if len(calicoEntries) > 1 {
		result.Issues = append(result.Issues, planbase.CalicoPrimaryIssue{
			Kind: planbase.CalicoIssuePrimaryFieldsMisplaced,
		})
	}
	if len(calicoEntries) == 0 {
		return result, nil
	}

	entry := calicoEntries[0]
	calico := entry.Destination.Calico
	issueBase := planbase.CalicoPrimaryIssue{Network: calico.Network, VLAN: calico.Vlan}

	if !r.Plan.Spec.PreserveStaticIPs {
		result.Warnings = append(result.Warnings, planbase.CalicoPrimaryIssue{
			Kind: planbase.CalicoIssuePrimaryStaticIPsNotPreserved,
		})
	}

	pools, err := calicoclient.ListIPPools(context.TODO(), c)
	if err != nil {
		if meta.IsNoMatchError(err) {
			ib := issueBase
			ib.Kind = planbase.CalicoIssuePrimaryUnsupported
			result.Issues = append(result.Issues, ib)
			return result, nil
		}
		return planbase.CalicoPrimaryValidationResult{}, liberr.Wrap(err, "network", calico.Network)
	}

	if r.Plan.DestinationHasUdnNetwork(c) {
		ib := issueBase
		ib.Kind = planbase.CalicoIssuePrimaryConflictsWithUDN
		result.Issues = append(result.Issues, ib)
		return result, nil
	}

	if calico.Network == "" {
		result.Cache.Primary = &planbase.ResolvedCalicoPrimary{
			Source:          entry.Source.Ref,
			L3EligiblePools: calicoclient.L3EligiblePools(pools),
		}
		return result, nil
	}

	nw, err := calicoclient.GetNetwork(context.TODO(), c, calico.Network)
	if err != nil {
		switch {
		case meta.IsNoMatchError(err):
			ib := issueBase
			ib.Kind = planbase.CalicoIssuePrimaryNetworkCRDAbsent
			result.Issues = append(result.Issues, ib)
			return result, nil
		case k8serr.IsNotFound(err):
			ib := issueBase
			ib.Kind = planbase.CalicoIssuePrimaryNetworkNotFound
			result.Issues = append(result.Issues, ib)
			return result, nil
		default:
			if r.Log != nil {
				r.Log.Error(err, "Calico-primary: failed to fetch Network",
					"network", calico.Network)
			}
			return planbase.CalicoPrimaryValidationResult{}, liberr.Wrap(err, "network", calico.Network)
		}
	}

	// A non-l2Bridge network (e.g. a VRF network) carries no VLAN, so
	// the VLANRequired / VLAN-matching checks below would mislead.
	if nw.IsVRF {
		ib := issueBase
		ib.Kind = planbase.CalicoIssuePrimaryNetworkTypeUnsupported
		result.Issues = append(result.Issues, ib)
		return result, nil
	}
	if nw.L2Bridge == nil {
		ib := issueBase
		ib.Kind = planbase.CalicoIssuePrimaryNetworkHasNoL2Bridge
		result.Issues = append(result.Issues, ib)
		return result, nil
	}

	// L2 networks require the BPF dataplane.
	bpfEnabled, err := calicoclient.GetBPFEnabled(context.TODO(), c)
	if err != nil {
		return planbase.CalicoPrimaryValidationResult{}, liberr.Wrap(err, "network", calico.Network)
	}
	if !bpfEnabled {
		ib := issueBase
		ib.Kind = planbase.CalicoIssuePrimaryDataplaneNotBPF
		result.Issues = append(result.Issues, ib)
	}

	// A Network reference requires an explicit VLAN.
	if calico.Vlan == 0 {
		ib := issueBase
		ib.Kind = planbase.CalicoIssuePrimaryVLANRequired
		result.Issues = append(result.Issues, ib)
		return result, nil
	}

	vlanEntry, vlanIssueKind := resolveVLANEntry(nw.L2Bridge.VLANs, calico.Vlan)
	if vlanIssueKind != "" {
		ib := issueBase
		ib.Kind = translateVLANIssueKindToPrimary(vlanIssueKind)
		result.Issues = append(result.Issues, ib)
		return result, nil
	}

	l2Pools := calicoclient.L2WorkloadEligiblePools(pools, vlanEntry.Subnets)
	if len(l2Pools) == 0 {
		ib := issueBase
		ib.Kind = planbase.CalicoIssuePrimaryNoEligibleIPPool
		result.Issues = append(result.Issues, ib)
		return result, nil
	}

	result.Cache.Primary = &planbase.ResolvedCalicoPrimary{
		Network:         calico.Network,
		VLAN:            *vlanEntry,
		L2EligiblePools: l2Pools,
		Source:          entry.Source.Ref,
	}
	return result, nil
}

// CalicoPrimaryIssues returns per-VM Calico-primary issues for vmRef using
// the cache from ValidateCalicoPrimary.
func (r *Validator) CalicoPrimaryIssues(vmRef ref.Ref, cache *planbase.CalicoPrimaryValidationCache) ([]planbase.CalicoPrimaryIssue, error) {
	if !r.Plan.Spec.PreserveStaticIPs {
		return nil, nil
	}
	if cache == nil || cache.Primary == nil {
		return nil, nil
	}
	if r.Plan.Referenced.Map.Network == nil {
		return nil, nil
	}
	vm := &model.VM{}
	if err := r.Source.Inventory.Find(vm, vmRef); err != nil {
		return nil, liberr.Wrap(err, "vm", vmRef.String())
	}

	primary := cache.Primary
	var issues []planbase.CalicoPrimaryIssue
	seen := map[planbase.CalicoPrimaryIssue]struct{}{}
	emit := func(i planbase.CalicoPrimaryIssue) {
		if _, ok := seen[i]; ok {
			return
		}
		seen[i] = struct{}{}
		issues = append(issues, i)
	}
	nadPool := planbase.NewNADPool()
	nicKeys, pairsBySource, err := r.buildNICResolver(vm.NICs)
	if err != nil {
		return nil, liberr.Wrap(err, "vm", vmRef.String())
	}

	for i, nic := range vm.NICs {
		pair, allocated := planbase.AllocateNetwork(nadPool, pairsBySource[nicKeys[i]])
		if !allocated || pair.Destination.Type != planbase.Pod || pair.Destination.Calico == nil {
			continue
		}
		issueBase := planbase.CalicoPrimaryIssue{VMRef: vmRef, Network: primary.Network, VLAN: primary.VLAN.VID}
		ips := findInterfaceIps(vm, nic)
		if len(ips) > 1 {
			multi := issueBase
			multi.Kind = planbase.CalicoIssuePrimaryTooManyIPs
			multi.IP = strings.Join(ips, ",")
			emit(multi)
			continue
		}
		for _, ip := range ips {
			perIP := issueBase
			perIP.IP = ip
			if primary.Network == "" {
				// Case A: implicit L3 IPAM. Pool must cover IP.
				if calicoclient.L3EligiblePoolForIP(primary.L3EligiblePools, ip) == nil {
					perIP.Kind = planbase.CalicoIssuePrimaryNoEligibleIPPool
					emit(perIP)
				}
				continue
			}
			// Case C: IP must be in matched VLAN subnet AND covered by an
			// L2Workload pool.
			if !ipInAnySubnet(ip, primary.VLAN.Subnets) {
				perIP.Kind = planbase.CalicoIssuePrimaryIPNotInSubnet
				emit(perIP)
				continue
			}
			if calicoclient.L2WorkloadEligiblePoolForIP(primary.L2EligiblePools, ip, primary.VLAN.Subnets) == nil {
				perIP.Kind = planbase.CalicoIssuePrimaryNoEligibleIPPool
				emit(perIP)
			}
		}
	}
	return issues, nil
}

// translateVLANIssueKindToPrimary converts the secondary-NAD-path VLAN issue
// kinds returned by resolveVLANEntry into the Calico-primary equivalents.
// The shared resolver returns the NAD-path kinds; the primary path emits its
// own kinds so users can disambiguate primary vs secondary failures in the
// Plan condition.
func translateVLANIssueKindToPrimary(k planbase.CalicoIssueKind) planbase.CalicoIssueKind {
	switch k {
	case planbase.CalicoIssueNetworkHasNoVLANs:
		return planbase.CalicoIssuePrimaryNetworkHasNoVLANs
	case planbase.CalicoIssueVLANNotInNetwork:
		return planbase.CalicoIssuePrimaryVLANNotInNetwork
	}
	return k
}

// buildNICResolver indexes the NetworkMap pairs by source-network ID and Key
// so a per-NIC lookup returns every candidate destination. Mirrors the
// Builder's resolver so the Validator validates exactly what the Builder
// will allocate.
func (r *Validator) buildNICResolver(nics []vsphere.NIC) ([]string, map[string][]api.NetworkPair, error) {
	pairsBySource := map[string][]api.NetworkPair{}
	for _, pair := range r.Plan.Referenced.Map.Network.Spec.Map {
		network := &model.Network{}
		if err := r.Source.Inventory.Find(network, pair.Source.Ref); err != nil {
			return nil, nil, liberr.Wrap(err, "buildNICResolver, source", pair.Source.String())
		}
		if network.Variant == vsphere.NetDvPortGroup || network.Variant == vsphere.OpaqueNetwork {
			pairsBySource[network.Key] = append(pairsBySource[network.Key], pair)
		}
		pairsBySource[network.ID] = append(pairsBySource[network.ID], pair)
	}
	nicKeys := make([]string, len(nics))
	for i, nic := range nics {
		nicKeys[i] = nic.Network.ID
	}
	return nicKeys, pairsBySource, nil
}

// vrfReservedRouteTables are the kernel's own route tables — 253 (default),
// 254 (main), 255 (local) — which a VRF must never claim.
var vrfReservedRouteTables = map[int64]bool{253: true, 254: true, 255: true}

// vrfHasAllNodesEntry reports whether any spec.vrf.hostConfig entry has an
// empty (or absent) nodeSelector — such an entry applies to every node, so
// the VRF network is guaranteed to exist wherever a VM lands.
func vrfHasAllNodesEntry(entries []calicoclient.VRFHostEntry) bool {
	for _, e := range entries {
		if e.NodeSelector == "" {
			return true
		}
	}
	return false
}

// vrfHasEntryWithoutHostInterfaces reports whether any spec.vrf.hostConfig
// entry names no host interfaces. Such an entry gives pods on its nodes no
// path off the node inside the VRF, so its VMs are unreachable beyond their
// own node.
func vrfHasEntryWithoutHostInterfaces(entries []calicoclient.VRFHostEntry) bool {
	for _, e := range entries {
		if !e.HasHostInterfaces {
			return true
		}
	}
	return false
}

// vrfRouteTableIssues checks the referenced VRF Network's routeTableIndex
// values for reserved-table use and for collisions with other VRF Networks
// and with the FelixConfiguration routeTableRanges. issueBase supplies the
// NAD/Network attribution; each finding carries the offending table index.
//
// A collision with another VRF Network is provable (Critical) when at least
// one of the two entries carrying the index has no nodeSelector — an
// all-nodes entry overlaps any other node set. When both entries are
// selector-scoped, the overlap depends on which nodes the selectors match;
// selector evaluation is deliberately out of scope, so those pairs are
// reported as possible conflicts (Warn). Entries of the same Network sharing
// an index are legitimate — same VRF, same table — and never reported.
//
// The FelixConfiguration sub-check runs only when spec.routeTableRanges is
// explicitly set. When absent, Felix falls back to version-dependent
// defaults that are not modelled here — guessing them would risk false
// Criticals against tables Felix never touches.
func vrfRouteTableIssues(issueBase planbase.CalicoNADIssue, nw *calicoclient.Network, all []calicoclient.Network, felix *calicoclient.FelixConfig) (criticals, warnings []planbase.CalicoNADIssue) {
	// Distinct indexes in entry order; per index, remember whether any
	// entry carrying it applies to all nodes.
	var indexes []int64
	seen := map[int64]bool{}
	allNodes := map[int64]bool{}
	for _, e := range nw.VRFHostConfig {
		if !seen[e.RouteTableIndex] {
			seen[e.RouteTableIndex] = true
			indexes = append(indexes, e.RouteTableIndex)
		}
		if e.NodeSelector == "" {
			allNodes[e.RouteTableIndex] = true
		}
	}

	for _, idx := range indexes {
		if vrfReservedRouteTables[idx] {
			ib := issueBase
			ib.Kind = planbase.CalicoIssueVRFRouteTableReserved
			ib.RouteTable = idx
			criticals = append(criticals, ib)
		}
		for _, rng := range felix.RouteTableRanges {
			if idx >= rng.Min && idx <= rng.Max {
				ib := issueBase
				ib.Kind = planbase.CalicoIssueVRFRouteTableConflict
				ib.RouteTable = idx
				criticals = append(criticals, ib)
				break
			}
		}
	}

	for i := range all {
		other := &all[i]
		if other.Name == nw.Name || !other.IsVRF {
			continue
		}
		otherHas := map[int64]bool{}
		otherAllNodes := map[int64]bool{}
		for _, o := range other.VRFHostConfig {
			otherHas[o.RouteTableIndex] = true
			if o.NodeSelector == "" {
				otherAllNodes[o.RouteTableIndex] = true
			}
		}
		for _, idx := range indexes {
			if !otherHas[idx] {
				continue
			}
			ib := issueBase
			ib.RouteTable = idx
			ib.ConflictsWith = other.Name
			if allNodes[idx] || otherAllNodes[idx] {
				ib.Kind = planbase.CalicoIssueVRFRouteTableConflict
				criticals = append(criticals, ib)
			} else {
				ib.Kind = planbase.CalicoIssueVRFRouteTablePossibleConflict
				warnings = append(warnings, ib)
			}
		}
	}
	return
}

// resolveVLANEntry returns the l2Bridge.vlans[] entry matched by nadVLAN.
func resolveVLANEntry(vlans []calicoclient.VLANEntry, nadVLAN uint16) (*calicoclient.VLANEntry, planbase.CalicoIssueKind) {
	if len(vlans) == 0 {
		return nil, planbase.CalicoIssueNetworkHasNoVLANs
	}
	for i := range vlans {
		if vlans[i].VID == nadVLAN {
			return &vlans[i], ""
		}
	}
	return nil, planbase.CalicoIssueVLANNotInNetwork
}

func ipInAnySubnet(ip string, subnets []string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, s := range subnets {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			continue
		}
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}
