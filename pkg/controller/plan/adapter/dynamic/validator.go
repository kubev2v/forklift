package dynamic

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/dynamic"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/inventory/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Validator validates dynamic provider VMs for migration.
type Validator struct {
	*plancontext.Context
	schema *ProviderSchema
}

// NewValidator creates a new dynamic validator with the given schema.
func NewValidator(ctx *plancontext.Context, schema *ProviderSchema) *Validator {
	return &Validator{
		Context: ctx,
		schema:  schema,
	}
}

// WarmMigration validates whether warm migration is supported.
// Dynamic providers don't support warm migration yet.
func (r *Validator) WarmMigration() (ok bool) {
	ok = false
	return
}

// NetworkMapping validates network mappings.
func (r *Validator) NetworkMapping(vmRef ref.Ref) (ok bool, err error) {
	if r.Plan.Referenced.Map.Network == nil {
		return
	}

	// Get VM using typed model
	typedVM := &model.VM{}
	err = r.Source.Inventory.Find(typedVM, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	// Convert to unstructured for schema-based access
	obj, err := typedVM.GetObject()
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	vm := &unstructured.Unstructured{}
	vm.Object = obj

	// Get networks from VM
	networksPath, found := r.schema.VM.GetField("networks")
	if !found {
		ok = true
		return
	}

	networksData, found := vm.GetSlice(networksPath)
	if !found {
		ok = true
		return
	}

	// Check each network is mapped
	for _, netData := range networksData {
		netRef := &unstructured.Unstructured{}
		netRef.Object = netData.(map[string]interface{})
		idPath, _ := r.schema.Network.GetField("id")
		id, _ := netRef.GetString(idPath)

		if !r.Plan.Referenced.Map.Network.Status.Refs.Find(ref.Ref{ID: id}) {
			return
		}
	}

	ok = true
	return
}

// PodNetwork validates that no more than one network is mapped to pod network.
func (r *Validator) PodNetwork(vmRef ref.Ref) (ok bool, err error) {
	if r.Plan.Referenced.Map.Network == nil {
		return
	}

	// Get VM using typed model
	typedVM := &model.VM{}
	err = r.Source.Inventory.Find(typedVM, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	// Convert to unstructured for schema-based access
	obj, err := typedVM.GetObject()
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	vm := &unstructured.Unstructured{}
	vm.Object = obj

	// Get NICs from VM
	nicsPath, found := r.schema.VM.GetField("nics")
	if !found {
		ok = true
		return
	}

	nicsData, found := vm.GetSlice(nicsPath)
	if !found {
		ok = true
		return
	}

	mapping := r.Plan.Referenced.Map.Network.Spec.Map
	podMapped := 0

	for i := range mapping {
		mapped := &mapping[i]
		ref := mapped.Source

		// Get network from inventory using typed model
		typedNetwork := &model.Network{}
		fErr := r.Source.Inventory.Find(typedNetwork, ref)
		if fErr != nil {
			err = fErr
			return
		}

		// Convert to unstructured for schema-based access
		netObj, fErr := typedNetwork.GetObject()
		if fErr != nil {
			err = fErr
			return
		}
		network := &unstructured.Unstructured{}
		network.Object = netObj
		networkNamePath, _ := r.schema.Network.GetField("name")
		networkName, _ := network.GetString(networkNamePath)

		// Check if any NICs use this network and it's mapped to pod
		for _, nicData := range nicsData {
			nic := &unstructured.Unstructured{}
			nic.Object = nicData.(map[string]interface{})
			nicNetworkPath, _ := r.schema.NIC.GetField("network")
			nicNetwork, _ := nic.GetString(nicNetworkPath)

			if nicNetwork == networkName && mapped.Destination.Type == Pod {
				podMapped++
			}
		}
	}

	ok = podMapped <= 1
	return
}

// StorageMapping validates storage mappings.
func (r *Validator) StorageMapping(vmRef ref.Ref) (ok bool, err error) {
	// Generic validation - can be enhanced
	ok = true
	return
}

// ChangeTrackingEnabled checks if change tracking is enabled.
// Not supported for dynamic providers.
func (r *Validator) ChangeTrackingEnabled(vmRef ref.Ref) (enabled bool, err error) {
	enabled = false
	return
}

// DirectStorage validates direct LUN/FC storage.
// Not applicable for dynamic providers.
func (r *Validator) DirectStorage(vmRef ref.Ref) (ok bool, err error) {
	ok = true
	return
}

// StorageMapped validates that VM storage has been mapped.
func (r *Validator) StorageMapped(vmRef ref.Ref) (ok bool, err error) {
	ok = r.Context.Map.Storage != nil
	return
}

// NetworksMapped validates that VM networks have been mapped.
func (r *Validator) NetworksMapped(vmRef ref.Ref) (ok bool, err error) {
	ok = r.Context.Map.Network != nil
	return
}

// MaintenanceMode validates host is not in maintenance.
// Not applicable for dynamic providers.
func (r *Validator) MaintenanceMode(vmRef ref.Ref) (ok bool, err error) {
	ok = true
	return
}

// MigrationType validates migration type is supported.
func (r *Validator) MigrationType() bool {
	// Cold migration only for dynamic providers
	return !r.Context.Plan.Spec.Warm
}

// StaticIPs validates static IP information.
// Not enforced for dynamic providers.
func (r *Validator) StaticIPs(vmRef ref.Ref) (ok bool, err error) {
	ok = true
	return
}

// UdnStaticIPs validates UDN subnet matches VM IP.
// Not enforced for dynamic providers.
func (r *Validator) UdnStaticIPs(vmRef ref.Ref, client client.Client) (ok bool, err error) {
	ok = true
	return
}

// SharedDisks validates shared disks.
// Not enforced for dynamic providers.
func (r *Validator) SharedDisks(vmRef ref.Ref, client client.Client) (ok bool, msg string, category string, err error) {
	ok = true
	return
}

// HasSnapshot validates VM has no pre-existing snapshots.
// Not enforced for dynamic providers.
func (r *Validator) HasSnapshot(vmRef ref.Ref) (ok bool, msg string, category string, err error) {
	ok = true
	return
}

// PowerState validates VM power state is compatible.
func (r *Validator) PowerState(vmRef ref.Ref) (ok bool, err error) {
	// Dynamic providers accept any power state
	ok = true
	return
}

// VMMigrationType validates VM is compatible with migration type.
func (r *Validator) VMMigrationType(vmRef ref.Ref) (ok bool, err error) {
	ok = true
	return
}

// UnSupportedDisks validates VM disks are supported.
func (r *Validator) UnSupportedDisks(vmRef ref.Ref) (unsupported []string, err error) {
	// No unsupported disks for dynamic providers
	return
}

// InvalidDiskSizes validates VM disks have valid sizes.
func (r *Validator) InvalidDiskSizes(vmRef ref.Ref) (invalid []string, err error) {
	// Get VM using typed model
	typedVM := &model.VM{}
	err = r.Source.Inventory.Find(typedVM, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	// Convert to unstructured for schema-based access
	obj, err := typedVM.GetObject()
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	vm := &unstructured.Unstructured{}
	vm.Object = obj

	disksPath, _ := r.schema.VM.GetField("disks")
	disksData, found := vm.GetSlice(disksPath)
	if !found {
		return
	}

	for _, diskData := range disksData {
		disk := &unstructured.Unstructured{}
		disk.Object = diskData.(map[string]interface{})
		capacityPath, _ := r.schema.Disk.GetField("capacity")
		capacity, _ := disk.GetInt64(capacityPath)

		if capacity <= 0 {
			nameField, _ := r.schema.Disk.GetField("name")
			diskName, _ := disk.GetString(nameField)
			invalid = append(invalid, diskName)
		}
	}

	return
}

// MacConflicts validates MAC addresses don't conflict.
func (r *Validator) MacConflicts(vmRef ref.Ref) (conflicts []planbase.MacConflict, err error) {
	// Generic MAC conflict detection
	// TODO: Implement if needed
	return
}

// PVCNameTemplate validates PVC name template.
func (r *Validator) PVCNameTemplate(vmRef ref.Ref, pvcNameTemplate string) (ok bool, err error) {
	// Accept all templates for dynamic providers
	ok = true
	return
}

// GuestToolsInstalled validates guest tools installation.
// Not enforced for dynamic providers.
func (r *Validator) GuestToolsInstalled(vmRef ref.Ref) (ok bool, err error) {
	ok = true
	return
}
