package collector

import (
	"context"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

type vmSizeSpec struct {
	CpuCount int32
	MemoryMB int32
}

func (r *Collector) collectVMs(ctx context.Context) error {
	vms, err := r.client.ListVirtualMachines(ctx)
	if err != nil {
		return err
	}

	r.log.V(1).Info("Collected VMs", "count", len(vms))

	vmSizes := r.loadVMSizeMap(ctx, vms)

	var created, updated, unchanged int
	for _, azureVM := range vms {
		if azureVM == nil || azureVM.ID == nil {
			continue
		}

		m := &model.VM{}
		m.UID = *azureVM.ID

		if azureVM.Name != nil {
			m.Name = *azureVM.Name
		} else {
			m.Name = m.UID
		}

		m.Kind = "azure-vm"
		m.Provider = string(r.provider.UID)
		m.VMSize = getVMSize(azureVM)
		m.PowerState = r.fetchPowerState(ctx, azureVM)
		m.OSType = getOSType(azureVM)
		m.GuestId = buildGuestId(azureVM)
		m.Disks = buildDisks(azureVM)

		if spec, ok := vmSizes[m.VMSize]; ok {
			m.CpuCount = spec.CpuCount
			m.MemoryMB = spec.MemoryMB
		}

		m.Object = *azureVM

		r.enrichDiskSizes(m)

		existing := &model.VM{}
		existing.UID = m.UID
		if err := r.db.Get(existing); err == nil {
			if !existing.HasChanged(m) {
				unchanged++
				continue
			}
			m.Revision = existing.Revision + 1
			if err := r.db.Update(m); err != nil {
				r.log.Error(err, "Failed to update VM", "vmId", m.UID)
				continue
			}
			updated++
		} else {
			m.Revision = 1
			if err := r.db.Insert(m); err != nil {
				r.log.Error(err, "Failed to insert VM", "vmId", m.UID)
				continue
			}
			created++
		}
	}

	r.log.V(1).Info("VMs processed", "created", created, "updated", updated, "unchanged", unchanged)
	return nil
}

// loadVMSizeMap fetches VM size metadata for each distinct location
// used by the VMs and builds a vmSize-name -> spec lookup.
func (r *Collector) loadVMSizeMap(ctx context.Context, vms []*armcompute.VirtualMachine) map[string]vmSizeSpec {
	locations := map[string]bool{}
	for _, vm := range vms {
		if vm != nil && vm.Location != nil {
			locations[strings.ToLower(*vm.Location)] = true
		}
	}

	result := map[string]vmSizeSpec{}
	for loc := range locations {
		sizes, err := r.client.ListVMSizes(ctx, loc)
		if err != nil {
			r.log.Error(err, "Failed to list VM sizes", "location", loc)
			continue
		}
		for _, s := range sizes {
			if s.Name == nil {
				continue
			}
			spec := vmSizeSpec{}
			if s.NumberOfCores != nil {
				spec.CpuCount = *s.NumberOfCores
			}
			if s.MemoryInMB != nil {
				spec.MemoryMB = *s.MemoryInMB
			}
			result[*s.Name] = spec
		}
	}
	return result
}

// fetchPowerState retrieves the VM instance view to get the actual
// power state (running, deallocated, stopped, etc.).
func (r *Collector) fetchPowerState(ctx context.Context, vm *armcompute.VirtualMachine) string {
	if vm.Properties != nil && vm.Properties.InstanceView != nil {
		if ps := extractPowerState(vm.Properties.InstanceView.Statuses); ps != "" {
			return ps
		}
	}

	if vm.Name == nil {
		return "unknown"
	}
	iv, err := r.client.GetVMInstanceView(ctx, *vm.Name)
	if err != nil {
		r.log.V(2).Info("Failed to get instance view, powerState will be unknown", "vm", *vm.Name, "error", err)
		return "unknown"
	}
	if ps := extractPowerState(iv.Statuses); ps != "" {
		return ps
	}
	return "unknown"
}

func extractPowerState(statuses []*armcompute.InstanceViewStatus) string {
	for _, status := range statuses {
		if status.Code != nil && strings.HasPrefix(*status.Code, "PowerState/") {
			return strings.TrimPrefix(*status.Code, "PowerState/")
		}
	}
	return ""
}

func getVMSize(vm *armcompute.VirtualMachine) string {
	if vm.Properties != nil && vm.Properties.HardwareProfile != nil && vm.Properties.HardwareProfile.VMSize != nil {
		return string(*vm.Properties.HardwareProfile.VMSize)
	}
	return ""
}

func getOSType(vm *armcompute.VirtualMachine) string {
	if vm.Properties == nil || vm.Properties.StorageProfile == nil || vm.Properties.StorageProfile.OSDisk == nil {
		return "linux"
	}
	if vm.Properties.StorageProfile.OSDisk.OSType != nil {
		osType := string(*vm.Properties.StorageProfile.OSDisk.OSType)
		return strings.ToLower(osType)
	}
	return "linux"
}
