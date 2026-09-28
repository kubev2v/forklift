package nutanix

import (
	"strings"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	libclient "github.com/kubev2v/forklift/pkg/lib/client/nutanix"
)

// vmEntity is a v3 VM wire entity mapped into inventory.
type vmEntity libclient.VM

func (e vmEntity) id() string {
	return e.Metadata.UUID
}

func (e vmEntity) mergedResources() libclient.VMResources {
	spec := e.Spec.Resources
	status := e.Status.Resources
	out := spec

	if out.PowerState == "" {
		out.PowerState = status.PowerState
	}
	if len(out.NICList) == 0 {
		out.NICList = status.NICList
	}
	if len(out.DiskList) == 0 {
		out.DiskList = status.DiskList
	}
	return out
}

func (e *vmEntity) ApplyTo(m *model.VM) {
	resources := e.mergedResources()

	m.ID = e.id()
	m.UUID = e.id()
	m.Name = libclient.Coalesce(e.Spec.Name, e.Metadata.Name)
	m.Categories = e.Metadata.Categories
	m.Cluster = e.Spec.ClusterReference.UUID
	m.Host = e.Status.Resources.HostReference.UUID
	m.Description = e.Spec.Description
	m.PowerState = resources.PowerState
	m.NumSockets = resources.NumSockets
	m.NumVcpusPerSocket = resources.NumVcpusPerSocket
	m.NumThreadsPerCore = resources.NumThreadsPerCore
	m.MemorySizeMiB = resources.MemorySizeMiB
	m.BootType = resources.BootConfig.BootType
	m.BootDeviceOrder = strings.Join(resources.BootConfig.BootDeviceOrderList, ",")
	m.MachineType = resources.MachineType
	m.HardwareClockTZ = resources.HardwareClockTZ
	m.VGAConsoleEnabled = resources.VGAConsoleEnabled
	m.HypervisorType = e.Status.Resources.HypervisorType
	m.GuestOSID = libclient.Coalesce(e.Spec.Resources.GuestOSID, e.Status.Resources.GuestOSID)

	m.SerialPorts = applySerialPorts(resources.SerialPortList)
	m.NICs = applyNICs(resources.NICList)
	m.Disks = applyDisks(resources.DiskList)
	mergeGuestTools(e.Spec.Resources.GuestTools, e.Status.Resources.GuestTools, m)
}

func applySerialPorts(ports []libclient.VMSerialPort) []model.SerialPort {
	result := make([]model.SerialPort, 0, len(ports))
	for _, port := range ports {
		result = append(result, model.SerialPort{
			Index:       port.Index,
			IsConnected: port.IsConnected,
		})
	}
	return result
}

func applyNICs(nics []libclient.VMNIC) []model.NIC {
	result := make([]model.NIC, 0, len(nics))
	for _, nic := range nics {
		addresses := make([]string, 0, len(nic.IPEndpointList))
		for _, endpoint := range nic.IPEndpointList {
			if endpoint.IP != "" {
				addresses = append(addresses, endpoint.IP)
			}
		}

		result = append(result, model.NIC{
			IPAddresses: addresses,
			IsConnected: nic.IsConnected,
			MACAddress:  nic.MACAddress,
			Model:       nic.Model,
			NicType:     nic.NicType,
			SubnetName:  nic.SubnetReference.Name,
			SubnetUUID:  nic.SubnetReference.UUID,
			UUID:        nic.UUID,
			VlanMode:    nic.VlanMode,
		})
	}
	return result
}

func applyDisks(disks []libclient.VMDisk) []model.Disk {
	result := make([]model.Disk, 0, len(disks))
	for i := range disks {
		result = append(result, applyDisk(&disks[i]))
	}
	return result
}

func applyDisk(d *libclient.VMDisk) model.Disk {
	disk := model.Disk{
		AdapterType:     d.DeviceProperties.DiskAddress.AdapterType,
		DeviceIndex:     d.DeviceProperties.DiskAddress.DeviceIndex,
		DeviceType:      d.DeviceProperties.DeviceType,
		DiskSizeMiB:     d.DiskSizeMiB,
		FlashMode:       libclient.NutanixBool(d.StorageConfig.FlashMode),
		IsCdrom:         d.DeviceProperties.DeviceType == "CDROM",
		SourceImageUUID: d.DataSourceReference.UUID,
		UUID:            d.UUID,
	}

	if d.DiskSizeMiB > 0 {
		disk.DiskSizeBytes = d.DiskSizeMiB * 1024 * 1024
	}
	if d.DiskSizeBytes > 0 {
		disk.DiskSizeBytes = d.DiskSizeBytes
	}

	container := d.StorageConfig.StorageContainerReference
	if container.UUID == "" {
		container = d.StorageContainerReference
	}
	disk.StorageContainerUUID = container.UUID
	disk.StorageContainerName = container.Name

	return disk
}

func mergeGuestTools(spec, status libclient.VMGuestTools, m *model.VM) {
	for _, tools := range []libclient.VMGuestTools{spec, status} {
		ngt := tools.NutanixGuestTools
		if ngt.Enabled {
			m.GuestToolsEnabled = ngt.Enabled
		}
		if ngt.Version != "" {
			m.GuestToolsVersion = ngt.Version
		}
		if ngt.IsReachable {
			m.GuestToolsReachable = ngt.IsReachable
		}
		if ngt.ISOMountState != "" {
			m.GuestToolsMounted = ngt.ISOMountState == "MOUNTED"
		}
		if ngt.GuestOSVersion != "" {
			m.GuestOSVersion = ngt.GuestOSVersion
		}
	}
}

func enrichVM(m *model.VM, storageNames, networkNames map[string]string) {
	for i := range m.Disks {
		if m.Disks[i].StorageContainerName == "" && m.Disks[i].StorageContainerUUID != "" {
			m.Disks[i].StorageContainerName = storageNames[m.Disks[i].StorageContainerUUID]
		}
	}
	for i := range m.NICs {
		if m.NICs[i].SubnetName == "" && m.NICs[i].SubnetUUID != "" {
			m.NICs[i].SubnetName = networkNames[m.NICs[i].SubnetUUID]
		}
	}
}
