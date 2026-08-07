package nutanix

import (
	"strings"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
)

// Helper functions to safely extract values from map[string]interface{}

func getString(m map[string]interface{}, path string) string {
	parts := strings.Split(path, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - get the value
			if val, ok := current[part]; ok {
				if str, ok := val.(string); ok {
					return str
				}
			}
			return ""
		}

		// Navigate deeper
		if val, ok := current[part]; ok {
			if next, ok := val.(map[string]interface{}); ok {
				current = next
			} else {
				return ""
			}
		} else {
			return ""
		}
	}

	return ""
}

func getInt(m map[string]interface{}, path string) int {
	parts := strings.Split(path, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - get the value
			if val, ok := current[part]; ok {
				switch v := val.(type) {
				case int:
					return v
				case int64:
					return int(v)
				case float64:
					return int(v)
				}
			}
			return 0
		}

		// Navigate deeper
		if val, ok := current[part]; ok {
			if next, ok := val.(map[string]interface{}); ok {
				current = next
			} else {
				return 0
			}
		} else {
			return 0
		}
	}

	return 0
}

func getInt64(m map[string]interface{}, path string) int64 {
	parts := strings.Split(path, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - get the value
			if val, ok := current[part]; ok {
				switch v := val.(type) {
				case int64:
					return v
				case int:
					return int64(v)
				case float64:
					return int64(v)
				}
			}
			return 0
		}

		// Navigate deeper
		if val, ok := current[part]; ok {
			if next, ok := val.(map[string]interface{}); ok {
				current = next
			} else {
				return 0
			}
		} else {
			return 0
		}
	}

	return 0
}

func getStringSlice(m map[string]interface{}, path string) []string {
	parts := strings.Split(path, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - get the value
			if val, ok := current[part]; ok {
				if list, ok := val.([]interface{}); ok {
					result := make([]string, 0, len(list))
					for _, item := range list {
						if str, ok := item.(string); ok {
							result = append(result, str)
						}
					}
					return result
				}
			}
			return nil
		}

		// Navigate deeper
		if val, ok := current[part]; ok {
			if next, ok := val.(map[string]interface{}); ok {
				current = next
			} else {
				return nil
			}
		} else {
			return nil
		}
	}

	return nil
}

func getBool(m map[string]interface{}, path string) bool {
	parts := strings.Split(path, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - get the value
			if val, ok := current[part]; ok {
				if b, ok := val.(bool); ok {
					return b
				}
			}
			return false
		}

		// Navigate deeper
		if val, ok := current[part]; ok {
			if next, ok := val.(map[string]interface{}); ok {
				current = next
			} else {
				return false
			}
		} else {
			return false
		}
	}

	return false
}

// Apply cluster data to model.
func applyCluster(entity map[string]interface{}, m *model.Cluster) {
	// metadata
	metadata, _ := entity["metadata"].(map[string]interface{})
	if uuid, ok := metadata["uuid"].(string); ok {
		m.ID = uuid
		m.ClusterUUID = uuid
	}
	// v3 intentful entities carry their name under spec/status, never
	// under metadata.
	m.Name = firstString(entity, "spec.name", "status.name")

	// status.resources
	status, _ := entity["status"].(map[string]interface{})
	resources, _ := status["resources"].(map[string]interface{})

	// config
	config, _ := resources["config"].(map[string]interface{})
	if timezone, ok := config["timezone"].(string); ok {
		m.Timezone = timezone
	}
	if arch, ok := config["cluster_arch"].(string); ok {
		m.ClusterArch = arch
	}
	if opMode, ok := config["operation_mode"].(string); ok {
		m.OperationMode = opMode
	}

	// build version
	if build, ok := config["build"].(map[string]interface{}); ok {
		if version, ok := build["version"].(string); ok {
			m.Version = version
		}
		if fullVersion, ok := build["full_version"].(string); ok {
			m.BuildVersion = fullVersion
		}
	}

	// network
	network, _ := resources["network"].(map[string]interface{})
	if externalIP, ok := network["external_ip"].(string); ok {
		m.ExternalIP = externalIP
	}

	// nodes count
	nodes, _ := resources["nodes"].(map[string]interface{})
	if hypervisors, ok := nodes["hypervisor_server_list"].([]interface{}); ok {
		m.NumNodes = len(hypervisors)
	}

	// analysis
	analysis, _ := resources["analysis"].(map[string]interface{})
	if vmCount, ok := analysis["vm_count"].(float64); ok {
		m.VMCount = int64(vmCount)
	} else if vmCount, ok := analysis["vm_count"].(int); ok {
		m.VMCount = int64(vmCount)
	}

	storage, _ := analysis["storage_summary"].(map[string]interface{})
	if totalCap, ok := storage["total_capacity_bytes"].(float64); ok {
		m.TotalCapacity = int64(totalCap)
	} else if totalCap, ok := storage["total_capacity_bytes"].(int64); ok {
		m.TotalCapacity = totalCap
	}
	if usedCap, ok := storage["usage_bytes"].(float64); ok {
		m.UsedCapacity = int64(usedCap)
	} else if usedCap, ok := storage["usage_bytes"].(int64); ok {
		m.UsedCapacity = usedCap
	}
}

