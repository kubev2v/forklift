package collector

import (
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

// buildGuestId constructs a guest identifier from the storage profile's
// image reference and OS type, e.g. "CentOS 7.9 (Linux)".
func buildGuestId(vm *armcompute.VirtualMachine) string {
	if vm.Properties == nil || vm.Properties.StorageProfile == nil {
		return ""
	}
	sp := vm.Properties.StorageProfile

	osType := "Linux"
	if sp.OSDisk != nil && sp.OSDisk.OSType != nil {
		osType = string(*sp.OSDisk.OSType)
	}

	if ir := sp.ImageReference; ir != nil {
		parts := []string{}
		if ir.Offer != nil && *ir.Offer != "" {
			parts = append(parts, *ir.Offer)
		}
		if ir.SKU != nil && *ir.SKU != "" {
			parts = append(parts, *ir.SKU)
		}
		if len(parts) > 0 {
			return fmt.Sprintf("%s (%s)", strings.Join(parts, " "), osType)
		}
	}

	return osType
}

// buildDisks constructs a standard disk array from the VM's storage profile.
func buildDisks(vm *armcompute.VirtualMachine) []model.VMDisk {
	if vm.Properties == nil || vm.Properties.StorageProfile == nil {
		return nil
	}
	sp := vm.Properties.StorageProfile
	var disks []model.VMDisk

	if osDisk := sp.OSDisk; osDisk != nil {
		d := model.VMDisk{IsOS: true}
		if osDisk.Name != nil {
			d.Name = *osDisk.Name
		}
		if osDisk.ManagedDisk != nil && osDisk.ManagedDisk.ID != nil {
			d.ID = *osDisk.ManagedDisk.ID
		}
		if osDisk.DiskSizeGB != nil {
			d.SizeGB = *osDisk.DiskSizeGB
		}
		if osDisk.ManagedDisk != nil && osDisk.ManagedDisk.StorageAccountType != nil {
			d.Sku = string(*osDisk.ManagedDisk.StorageAccountType)
		}
		if osDisk.OSType != nil {
			d.OSType = string(*osDisk.OSType)
		}
		disks = append(disks, d)
	}

	for _, dataDisk := range sp.DataDisks {
		if dataDisk == nil {
			continue
		}
		d := model.VMDisk{}
		if dataDisk.Name != nil {
			d.Name = *dataDisk.Name
		}
		if dataDisk.ManagedDisk != nil && dataDisk.ManagedDisk.ID != nil {
			d.ID = *dataDisk.ManagedDisk.ID
		}
		if dataDisk.DiskSizeGB != nil {
			d.SizeGB = *dataDisk.DiskSizeGB
		}
		if dataDisk.ManagedDisk != nil && dataDisk.ManagedDisk.StorageAccountType != nil {
			d.Sku = string(*dataDisk.ManagedDisk.StorageAccountType)
		}
		disks = append(disks, d)
	}

	return disks
}

// enrichDiskSizes fills in zero-sized disks by looking up the actual
// disk resource from the DB (collected earlier by collectDisks).
func (r *Collector) enrichDiskSizes(vm *model.VM) {
	cache := r.diskNameCache()
	for i := range vm.Disks {
		if vm.Disks[i].SizeGB != 0 {
			continue
		}
		diskName := vm.Disks[i].Name
		if diskName == "" {
			continue
		}
		disk, ok := cache[diskName]
		if !ok {
			continue
		}
		if disk.SizeGB > 0 {
			vm.Disks[i].SizeGB = int32(disk.SizeGB)
			r.log.V(1).Info("Enriched disk size from disk resource",
				"vm", vm.Name, "disk", diskName, "sizeGB", disk.SizeGB)
		}
		if vm.Disks[i].Sku == "" && disk.DiskType != "" {
			vm.Disks[i].Sku = disk.DiskType
		}
	}
}

// diskNameCache loads all disks into a name-keyed map for enrichment.
func (r *Collector) diskNameCache() map[string]model.Disk {
	cache := map[string]model.Disk{}
	var disks []model.Disk
	if err := r.db.List(&disks, libmodel.ListOptions{}); err != nil {
		return cache
	}
	for _, d := range disks {
		cache[d.Name] = d
	}
	return cache
}
