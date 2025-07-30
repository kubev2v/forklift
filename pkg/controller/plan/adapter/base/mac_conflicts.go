package base

import (
	"fmt"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	webbase "github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	cnv "kubevirt.io/api/core/v1"
)

// InventoryClient defines the interface for inventory clients used in MAC conflict detection
// This interface matches the behavior of web.Client without importing it directly to avoid cyclic dependencies
type InventoryClient interface {
	List(interface{}, ...webbase.Param) error
	Find(interface{}, ref.Ref) error
}

// MacConflict represents a MAC address conflict between source and destination VMs
type MacConflict struct {
	// MAC address that is conflicting
	MAC string
	// Destination VM that has the conflicting MAC address
	DestinationVM string
}

// DestinationVM represents a destination VM with MAC addresses for conflict checking
type DestinationVM struct {
	Namespace string
	Name      string
	MACs      []string
}

// ExtractMACsFromInterfaces is a helper that extracts MAC addresses from KubeVirt interface slices
// This reduces the inner loop duplication across providers
func ExtractMACsFromInterfaces(interfaces []cnv.Interface) []string {
	macs := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		macs = append(macs, iface.MacAddress)
	}
	return macs
}

// FindSourceVM retrieves a source VM by reference using the common pattern
// This encapsulates the vm := &Type{} + Find() + error wrapping pattern
func FindSourceVM[T any](inventory InventoryClient, vmRef ref.Ref) (*T, error) {
	vm := new(T)
	err := inventory.Find(vm, vmRef)
	if err != nil {
		return nil, liberr.Wrap(err, "vm", vmRef.String())
	}
	return vm, nil
}

// GetDestinationVMsFromInventory retrieves and extracts destination VMs using the common pattern
// This works for all providers (vsphere, ovirt, openstack, ova, ocp)
//
// Note: Uses ocp.VM because ALL destination environments are KubeVirt/OCP clusters.
// While this creates a dependency on the OCP package, it reflects the reality that
// all migrations target KubeVirt-based destinations.
func GetDestinationVMsFromInventory(client InventoryClient, params ...webbase.Param) ([]DestinationVM, error) {
	// Get list of existing destination VMs using the ocp.VM type
	var list []ocp.VM
	err := client.List(&list, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to list destination VMs: %w", err)
	}

	// Extract destination VMs and their MACs
	var destinationVMs []DestinationVM
	for _, kVM := range list {
		var macs []string
		if kVM.Object.Spec.Template != nil {
			macs = ExtractMACsFromInterfaces(kVM.Object.Spec.Template.Spec.Domain.Devices.Interfaces)
		}
		destinationVMs = append(destinationVMs, DestinationVM{
			Namespace: kVM.Resource.Namespace,
			Name:      kVM.Resource.Name,
			MACs:      macs,
		})
	}

	return destinationVMs, nil
}

// CheckMacConflicts is a common helper function to check MAC address conflicts.
// It takes source MAC addresses and destination VMs, returning any conflicts found.
func CheckMacConflicts(sourceMacs []string, destinationVMs []DestinationVM) []MacConflict {
	// Build MAC conflicts map from destination VMs
	macConflictsMap := make(map[string]string)
	for _, destVM := range destinationVMs {
		vmName := fmt.Sprintf("%s/%s", destVM.Namespace, destVM.Name)
		for _, mac := range destVM.MACs {
			// Skip empty MAC addresses when building conflict map - these will be auto-generated
			if mac != "" {
				macConflictsMap[mac] = vmName
			}
		}
	}

	// Check source MACs for conflicts
	var conflicts []MacConflict
	for _, mac := range sourceMacs {
		// Skip empty MAC addresses - these will be auto-generated
		if mac != "" {
			if conflictingVm, found := macConflictsMap[mac]; found {
				conflicts = append(conflicts, MacConflict{
					MAC:           mac,
					DestinationVM: conflictingVm,
				})
			}
		}
	}

	return conflicts
}