// Apply host data to model.
func applyHost(entity map[string]interface{}, m *model.Host) {
	metadata, _ := entity["metadata"].(map[string]interface{})
	if uuid, ok := metadata["uuid"].(string); ok {
		m.ID = uuid
		m.HostUUID = uuid
	}
	// v3 intentful entities carry their name under spec/status, never
	// under metadata.
	m.Name = firstString(entity, "spec.name", "status.name")

	status, _ := entity["status"].(map[string]interface{})
	resources, _ := status["resources"].(map[string]interface{})

	// cluster reference lives directly under spec/status, not nested
	// under status.resources.
	m.Cluster = firstString(entity, "spec.cluster_reference.uuid", "status.cluster_reference.uuid")

	// serial number
	m.SerialNumber = getString(resources, "serial_number")
	m.BlockModel = getString(resources, "block.block_model")

	// hypervisor info -- unlike VMs, hosts have no dedicated type-enum
	// field (no "type"/"hypervisor_type" key); only a free-text
	// "hypervisor_full_name" is available (e.g. "Nutanix 20240802.100").
	if hvState, ok := resources["hypervisor"].(map[string]interface{}); ok {
		m.HypervisorType = getString(hvState, "hypervisor_full_name")
		if numVMs, ok := hvState["num_vms"].(float64); ok {
			m.NumVMs = int(numVMs)
		} else if numVMs, ok := hvState["num_vms"].(int); ok {
			m.NumVMs = numVMs
		}
	}

	// State (entity lifecycle status, e.g. "COMPLETE") lives directly
	// under status, not status.resources. HostType (topology, e.g.
	// "HYPER_CONVERGED") is a distinct field under status.resources.
	m.State = getString(status, "state")
	m.HostType = getString(resources, "host_type")

	// CPU info
	m.CPUModel = getString(resources, "cpu_model")
	m.CPUCapacityHz = getInt64(resources, "cpu_capacity_hz")
	m.NumCpuSockets = getInt(resources, "num_cpu_sockets")
	m.NumCpuCores = getInt(resources, "num_cpu_cores")
	m.NumCpuThreads = getInt(resources, "num_cpu_threads")

	// Memory
	m.MemoryCapacityMiB = getInt64(resources, "memory_capacity_mib")

	// Management
	m.IPMIAddress = getString(resources, "ipmi.ip")
}

