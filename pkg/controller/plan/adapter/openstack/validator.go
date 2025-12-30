package openstack

import (
	"strconv"

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

// NOOP
func (r *Validator) UnSupportedDisks(vmRef ref.Ref) ([]string, error) {
	return []string{}, nil
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
					if macAddress, ok := m[OSExtIPsMacAddr]; ok {
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

// Validate that no more than one of a VM's networks is mapped to the pod network.
// For OpenStack, this validates that networks mapped to pod networking don't have
// multiple NICs (identified by different MAC addresses).
func (r *Validator) PodNetwork(vmRef ref.Ref) (ok bool, msg string, err error) {
	if r.Plan.Referenced.Map.Network == nil {
		return true, "", nil
	}

	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	// Count unique NICs per network (by MAC address)
	networkUniqueNICs := r.countUniqueNICsPerNetwork(vm, vmRef)

	// Count total NICs mapped to pod networking
	podNetworkNICCount, podNetworks := r.countPodNetworkNICs(networkUniqueNICs, vmRef)

	// Validate: pod networking supports maximum 1 NIC
	if podNetworkNICCount > 1 {
		msg = r.buildValidationMessage(podNetworks, networkUniqueNICs)
		return false, msg, nil
	}

	return true, "", nil
}

// countUniqueNICsPerNetwork counts unique NICs (by MAC) for each network
func (r *Validator) countUniqueNICsPerNetwork(vm *model.Workload, vmRef ref.Ref) map[string]int {
	networkUniqueNICs := make(map[string]int)

	for networkName, addresses := range vm.Addresses {
		networkID := r.findNetworkID(vm.Networks, networkName)
		if networkID == "" {
			continue
		}

		seenMACs := make(map[string]bool)
		if nics, ok := addresses.([]interface{}); ok {
			for _, nicEntry := range nics {
				if m, ok := nicEntry.(map[string]interface{}); ok {
					// Skip floating IPs
					if ipType, ok := m[OSExtIPsType]; ok && ipType.(string) == "floating" {
						continue
					}
					// Count unique MACs
					if macAddress, ok := m[OSExtIPsMacAddr]; ok {
						if macAddr := macAddress.(string); macAddr != "" {
							seenMACs[macAddr] = true
						}
					}
				}
			}
		}
		networkUniqueNICs[networkID] = len(seenMACs)
	}

	return networkUniqueNICs
}

// findNetworkID finds the network ID for a given network name
func (r *Validator) findNetworkID(networks []model.Network, networkName string) string {
	for _, network := range networks {
		if network.Name == networkName {
			return network.ID
		}
	}
	return ""
}

// countPodNetworkNICs counts total NICs mapped to pod networking
func (r *Validator) countPodNetworkNICs(networkUniqueNICs map[string]int, vmRef ref.Ref) (int, []api.NetworkPair) {
	mapping := r.Plan.Referenced.Map.Network.Spec.Map
	podNetworkNICCount := 0
	var podNetworks []api.NetworkPair

	for i := range mapping {
		mapped := &mapping[i]
		if mapped.Destination.Type != Pod {
			continue
		}

		nicCount := networkUniqueNICs[mapped.Source.ID]
		podNetworkNICCount += nicCount
		podNetworks = append(podNetworks, *mapped)
	}

	return podNetworkNICCount, podNetworks
}

// buildValidationMessage creates a detailed error message for validation failure
func (r *Validator) buildValidationMessage(podNetworks []api.NetworkPair, networkUniqueNICs map[string]int) string {
	var networkDetails []string
	for _, mapped := range podNetworks {
		nicCount := networkUniqueNICs[mapped.Source.ID]
		if nicCount > 1 {
			networkDetails = append(networkDetails,
				mapped.Source.Name+" ("+strconv.Itoa(nicCount)+" NICs with different MAC addresses)")
		} else if nicCount == 1 {
			networkDetails = append(networkDetails,
				mapped.Source.Name+" (1 NIC)")
		}
	}

	var detailMsg string
	if len(networkDetails) > 0 {
		detailMsg = "Networks mapped to pod: " + networkDetails[0]
		for i := 1; i < len(networkDetails); i++ {
			detailMsg += ", " + networkDetails[i]
		}
		detailMsg += ". "
	}

	return "For OpenStack VMs, this can occur when a single network has multiple NICs (different MAC addresses). " +
		detailMsg +
		"Pod networking supports only 1 interface per VM. " +
		"Please map networks with multiple NICs to Multus networking instead."
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
