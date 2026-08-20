package nutanix

import (
	"strconv"
	"strings"
)

// Metadata is the common v3 entity metadata block.
type Metadata struct {
	UUID       string            `json:"uuid"`
	Name       string            `json:"name"`
	Categories map[string]string `json:"categories"`
}

// Ref is a Nutanix v3 UUID/name reference pair.
type Ref struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type v3ListMetadata struct {
	TotalMatches int `json:"total_matches"`
}

// V3ListResponse is a paginated v3 list response.
type V3ListResponse[T any] struct {
	Metadata v3ListMetadata `json:"metadata"`
	Entities []T            `json:"entities"`
}

type v4ListMetadata struct {
	TotalAvailableResults int `json:"totalAvailableResults"`
}

// V4ListResponse is a paginated v4 list response.
type V4ListResponse[T any] struct {
	Metadata v4ListMetadata `json:"metadata"`
	Data     []T            `json:"data"`
}

type v3ListRequest struct {
	Kind   string `json:"kind"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

type v3ListRequestWithFilter struct {
	Kind   string         `json:"kind"`
	Offset int            `json:"offset"`
	Length int            `json:"length"`
	Filter map[string]any `json:"filter,omitempty"`
}

// V3Image is a v3 Image Service entity.
type V3Image struct {
	Metadata Metadata `json:"metadata"`
	Spec     struct {
		Name string `json:"name"`
	} `json:"spec"`
	Status struct {
		Name      string           `json:"name"`
		State     string           `json:"state"`
		Resources V3ImageResources `json:"resources"`
	} `json:"status"`
}

// V3ImageResources is the v3 image status.resources block.
type V3ImageResources struct {
	Architecture string `json:"architecture"`
	ImageType    string `json:"image_type"`
	SizeBytes    int64  `json:"size_bytes"`
	SourceURI    string `json:"source_uri"`
}

// ImageV4 is a v4 Image Service entity.
type ImageV4 struct {
	ExtID     string `json:"extId"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
}

// Cluster is a v3 cluster entity (only fields used by migration callers).
type Cluster struct {
	Status struct {
		Resources struct {
			Network struct {
				ExternalIP string `json:"external_ip"`
			} `json:"network"`
		} `json:"resources"`
	} `json:"status"`
}

// VM is a full v3 VM entity for GET/modify/PUT round-trips.
type VM struct {
	APIVersion string   `json:"api_version,omitempty"`
	Metadata   Metadata `json:"metadata"`
	Spec       VMSpec   `json:"spec"`
	Status     VMStatus `json:"status"`
}

// VMStatus is the v3 VM status block.
type VMStatus struct {
	Resources VMResources `json:"resources"`
}

// VMSpec is the v3 VM spec block.
type VMSpec struct {
	ClusterReference Ref         `json:"cluster_reference"`
	Description      string      `json:"description"`
	Name             string      `json:"name"`
	Resources        VMResources `json:"resources"`
}

// VMResources is the v3 VM resources block (spec and status).
type VMResources struct {
	BootConfig struct {
		BootDeviceOrderList []string `json:"boot_device_order_list"`
		BootType            string   `json:"boot_type"`
	} `json:"boot_config"`
	DiskList          []VMDisk       `json:"disk_list"`
	GuestOSID         string         `json:"guest_os_id"`
	GuestTools        VMGuestTools   `json:"guest_tools"`
	HardwareClockTZ   string         `json:"hardware_clock_timezone"`
	HostReference     Ref            `json:"host_reference"`
	HypervisorType    string         `json:"hypervisor_type"`
	MachineType       string         `json:"machine_type"`
	MemorySizeMiB     int64          `json:"memory_size_mib"`
	NICList           []VMNIC        `json:"nic_list"`
	NumSockets        int            `json:"num_sockets"`
	NumThreadsPerCore int            `json:"num_threads_per_core"`
	NumVcpusPerSocket int            `json:"num_vcpus_per_socket"`
	PowerState        string         `json:"power_state"`
	SerialPortList    []VMSerialPort `json:"serial_port_list"`
	VGAConsoleEnabled bool           `json:"vga_console_enabled"`
}

type VMSerialPort struct {
	Index       int  `json:"index"`
	IsConnected bool `json:"is_connected"`
}