// Apply network data to model.
func applyNetwork(entity map[string]interface{}, m *model.Network) {
	metadata, _ := entity["metadata"].(map[string]interface{})
	if uuid, ok := metadata["uuid"].(string); ok {
		m.ID = uuid
		m.NetworkUUID = uuid
	}
	// v3 intentful entities carry their name under spec/status, never
	// under metadata.
	m.Name = firstString(entity, "spec.name", "status.name")

	status, _ := entity["status"].(map[string]interface{})
	resources, _ := status["resources"].(map[string]interface{})

	// cluster reference lives directly under spec/status, not nested
	// under status.resources.
	m.Cluster = firstString(entity, "spec.cluster_reference.uuid", "status.cluster_reference.uuid")

	m.SubnetType = getString(resources, "subnet_type")
	m.VlanID = getInt(resources, "vlan_id")

	// IP config
	if ipConfig, ok := resources["ip_config"].(map[string]interface{}); ok {
		if subnetIP, ok := ipConfig["subnet_ip"].(string); ok {
			m.NetworkAddress = subnetIP
		}
		if prefixLen, ok := ipConfig["prefix_length"].(float64); ok {
			m.PrefixLength = int(prefixLen)
		} else if prefixLen, ok := ipConfig["prefix_length"].(int); ok {
			m.PrefixLength = prefixLen
		}
		if gateway, ok := ipConfig["default_gateway_ip"].(string); ok {
			m.DefaultGateway = gateway
		}

		// DHCP info
		if dhcpOptions, ok := ipConfig["dhcp_options"].(map[string]interface{}); ok {
			if dhcpIP, ok := dhcpOptions["dhcp_server_address"].(string); ok {
				m.DHCPServerIP = dhcpIP
			}
			if domainName, ok := dhcpOptions["domain_name"].(string); ok {
				m.DHCPDomainName = domainName
			}
		}

		// Pool ranges
		if pools, ok := ipConfig["pool_list"].([]interface{}); ok {
			ranges := make([]string, 0, len(pools))
			for _, p := range pools {
				if pool, ok := p.(map[string]interface{}); ok {
					if start, ok := pool["range"].(string); ok {
						ranges = append(ranges, start)
					}
				}
			}
			m.IPPoolRanges = strings.Join(ranges, ",")
		}
	}
}

// Apply storage container data to model.
func applyStorageContainer(entity map[string]interface{}, m *model.StorageContainer) {
	metadata, _ := entity["metadata"].(map[string]interface{})
	if uuid, ok := metadata["uuid"].(string); ok {
		m.ID = uuid
		m.StorageContainerUUID = uuid
	}
	if name, ok := metadata["name"].(string); ok {
		m.Name = name
	}

	status, _ := entity["status"].(map[string]interface{})
	resources, _ := status["resources"].(map[string]interface{})

	// cluster reference
	if clusterRef, ok := resources["cluster_reference"].(map[string]interface{}); ok {
		if clusterUUID, ok := clusterRef["uuid"].(string); ok {
			m.Cluster = clusterUUID
		}
	}

	m.ReplicationFactor = getInt(resources, "replication_factor")

	// Capacity
	m.MaxCapacityBytes = getInt64(resources, "max_capacity_bytes")
	m.UsageBytes = getInt64(resources, "usage_bytes")
	if m.MaxCapacityBytes > 0 {
		m.FreeBytes = m.MaxCapacityBytes - m.UsageBytes
	}

	// Features
	m.CompressionEnabled = getBool(resources, "compression_enabled")
	m.OnDiskDedup = getString(resources, "on_disk_dedup")
	m.ErasureCode = getString(resources, "erasure_code")
}

// Apply image data to model.
func applyImage(entity map[string]interface{}, m *model.Image) {
	metadata, _ := entity["metadata"].(map[string]interface{})
	if uuid, ok := metadata["uuid"].(string); ok {
		m.ID = uuid
		m.ImageUUID = uuid
	}
	// v3 intentful entities carry their name under spec/status, never
	// under metadata.
	m.Name = firstString(entity, "spec.name", "status.name")

	status, _ := entity["status"].(map[string]interface{})
	resources, _ := status["resources"].(map[string]interface{})

	m.ImageType = getString(resources, "image_type")
	m.SizeBytes = getInt64(resources, "size_bytes")
	m.Architecture = getString(resources, "architecture")
	m.SourceURI = getString(resources, "source_uri")
}

