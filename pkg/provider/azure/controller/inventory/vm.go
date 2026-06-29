package inventory

import (
	"errors"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/web"
)

var log = logging.WithName("azure|controller|inventory")

type VMDetails = model.VMDetails

var ErrNoAzureVMObject = errors.New("azure VM object not found in inventory")

type Inventory interface {
	Find(resource interface{}, ref ref.Ref) error
}

func GetAzureVM(inv Inventory, vmRef ref.Ref) (*VMDetails, error) {
	vm := &web.VM{}
	err := inv.Find(vm, vmRef)
	if err != nil {
		return nil, err
	}
	if vm.Object == nil {
		return nil, ErrNoAzureVMObject
	}
	// The VMDetails embeds armcompute.VirtualMachine which has a custom
	// MarshalJSON that drops extra fields during JSON serialization over
	// the wire. Restore Disks from the top-level web.VM field.
	if len(vm.Object.Disks) == 0 && len(vm.Disks) > 0 {
		vm.Object.Disks = vm.Disks
	}
	return vm.Object, nil
}

func GetManagedDisks(vmDetails *VMDetails) []model.VMDisk {
	if len(vmDetails.ManagedDisks) > 0 {
		return vmDetails.ManagedDisks
	}
	return vmDetails.Disks
}

func GetManagedDiskIDs(vmDetails *VMDetails) []string {
	disks := GetManagedDisks(vmDetails)
	log.Info("GetManagedDiskIDs",
		"managedDisksLen", len(vmDetails.ManagedDisks),
		"disksLen", len(vmDetails.Disks),
		"resolvedDisksLen", len(disks))
	var ids []string
	for _, disk := range disks {
		if disk.ID != "" {
			ids = append(ids, disk.ID)
			log.Info("Found disk ID", "id", disk.ID, "name", disk.Name, "sizeGB", disk.SizeGB)
		} else {
			log.Info("Disk has empty ID", "name", disk.Name, "sizeGB", disk.SizeGB)
		}
	}
	return ids
}

func GetNetworkInterfaces(vmDetails *VMDetails) ([]model.VMNetworkInterface, bool) {
	return vmDetails.NetworkInterfaces, len(vmDetails.NetworkInterfaces) > 0
}

func GetNetworkInterfaceIDs(vmDetails *VMDetails) []string {
	var ids []string
	for _, iface := range vmDetails.NetworkInterfaces {
		ids = append(ids, iface.ID)
	}
	return ids
}

func GetVMName(vmDetails *VMDetails) string {
	if vmDetails.Name != "" {
		return vmDetails.Name
	}
	return vmDetails.ID
}