type VMNIC struct {
	IPEndpointList []struct {
		IP string `json:"ip"`
	} `json:"ip_endpoint_list"`
	IsConnected     bool   `json:"is_connected"`
	MACAddress      string `json:"mac_address"`
	Model           string `json:"model"`
	NicType         string `json:"nic_type"`
	SubnetReference Ref    `json:"subnet_reference"`
	UUID            string `json:"uuid"`
	VlanMode        string `json:"vlan_mode"`
}

type VMDisk struct {
	DataSourceReference Ref `json:"data_source_reference"`
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
		FlashMode                 any `json:"flash_mode"`
		StorageContainerReference Ref `json:"storage_container_reference"`
	} `json:"storage_config"`
	StorageContainerReference Ref    `json:"storage_container_reference"`
	UUID                      string `json:"uuid"`
}

type VMGuestTools struct {
	NutanixGuestTools struct {
		Enabled        bool   `json:"enabled"`
		GuestOSVersion string `json:"guest_os_version"`
		ISOMountState  string `json:"iso_mount_state"`
		IsReachable    bool   `json:"is_reachable"`
		Version        string `json:"version"`
	} `json:"nutanix_guest_tools"`
}

// PowerState returns the VM power state, preferring status over spec.
func (v VM) PowerState() string {
	if state := v.Status.Resources.PowerState; state != "" {
		return state
	}
	return v.Spec.Resources.PowerState
}

type vmUpdateRequest struct {
	APIVersion string   `json:"api_version,omitempty"`
	Metadata   Metadata `json:"metadata"`
	Spec       VMSpec   `json:"spec"`
}

// VMUpdateRequest is the v3 VM PUT payload.
type VMUpdateRequest = vmUpdateRequest

type v3ImageCreateRequest struct {
	APIVersion string `json:"api_version"`
	Metadata   struct {
		Kind string `json:"kind"`
	} `json:"metadata"`
	Spec struct {
		Name      string `json:"name"`
		Resources struct {
			ImageType           string          `json:"image_type"`
			DataSourceReference entityReference `json:"data_source_reference"`
		} `json:"resources"`
	} `json:"spec"`
}

type entityReference struct {
	Kind string `json:"kind"`
	UUID string `json:"uuid"`
}

type imageV4CreateRequest struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Source struct {
		ObjectType string `json:"$objectType"`
		ExtID      string `json:"extId"`
	} `json:"source"`
}

// VMUpdateBody builds the v3 VM PUT payload from a fetched VM entity.
func VMUpdateBody(vm VM) vmUpdateRequest {
	return vmUpdateRequest{
		APIVersion: vm.APIVersion,
		Metadata:   vm.Metadata,
		Spec:       vm.Spec,
	}
}

// V3ImageCreateBody builds a v3 image creation request for a VM disk.
func V3ImageCreateBody(name, diskUUID string) v3ImageCreateRequest {
	body := v3ImageCreateRequest{APIVersion: "3.1.0"}
	body.Metadata.Kind = "image"
	body.Spec.Name = name
	body.Spec.Resources.ImageType = "DISK_IMAGE"
	body.Spec.Resources.DataSourceReference = entityReference{
		Kind: "vm_disk",
		UUID: diskUUID,
	}
	return body
}

// ImageV4CreateBody builds a v4 image creation request for a VM disk.
func ImageV4CreateBody(name, diskUUID string) imageV4CreateRequest {
	body := imageV4CreateRequest{
		Name: name,
		Type: "DISK_IMAGE",
	}
	body.Source.ObjectType = "vmm.v4.content.VmDiskSource"
	body.Source.ExtID = diskUUID
	return body
}

// Coalesce returns the first non-empty string.
func Coalesce(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// NutanixBool coerces Nutanix JSON booleans that may arrive as native booleans
// or string values such as flash_mode "ENABLED"/"DISABLED".
func NutanixBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToUpper(strings.TrimSpace(typed)) {
		case "TRUE", "1", "YES", "ON", "ENABLED":
			return true
		case "FALSE", "0", "NO", "OFF", "DISABLED":
			return false
		}
		parsed, err := strconv.ParseBool(typed)
		return err == nil && parsed
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	default:
		return false
	}
}

// ParseNumericString parses Nutanix numeric fields that may arrive as strings.
func ParseNumericString(value any) int64 {
	switch typed := value.(type) {
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err == nil {
			return parsed
		}
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	}
	return 0
}