// Apply VM data to model.
func applyVM(entity map[string]interface{}, m *model.VM) {
	metadata, _ := entity["metadata"].(map[string]interface{})
	if uuid, ok := metadata["uuid"].(string); ok {
		m.ID = uuid
		m.UUID = uuid
	}

	// Categories
	if categories, ok := metadata["categories"].(map[string]interface{}); ok {
		m.Categories = make(map[string]string)
		for k, v := range categories {
			if str, ok := v.(string); ok {
				m.Categories[k] = str
			}
		}
	}

	// Get both spec and status sections
	spec, _ := entity["spec"].(map[string]interface{})
	specResources, _ := spec["resources"].(map[string]interface{})
	status, _ := entity["status"].(map[string]interface{})
	statusResources, _ := status["resources"].(map[string]interface{})

	if name, ok := spec["name"].(string); ok {
		m.Name = name
	}

	// Cluster reference from spec
	if clusterRef, ok := spec["cluster_reference"].(map[string]interface{}); ok {
		if clusterUUID, ok := clusterRef["uuid"].(string); ok {
			m.Cluster = clusterUUID
		}
	}

	// Host reference from status
	if hostRef, ok := statusResources["host_reference"].(map[string]interface{}); ok {
		if hostUUID, ok := hostRef["uuid"].(string); ok {
			m.Host = hostUUID
		}
	}

	// Description from spec
	if desc, ok := spec["description"].(string); ok {
		m.Description = desc
	}

	// Power state from spec or status
	m.PowerState = getString(specResources, "power_state")
	if m.PowerState == "" {
		m.PowerState = getString(statusResources, "power_state")
	}

	// CPU and memory from spec.resources
	m.NumSockets = getInt(specResources, "num_sockets")
	m.NumVcpusPerSocket = getInt(specResources, "num_vcpus_per_socket")
	m.NumThreadsPerCore = getInt(specResources, "num_threads_per_core")
	m.MemorySizeMiB = getInt64(specResources, "memory_size_mib")

	// Boot config from spec.resources
	if bootConfig, ok := specResources["boot_config"].(map[string]interface{}); ok {
		if bootType, ok := bootConfig["boot_type"].(string); ok {
			m.BootType = bootType
		}
		if bootOrder, ok := bootConfig["boot_device_order_list"].([]interface{}); ok {
			devices := make([]string, 0, len(bootOrder))
			for _, d := range bootOrder {
				if dev, ok := d.(string); ok {
					devices = append(devices, dev)
				}
			}
			m.BootDeviceOrder = strings.Join(devices, ",")
		}
	}

	// Machine config from spec.resources
	m.MachineType = getString(specResources, "machine_type")
	m.HardwareClockTZ = getString(specResources, "hardware_clock_timezone")
	m.VGAConsoleEnabled = getBool(specResources, "vga_console_enabled")

	// Hypervisor type from status.resources
	m.HypervisorType = getString(statusResources, "hypervisor_type")
	m.GuestOSID = getString(specResources, "guest_os_id")
	if m.GuestOSID == "" {
		m.GuestOSID = getString(statusResources, "guest_os_id")
	}

	// Serial ports from spec.resources
	if serialPorts, ok := specResources["serial_port_list"].([]interface{}); ok {
		m.SerialPorts = make([]model.SerialPort, 0, len(serialPorts))
		for _, sp := range serialPorts {
			if port, ok := sp.(map[string]interface{}); ok {
				serialPort := model.SerialPort{}
				if index, ok := port["index"].(float64); ok {
					serialPort.Index = int(index)
				} else if index, ok := port["index"].(int); ok {
					serialPort.Index = index
				}
				if connected, ok := port["is_connected"].(bool); ok {
					serialPort.IsConnected = connected
				}
				m.SerialPorts = append(m.SerialPorts, serialPort)
			}
		}
	}

	// NICs from spec.resources (has complete info), fall back to status if needed
	nicList := specResources["nic_list"]
	if nicList == nil {
		nicList = statusResources["nic_list"]
	}
	if nics, ok := nicList.([]interface{}); ok {
		m.NICs = make([]model.NIC, 0, len(nics))
		for _, n := range nics {
			if nicData, ok := n.(map[string]interface{}); ok {
				nic := model.NIC{}
				nic.UUID = getString(nicData, "uuid")
				nic.NicType = getString(nicData, "nic_type")
				nic.MACAddress = getString(nicData, "mac_address")
				nic.Model = getString(nicData, "model")
				nic.IsConnected = getBool(nicData, "is_connected")

				// Subnet reference
				if subnetRef, ok := nicData["subnet_reference"].(map[string]interface{}); ok {
					if subnetUUID, ok := subnetRef["uuid"].(string); ok {
						nic.SubnetUUID = subnetUUID
					}
					if subnetName, ok := subnetRef["name"].(string); ok {
						nic.SubnetName = subnetName
					}
				}

				// IP addresses
				if ips, ok := nicData["ip_endpoint_list"].([]interface{}); ok {
					nic.IPAddresses = make([]string, 0, len(ips))
					for _, ip := range ips {
						if ipMap, ok := ip.(map[string]interface{}); ok {
							if ipAddr, ok := ipMap["ip"].(string); ok {
								nic.IPAddresses = append(nic.IPAddresses, ipAddr)
							}
						}
					}
				}

				nic.VlanMode = getString(nicData, "vlan_mode")
				m.NICs = append(m.NICs, nic)
			}
		}
	}

	// Disks from spec.resources, fall back to status.resources
	diskList := specResources["disk_list"]
	if diskList == nil {
		diskList = statusResources["disk_list"]
	}
	if disks, ok := diskList.([]interface{}); ok {
		m.Disks = make([]model.Disk, 0, len(disks))
		for _, d := range disks {
			if diskData, ok := d.(map[string]interface{}); ok {
				m.Disks = append(m.Disks, applyDiskFromMap(diskData))
			}
		}
	}

	applyGuestTools(specResources, statusResources, m)
}

