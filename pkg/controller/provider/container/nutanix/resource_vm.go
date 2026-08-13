package nutanix

import (
	"strings"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
)

type vmEntity struct {
	Metadata metadata `json:"metadata"`
	Spec     struct {
		ClusterReference ref         `json:"cluster_reference"`
		Description      string      `json:"description"`
		Name             string      `json:"name"`
		Resources        vmResources `json:"resources"`
	} `json:"spec"`
	Status struct {
		Resources vmResources `json:"resources"`
	} `json:"status"`
}

type vmResources struct {
	BootConfig struct {
		BootDeviceOrderList []string `json:"boot_device_order_list"`
		BootType            string   `json:"boot_type"`
	} `json:"boot_config"`
	DiskList          []diskEntity `json:"disk_list"`
	GuestOSID         string       `json:"guest_os_id"`
	GuestTools        guestTools   `json:"guest_tools"`
	HardwareClockTZ   string       `json:"hardware_clock_timezone"`
	HostReference     ref          `json:"host_reference"`
	HypervisorType    string       `json:"hypervisor_type"`
	MachineType       string       `json:"machine_type"`
	MemorySizeMiB     int64        `json:"memory_size_mib"`
	NICList           []nicEntity  `json:"nic_list"`
	NumSockets        int          `json:"num_sockets"`
	NumThreadsPerCore int          `json:"num_threads_per_core"`
	NumVcpusPerSocket int          `json:"num_vcpus_per_socket"`
	PowerState        string       `json:"power_state"`
	SerialPortList    []serialPort `json:"serial_port_list"`
	VGAConsoleEnabled bool         `json:"vga_console_enabled"`
}

type serialPort struct {
	Index       int  `json:"index"`
	IsConnected bool `json:"is_connected"`
}

type nicEntity struct {
	IPEndpointList []struct {
		IP string `json:"ip"`
	} `json:"ip_endpoint_list"`
	IsConnected     bool   `json:"is_connected"`
	MACAddress      string `json:"mac_address"`
	Model           string `json:"model"`
	NicType         string `json:"nic_type"`
	SubnetReference ref    `json:"subnet_reference"`
	UUID            string `json:"uuid"`
	VlanMode        string `json:"vlan_mode"`
}

type diskEntity struct {
	DataSourceReference ref `json:"data_source_reference"`
	DeviceProperties    struct {
		DeviceType  string `json:"device_type"`
		DiskAddress struct {
			AdapterType string `json:"adapter_type"`
			DeviceIndex int    `json:"device_index"`
		} `json:"disk_address"`
	} `json:"device_properties"`
	DiskSizeBytes int64 `json:"disk_size_bytes"`
	DiskSizeMiB   int64 `json:"disk_size_mib"`
	StorageConfig struct {
		FlashMode                 interface{} `json:"flash_mode"`
		StorageContainerReference ref         `json:"storage_container_reference"`
	} `json:"storage_config"`
	StorageContainerReference ref    `json:"storage_container_reference"`
	UUID                      string `json:"uuid"`
}

type guestTools struct {
	NutanixGuestTools struct {
		Enabled        bool   `json:"enabled"`
		GuestOSVersion string `json:"guest_os_version"`
		ISOMountState  string `json:"iso_mount_state"`
		IsReachable    bool   `json:"is_reachable"`
		Version        string `json:"version"`
	} `json:"nutanix_guest_tools"`
}

func (e vmEntity) id() string {
	return e.Metadata.UUID
}

func (e vmEntity) mergedResources() vmResources {
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
	m.Name = coalesce(e.Spec.Name, e.Metadata.Name)
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
	m.GuestOSID = coalesce(e.Spec.Resources.GuestOSID, e.Status.Resources.GuestOSID)

	m.SerialPorts = applySerialPorts(resources.SerialPortList)
	m.NICs = applyNICs(resources.NICList)
	m.Disks = applyDisks(resources.DiskList)
	mergeGuestTools(e.Spec.Resources.GuestTools, e.Status.Resources.GuestTools, m)
}

func applySerialPorts(ports []serialPort) []model.SerialPort {
	result := make([]model.SerialPort, 0, len(ports))
	for _, port := range ports {
		result = append(result, model.SerialPort{
			Index:       port.Index,
			IsConnected: port.IsConnected,
		})
	}
	return result
}

func applyNICs(nics []nicEntity) []model.NIC {
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

func applyDisks(disks []diskEntity) []model.Disk {
	result := make([]model.Disk, 0, len(disks))
	for _, disk := range disks {
		result = append(result, disk.ApplyTo())
	}
	return result
}

func (d *diskEntity) ApplyTo() model.Disk {
	disk := model.Disk{
		AdapterType:     d.DeviceProperties.DiskAddress.AdapterType,
		DeviceIndex:     d.DeviceProperties.DiskAddress.DeviceIndex,
		DeviceType:      d.DeviceProperties.DeviceType,
		DiskSizeMiB:     d.DiskSizeMiB,
		FlashMode:       nutanixBool(d.StorageConfig.FlashMode),
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

func mergeGuestTools(spec, status guestTools, m *model.VM) {
	for _, tools := range []guestTools{spec, status} {
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
