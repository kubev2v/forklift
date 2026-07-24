package nutanix

import (
	"encoding/json"
	"os"
	"testing"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	libclient "github.com/kubev2v/forklift/pkg/lib/client/nutanix"
)

func TestApplyCluster(t *testing.T) {
	data, err := os.ReadFile("testdata/clusters_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var response struct {
		Entities []clusterEntity `json:"entities"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	if len(response.Entities) == 0 {
		t.Fatal("No entities in response")
	}

	m := &model.Cluster{}
	response.Entities[0].ApplyTo(m)

	if m.ID == "" {
		t.Error("Expected ID to be set")
	}
	if m.ClusterUUID == "" {
		t.Error("Expected ClusterUUID to be set")
	}
	if m.Name != "prod-cluster-01" {
		t.Errorf("Expected name 'prod-cluster-01', got %s", m.Name)
	}
	if m.Version != "6.8.2" {
		t.Errorf("Expected version '6.8.2', got %s", m.Version)
	}
	if m.Timezone != "America/Los_Angeles" {
		t.Errorf("Expected timezone 'America/Los_Angeles', got %s", m.Timezone)
	}
	if m.NumNodes != 2 {
		t.Errorf("Expected 2 nodes, got %d", m.NumNodes)
	}
	if m.VMCount != 25 {
		t.Errorf("Expected 25 VMs, got %d", m.VMCount)
	}
	if m.TotalCapacity == 0 {
		t.Error("Expected TotalCapacity to be set")
	}
	if m.UsedCapacity == 0 {
		t.Error("Expected UsedCapacity to be set")
	}
}

// TestApplyClusterNameNotFromMetadata verifies that Cluster.Name is read from
// spec/status, not metadata -- v3 intentful entities never carry "name" under
// metadata, only under spec/status. Also verifies the status.name fallback.
func TestApplyClusterNameNotFromMetadata(t *testing.T) {
	e := clusterEntity{}
	e.Metadata.UUID = "cluster-1"
	e.Metadata.Name = "wrong-name"
	e.Status.Name = "right-name"

	m := &model.Cluster{}
	e.ApplyTo(m)

	if m.Name != "right-name" {
		t.Errorf("Expected name 'right-name' from status, got %s", m.Name)
	}
}

func TestApplyHost(t *testing.T) {
	data, err := os.ReadFile("testdata/hosts_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var response struct {
		Entities []hostEntity `json:"entities"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	if len(response.Entities) == 0 {
		t.Fatal("No entities in response")
	}

	m := &model.Host{}
	response.Entities[0].ApplyTo(m)

	if m.ID == "" {
		t.Error("Expected ID to be set")
	}
	if m.HostUUID == "" {
		t.Error("Expected HostUUID to be set")
	}
	if m.Name != "ahv-node-01" {
		t.Errorf("Expected name 'ahv-node-01', got %s", m.Name)
	}
	if m.Cluster != "0005e123-4567-89ab-cdef-000000000001" {
		t.Errorf("Expected Cluster '0005e123-4567-89ab-cdef-000000000001', got %s", m.Cluster)
	}
	if m.CPUModel == "" {
		t.Error("Expected CPUModel to be set")
	}
	if m.NumCpuSockets == 0 {
		t.Error("Expected NumCpuSockets to be > 0")
	}
	if m.NumCpuCores == 0 {
		t.Error("Expected NumCpuCores to be > 0")
	}
	if m.MemoryCapacityMiB == 0 {
		t.Error("Expected MemoryCapacityMiB to be > 0")
	}
	if m.HypervisorType != "Nutanix 20240802.100" {
		t.Errorf("Expected HypervisorType 'Nutanix 20240802.100', got %s", m.HypervisorType)
	}
	if m.NumVMs != 15 {
		t.Errorf("Expected NumVMs 15, got %d", m.NumVMs)
	}
	if m.State != "COMPLETE" {
		t.Errorf("Expected State 'COMPLETE', got %s", m.State)
	}
	if m.HostType != "HYPER_CONVERGED" {
		t.Errorf("Expected HostType 'HYPER_CONVERGED', got %s", m.HostType)
	}
}

// TestApplyHostNameAndClusterNotFromWrongPaths verifies Host.Name is read from
// spec/status (not metadata) and Host.Cluster is read from the top-level
// spec/status.cluster_reference, not from status.resources.cluster_reference.
func TestApplyHostNameAndClusterNotFromWrongPaths(t *testing.T) {
	e := hostEntity{}
	e.Metadata.UUID = "host-1"
	e.Metadata.Name = "wrong-name"
	e.Status.Name = "right-name"
	e.Status.ClusterReference = libclient.Ref{UUID: "right-cluster"}
	// status.Resources has no ClusterReference field in the typed struct,
	// so wrong-path reads are impossible by construction.

	m := &model.Host{}
	e.ApplyTo(m)

	if m.Name != "right-name" {
		t.Errorf("Expected name 'right-name' from status, got %s", m.Name)
	}
	if m.Cluster != "right-cluster" {
		t.Errorf("Expected cluster 'right-cluster' from top-level status, got %s", m.Cluster)
	}
}

func TestApplyHostHypervisorTypeAndState(t *testing.T) {
	e := hostEntity{}
	e.Metadata.UUID = "host-1"
	e.Status.State = "COMPLETE"
	e.Status.Resources.Hypervisor.HypervisorFullName = "Nutanix 20240802.100"
	e.Status.Resources.HostType = "HYPER_CONVERGED"

	m := &model.Host{}
	e.ApplyTo(m)

	if m.HypervisorType != "Nutanix 20240802.100" {
		t.Errorf("Expected HypervisorType 'Nutanix 20240802.100', got %s", m.HypervisorType)
	}
	if m.State != "COMPLETE" {
		t.Errorf("Expected State 'COMPLETE', got %s", m.State)
	}
	if m.HostType != "HYPER_CONVERGED" {
		t.Errorf("Expected HostType 'HYPER_CONVERGED', got %s", m.HostType)
	}
}

func TestApplyNetwork(t *testing.T) {
	data, err := os.ReadFile("testdata/subnets_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var response struct {
		Entities []networkEntity `json:"entities"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	if len(response.Entities) == 0 {
		t.Fatal("No entities in response")
	}

	m := &model.Network{}
	response.Entities[0].ApplyTo(m)

	if m.ID == "" {
		t.Error("Expected ID to be set")
	}
	if m.NetworkUUID == "" {
		t.Error("Expected NetworkUUID to be set")
	}
	if m.Name != "Production-VLAN-100" {
		t.Errorf("Expected name 'Production-VLAN-100', got %s", m.Name)
	}
	if m.Cluster != "0005e123-4567-89ab-cdef-000000000001" {
		t.Errorf("Expected Cluster '0005e123-4567-89ab-cdef-000000000001', got %s", m.Cluster)
	}
	if m.SubnetType != "VLAN" {
		t.Errorf("Expected subnet type 'VLAN', got %s", m.SubnetType)
	}
	if m.VlanID == 0 {
		t.Error("Expected VlanID to be > 0")
	}
	if m.NetworkAddress == "" {
		t.Error("Expected NetworkAddress to be set")
	}
	if m.PrefixLength == 0 {
		t.Error("Expected PrefixLength to be > 0")
	}
}

// TestApplyNetworkNameAndClusterNotFromWrongPaths verifies Network.Name is
// read from spec/status (not metadata) and Network.Cluster is read from the
// top-level spec/status.cluster_reference.
func TestApplyNetworkNameAndClusterNotFromWrongPaths(t *testing.T) {
	e := networkEntity{}
	e.Metadata.UUID = "network-1"
	e.Metadata.Name = "wrong-name"
	e.Status.Name = "right-name"
	e.Status.ClusterReference = libclient.Ref{UUID: "right-cluster"}

	m := &model.Network{}
	e.ApplyTo(m)

	if m.Name != "right-name" {
		t.Errorf("Expected name 'right-name' from status, got %s", m.Name)
	}
	if m.Cluster != "right-cluster" {
		t.Errorf("Expected cluster 'right-cluster' from top-level status, got %s", m.Cluster)
	}
}

func TestApplyStorageContainer(t *testing.T) {
	data, err := os.ReadFile("testdata/storage_containers_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var response struct {
		Entities []storageContainerEntity `json:"entities"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	if len(response.Entities) == 0 {
		t.Fatal("No entities in response")
	}

	m := &model.StorageContainer{}
	response.Entities[0].ApplyTo(m)

	if m.ID == "" {
		t.Error("Expected ID to be set")
	}
	if m.StorageContainerUUID == "" {
		t.Error("Expected StorageContainerUUID to be set")
	}
	if m.Name != "default-container-prod" {
		t.Errorf("Expected name 'default-container-prod', got %s", m.Name)
	}
	if m.Cluster == "" {
		t.Error("Expected Cluster to be set")
	}
	if m.ReplicationFactor == 0 {
		t.Error("Expected ReplicationFactor to be > 0")
	}
	if m.MaxCapacityBytes == 0 {
		t.Error("Expected MaxCapacityBytes to be > 0")
	}
}

func TestApplyImage(t *testing.T) {
	data, err := os.ReadFile("testdata/images_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var response struct {
		Entities []imageEntity `json:"entities"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	if len(response.Entities) == 0 {
		t.Fatal("No entities in response")
	}

	m := &model.Image{}
	response.Entities[0].ApplyTo(m)

	if m.ID == "" {
		t.Error("Expected ID to be set")
	}
	if m.ImageUUID == "" {
		t.Error("Expected ImageUUID to be set")
	}
	if m.Name == "" {
		t.Error("Expected name to be set")
	}
	if m.ImageType == "" {
		t.Error("Expected ImageType to be set")
	}
	if m.SizeBytes == 0 {
		t.Error("Expected SizeBytes to be > 0")
	}
}

func TestApplyVM(t *testing.T) {
	data, err := os.ReadFile("testdata/vms_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var response struct {
		Entities []vmEntity `json:"entities"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	if len(response.Entities) == 0 {
		t.Fatal("No entities in response")
	}

	m := &model.VM{}
	response.Entities[0].ApplyTo(m)

	if m.ID == "" {
		t.Error("Expected ID to be set")
	}
	if m.UUID == "" {
		t.Error("Expected UUID to be set")
	}
	if m.Name != "web-server-rhel8" {
		t.Errorf("Expected name 'web-server-rhel8', got %s", m.Name)
	}
	if m.Cluster == "" {
		t.Error("Expected Cluster to be set")
	}
	if m.NumSockets != 2 {
		t.Errorf("Expected NumSockets to be 2, got %d", m.NumSockets)
	}
	if m.NumVcpusPerSocket != 2 {
		t.Errorf("Expected NumVcpusPerSocket to be 2, got %d", m.NumVcpusPerSocket)
	}
	if m.MemorySizeMiB != 8192 {
		t.Errorf("Expected MemorySizeMiB to be 8192, got %d", m.MemorySizeMiB)
	}
	if m.PowerState != "ON" {
		t.Errorf("Expected PowerState to be 'ON', got %s", m.PowerState)
	}
	if len(m.NICs) == 0 {
		t.Error("Expected at least one NIC")
	} else {
		if m.NICs[0].MACAddress == "" {
			t.Error("Expected NIC MAC address to be set")
		}
		if m.NICs[0].SubnetUUID == "" {
			t.Error("Expected NIC subnet UUID to be set")
		}
	}
	if len(m.Disks) == 0 {
		t.Error("Expected at least one disk")
	} else {
		if m.Disks[0].UUID == "" {
			t.Error("Expected disk UUID to be set")
		}
		if m.Disks[0].DiskSizeMiB == 0 {
			t.Error("Expected disk size to be > 0")
		}
		if m.Disks[0].StorageContainerUUID == "" {
			t.Error("Expected storage container UUID to be set")
		}
	}
	if m.BootType != "UEFI" {
		t.Errorf("Expected BootType to be 'UEFI', got %s", m.BootType)
	}
	if !m.GuestToolsEnabled {
		t.Error("Expected GuestToolsEnabled to be true")
	}
	if m.GuestToolsVersion != "3.2.0" {
		t.Errorf("Expected GuestToolsVersion '3.2.0', got %s", m.GuestToolsVersion)
	}
	if m.Disks[0].StorageContainerName != "default-container-prod" {
		t.Errorf("Expected storage container name 'default-container-prod', got %s", m.Disks[0].StorageContainerName)
	}
}

func TestApplyVMDetail(t *testing.T) {
	data, err := os.ReadFile("testdata/vm_detail_example.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var e vmEntity
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	m := &model.VM{}
	e.ApplyTo(m)

	if m.GuestOSID != "rhel8_64Guest" {
		t.Errorf("Expected GuestOSID 'rhel8_64Guest', got %s", m.GuestOSID)
	}
	if m.GuestOSVersion != "Red Hat Enterprise Linux 8.9 (Ootpa)" {
		t.Errorf("Expected GuestOSVersion to be set, got %s", m.GuestOSVersion)
	}
	if m.HypervisorType != "AHV" {
		t.Errorf("Expected HypervisorType 'AHV', got %s", m.HypervisorType)
	}
	if m.Host == "" {
		t.Error("Expected Host to be set from status.resources")
	}
}

func TestEnrichVM(t *testing.T) {
	m := &model.VM{
		Disks: []model.Disk{
			{StorageContainerUUID: "sc-1"},
		},
		NICs: []model.NIC{
			{SubnetUUID: "net-1"},
		},
	}

	enrichVM(m, map[string]string{"sc-1": "default-container"}, map[string]string{"net-1": "Production-VLAN"})

	if m.Disks[0].StorageContainerName != "default-container" {
		t.Errorf("Expected storage container name to be enriched, got %s", m.Disks[0].StorageContainerName)
	}
	if m.NICs[0].SubnetName != "Production-VLAN" {
		t.Errorf("Expected subnet name to be enriched, got %s", m.NICs[0].SubnetName)
	}
}

func TestApplyGuestTools_Disabled(t *testing.T) {
	spec := libclient.VMGuestTools{}
	spec.NutanixGuestTools.Enabled = false
	status := libclient.VMGuestTools{}

	m := &model.VM{}
	mergeGuestTools(spec, status, m)

	if m.GuestToolsEnabled {
		t.Error("Expected GuestToolsEnabled to be false")
	}
	if m.GuestToolsVersion != "" {
		t.Errorf("Expected empty GuestToolsVersion, got %q", m.GuestToolsVersion)
	}
	if m.GuestToolsMounted {
		t.Error("Expected GuestToolsMounted to be false")
	}
	if m.GuestToolsReachable {
		t.Error("Expected GuestToolsReachable to be false")
	}
}

// TestApplyGuestTools_UnmountedISO verifies that an ISO mount state other than
// "MOUNTED" is correctly reported as not mounted.
func TestApplyGuestTools_UnmountedISO(t *testing.T) {
	spec := libclient.VMGuestTools{}
	spec.NutanixGuestTools.Enabled = true
	spec.NutanixGuestTools.Version = "3.2.0"
	spec.NutanixGuestTools.IsReachable = true
	spec.NutanixGuestTools.ISOMountState = "UNMOUNTED"
	spec.NutanixGuestTools.GuestOSVersion = "Red Hat Enterprise Linux 8.9"
	status := libclient.VMGuestTools{}

	m := &model.VM{}
	mergeGuestTools(spec, status, m)

	if !m.GuestToolsEnabled {
		t.Error("Expected GuestToolsEnabled to be true")
	}
	if m.GuestToolsMounted {
		t.Error("Expected GuestToolsMounted to be false for an UNMOUNTED ISO")
	}
	if !m.GuestToolsReachable {
		t.Error("Expected GuestToolsReachable to be true")
	}
}

func TestApplyGuestTools_NoSection(t *testing.T) {
	m := &model.VM{}
	mergeGuestTools(libclient.VMGuestTools{}, libclient.VMGuestTools{}, m)

	if m.GuestToolsEnabled || m.GuestToolsMounted || m.GuestToolsReachable || m.GuestToolsVersion != "" {
		t.Errorf("Expected all guest tools fields to remain at zero value, got %+v", m)
	}
}

// TestApplyDisk_VolumeGroupDisk verifies that a volume-group-backed disk (no
// storage_container_reference) still captures size and device properties.
func TestApplyDisk_VolumeGroupDisk(t *testing.T) {
	d := libclient.VMDisk{
		UUID: "disk-vg-1",
		DeviceProperties: struct {
			DeviceType  string `json:"device_type"`
			DiskAddress struct {
				AdapterType string `json:"adapter_type"`
				DeviceIndex int    `json:"device_index"`
			} `json:"disk_address"`
		}{
			DeviceType: "DISK",
			DiskAddress: struct {
				AdapterType string `json:"adapter_type"`
				DeviceIndex int    `json:"device_index"`
			}{
				DeviceIndex: 1,
				AdapterType: "SCSI",
			},
		},
		DiskSizeMiB: 51200,
	}

	disk := applyDisk(&d)

	if disk.UUID != "disk-vg-1" {
		t.Errorf("Expected UUID 'disk-vg-1', got %s", disk.UUID)
	}
	if disk.DiskSizeMiB != 51200 {
		t.Errorf("Expected DiskSizeMiB 51200, got %d", disk.DiskSizeMiB)
	}
	if disk.DeviceIndex != 1 {
		t.Errorf("Expected DeviceIndex 1, got %d", disk.DeviceIndex)
	}
	if disk.StorageContainerUUID != "" {
		t.Errorf("Expected empty StorageContainerUUID for a volume-group disk, got %q", disk.StorageContainerUUID)
	}
	if disk.IsCdrom {
		t.Error("Expected IsCdrom to be false for a DISK device type")
	}
}

func TestApplyDisk_Cdrom(t *testing.T) {
	d := libclient.VMDisk{
		UUID: "disk-cdrom-1",
		DeviceProperties: struct {
			DeviceType  string `json:"device_type"`
			DiskAddress struct {
				AdapterType string `json:"adapter_type"`
				DeviceIndex int    `json:"device_index"`
			} `json:"disk_address"`
		}{
			DeviceType: "CDROM",
		},
	}

	disk := applyDisk(&d)

	if !disk.IsCdrom {
		t.Error("Expected IsCdrom to be true for a CDROM device type")
	}
	if disk.SourceImageUUID != "" {
		t.Errorf("Expected empty SourceImageUUID, got %q", disk.SourceImageUUID)
	}
}

// TestApplyDisk_NoStorageContainerRef verifies that a disk with no
// storage_container_reference leaves the storage fields at their zero value.
func TestApplyDisk_NoStorageContainerRef(t *testing.T) {
	d := libclient.VMDisk{UUID: "disk-1"}
	disk := applyDisk(&d)

	if disk.StorageContainerUUID != "" || disk.StorageContainerName != "" {
		t.Errorf("Expected empty storage container fields, got UUID=%q Name=%q",
			disk.StorageContainerUUID, disk.StorageContainerName)
	}
}