func applyGuestTools(specResources, statusResources map[string]interface{}, m *model.VM) {
	for _, resources := range []map[string]interface{}{specResources, statusResources} {
		guestTools, ok := resources["guest_tools"].(map[string]interface{})
		if !ok {
			continue
		}
		ngt, ok := guestTools["nutanix_guest_tools"].(map[string]interface{})
		if !ok {
			continue
		}
		if enabled, ok := ngt["enabled"].(bool); ok {
			m.GuestToolsEnabled = enabled
		}
		if v, ok := ngt["version"].(string); ok && v != "" {
			m.GuestToolsVersion = v
		}
		if reachable, ok := ngt["is_reachable"].(bool); ok {
			m.GuestToolsReachable = reachable
		}
		if mounted, ok := ngt["iso_mount_state"].(string); ok {
			m.GuestToolsMounted = mounted == "MOUNTED"
		}
		if guestOSVersion, ok := ngt["guest_os_version"].(string); ok && guestOSVersion != "" {
			m.GuestOSVersion = guestOSVersion
		}
	}
}

func applyDiskFromMap(diskData map[string]interface{}) model.Disk {
	disk := model.Disk{}
	disk.UUID = getString(diskData, "uuid")
	disk.DeviceType = getString(diskData, "device_properties.device_type")
	disk.AdapterType = getString(diskData, "device_properties.disk_address.adapter_type")

	if deviceProps, ok := diskData["device_properties"].(map[string]interface{}); ok {
		if diskAddr, ok := deviceProps["disk_address"].(map[string]interface{}); ok {
			if idx, ok := diskAddr["device_index"].(float64); ok {
				disk.DeviceIndex = int(idx)
			} else if idx, ok := diskAddr["device_index"].(int); ok {
				disk.DeviceIndex = idx
			}
		}
	}

	if diskSizeMib, ok := diskData["disk_size_mib"].(float64); ok {
		disk.DiskSizeMiB = int64(diskSizeMib)
		disk.DiskSizeBytes = int64(diskSizeMib) * 1024 * 1024
	} else if diskSizeMib, ok := diskData["disk_size_mib"].(int64); ok {
		disk.DiskSizeMiB = diskSizeMib
		disk.DiskSizeBytes = diskSizeMib * 1024 * 1024
	}
	if diskSizeBytes, ok := diskData["disk_size_bytes"].(float64); ok {
		disk.DiskSizeBytes = int64(diskSizeBytes)
	} else if diskSizeBytes, ok := diskData["disk_size_bytes"].(int64); ok {
		disk.DiskSizeBytes = diskSizeBytes
	}

	if storageConfig, ok := diskData["storage_config"].(map[string]interface{}); ok {
		applyStorageContainerRef(storageConfig, &disk)
		disk.FlashMode = getBool(storageConfig, "flash_mode")
	}
	if disk.StorageContainerUUID == "" {
		applyStorageContainerRef(diskData, &disk)
	}

	if imgRef, ok := diskData["data_source_reference"].(map[string]interface{}); ok {
		if imgUUID, ok := imgRef["uuid"].(string); ok {
			disk.SourceImageUUID = imgUUID
		}
	}

	disk.IsCdrom = disk.DeviceType == "CDROM"
	return disk
}

func applyStorageContainerRef(data map[string]interface{}, disk *model.Disk) {
	scRef, ok := data["storage_container_reference"].(map[string]interface{})
	if !ok {
		return
	}
	if scUUID, ok := scRef["uuid"].(string); ok {
		disk.StorageContainerUUID = scUUID
	}
	if scName, ok := scRef["name"].(string); ok {
		disk.StorageContainerName = scName
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
