package validator

import (
	"fmt"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/inventory"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/mapping"
)

func (r *Validator) validateStorage(vmRef ref.Ref) (ok bool, err error) {
	azureVM, err := r.getAzureVM(vmRef)
	if err != nil {
		r.log.Error(err, "Failed to get Azure VM from inventory", "vm", vmRef.String())
		return false, err
	}

	diskIDs := inventory.GetManagedDiskIDs(azureVM)
	if len(diskIDs) == 0 {
		r.log.Info("VM has no managed disks", "vm", vmRef.String())
		return false, fmt.Errorf("VM has no managed disks attached - cannot migrate VM without storage")
	}

	r.log.Info("Storage validation passed",
		"vm", vmRef.String(),
		"managedDisks", len(diskIDs))

	return true, nil
}

func (r *Validator) StorageMapped(vmRef ref.Ref) (bool, error) {
	azureVM, err := r.getAzureVM(vmRef)
	if err != nil {
		return false, err
	}

	diskCount := 0
	if azureVM.Properties != nil && azureVM.Properties.StorageProfile != nil {
		// Check OS disk
		if azureVM.Properties.StorageProfile.OSDisk != nil &&
			azureVM.Properties.StorageProfile.OSDisk.ManagedDisk != nil &&
			azureVM.Properties.StorageProfile.OSDisk.ManagedDisk.StorageAccountType != nil {
			sku := string(*azureVM.Properties.StorageProfile.OSDisk.ManagedDisk.StorageAccountType)
			if !mapping.HasStorageMapping(r.Map.Storage, sku) {
				return false, nil
			}
			diskCount++
		}

		// Check data disks
		for _, dataDisk := range azureVM.Properties.StorageProfile.DataDisks {
			if dataDisk.ManagedDisk != nil && dataDisk.ManagedDisk.StorageAccountType != nil {
				sku := string(*dataDisk.ManagedDisk.StorageAccountType)
				if !mapping.HasStorageMapping(r.Map.Storage, sku) {
					return false, nil
				}
				diskCount++
			}
		}
	}

	return true, nil
}

func (r *Validator) UnSupportedDisks(vmRef ref.Ref) ([]string, error) {
	azureVM, err := r.getAzureVM(vmRef)
	if err != nil {
		return nil, err
	}

	var unsupported []string
	if azureVM.Properties != nil && azureVM.Properties.StorageProfile != nil {
		// Check for ephemeral OS disk
		if azureVM.Properties.StorageProfile.OSDisk != nil &&
			azureVM.Properties.StorageProfile.OSDisk.DiffDiskSettings != nil {
			unsupported = append(unsupported, "OS disk (ephemeral)")
		}

		// Check for unmanaged disks
		if azureVM.Properties.StorageProfile.OSDisk != nil &&
			azureVM.Properties.StorageProfile.OSDisk.ManagedDisk == nil &&
			azureVM.Properties.StorageProfile.OSDisk.Vhd != nil {
			unsupported = append(unsupported, "OS disk (unmanaged VHD)")
		}

		for _, dataDisk := range azureVM.Properties.StorageProfile.DataDisks {
			if dataDisk.ManagedDisk == nil && dataDisk.Vhd != nil {
				name := ""
				if dataDisk.Name != nil {
					name = *dataDisk.Name
				}
				unsupported = append(unsupported, fmt.Sprintf("%s (unmanaged VHD)", name))
			}
		}
	}

	return unsupported, nil
}
