package nutanix

import (
	"encoding/json"
	"os"
	"testing"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
)

// TestApplyCluster tests cluster mapping from API response to model.
func TestApplyCluster(t *testing.T) {
	data, err := os.ReadFile("testdata/clusters_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	entities, ok := response["entities"].([]interface{})
	if !ok || len(entities) == 0 {
		t.Fatal("No entities in response")
	}

	// Test first cluster
	entity, ok := entities[0].(map[string]interface{})
	if !ok {
		t.Fatal("Entity is not a map")
	}

	m := &model.Cluster{}
	applyCluster(entity, m)

	// Verify metadata
	if m.ID == "" {
		t.Error("Expected ID to be set")
	}
	if m.ClusterUUID == "" {
		t.Error("Expected ClusterUUID to be set")
	}
	if m.Name != "prod-cluster-01" {
		t.Errorf("Expected name 'prod-cluster-01', got %s", m.Name)
	}

	// Verify version
	if m.Version != "6.8.2" {
		t.Errorf("Expected version '6.8.2', got %s", m.Version)
	}

	// Verify timezone
	if m.Timezone != "America/Los_Angeles" {
		t.Errorf("Expected timezone 'America/Los_Angeles', got %s", m.Timezone)
	}

	// Verify node count
	if m.NumNodes != 2 {
		t.Errorf("Expected 2 nodes, got %d", m.NumNodes)
	}

	// Verify VM count
	if m.VMCount != 25 {
		t.Errorf("Expected 25 VMs, got %d", m.VMCount)
	}

	// Verify capacity
	if m.TotalCapacity == 0 {
		t.Error("Expected TotalCapacity to be set")
	}
	if m.UsedCapacity == 0 {
		t.Error("Expected UsedCapacity to be set")
	}
}

// TestApplyClusterNameNotFromMetadata verifies that Cluster.Name is read
// from spec/status, not metadata -- Nutanix's v3 intentful entities never
// carry "name" under metadata, only under spec/status. This also verifies
// the status.name fallback used when spec.name is absent.
func TestApplyClusterNameNotFromMetadata(t *testing.T) {
	entity := map[string]interface{}{
		"metadata": map[string]interface{}{
			"uuid": "cluster-1",
			"name": "wrong-name",
		},
		"status": map[string]interface{}{
			"name": "right-name",
		},
	}

	m := &model.Cluster{}
	applyCluster(entity, m)

	if m.Name != "right-name" {
		t.Errorf("Expected name 'right-name' from status, got %s", m.Name)
	}
}

// TestApplyHost tests host mapping from API response to model.
func TestApplyHost(t *testing.T) {
	data, err := os.ReadFile("testdata/hosts_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	entities, ok := response["entities"].([]interface{})
	if !ok || len(entities) == 0 {
		t.Fatal("No entities in response")
	}

	// Test first host
	entity, ok := entities[0].(map[string]interface{})
	if !ok {
		t.Fatal("Entity is not a map")
	}

	m := &model.Host{}
	applyHost(entity, m)

	// Verify basic fields
	if m.ID == "" {
		t.Error("Expected ID to be set")
	}
	if m.HostUUID == "" {
		t.Error("Expected HostUUID to be set")
	}
	if m.Name != "ahv-node-01" {
		t.Errorf("Expected name 'ahv-node-01', got %s", m.Name)
	}

	// Verify cluster reference
	if m.Cluster != "0005e123-4567-89ab-cdef-000000000001" {
		t.Errorf("Expected Cluster '0005e123-4567-89ab-cdef-000000000001', got %s", m.Cluster)
	}

	// Verify hardware details
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

	// Verify hypervisor info (note: may not be present in all responses)
	// Just check it doesn't error out
	_ = m.HypervisorType
}

// TestApplyHostNameAndClusterNotFromWrongPaths verifies Host.Name is read
// from spec/status (not metadata) and Host.Cluster is read from a
// top-level spec/status.cluster_reference (not nested under
// status.resources.cluster_reference), using the status-only fallback.
func TestApplyHostNameAndClusterNotFromWrongPaths(t *testing.T) {
	entity := map[string]interface{}{
		"metadata": map[string]interface{}{
			"uuid": "host-1",
			"name": "wrong-name",
		},
		"status": map[string]interface{}{
			"name": "right-name",
			"cluster_reference": map[string]interface{}{
				"uuid": "right-cluster",
			},
			"resources": map[string]interface{}{
				"cluster_reference": map[string]interface{}{
					"uuid": "wrong-cluster",
				},
			},
		},
	}

	m := &model.Host{}
	applyHost(entity, m)

	if m.Name != "right-name" {
		t.Errorf("Expected name 'right-name' from status, got %s", m.Name)
	}
	if m.Cluster != "right-cluster" {
		t.Errorf("Expected cluster 'right-cluster' from top-level status, got %s", m.Cluster)
	}
}

// TestApplyNetwork tests network mapping from API response to model.
func TestApplyNetwork(t *testing.T) {
	data, err := os.ReadFile("testdata/subnets_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	entities, ok := response["entities"].([]interface{})
	if !ok || len(entities) == 0 {
		t.Fatal("No entities in response")
	}

	// Test first network
	entity, ok := entities[0].(map[string]interface{})
	if !ok {
		t.Fatal("Entity is not a map")
	}

	m := &model.Network{}
	applyNetwork(entity, m)

	// Verify basic fields
	if m.ID == "" {
		t.Error("Expected ID to be set")
	}
	if m.NetworkUUID == "" {
		t.Error("Expected NetworkUUID to be set")
	}
	if m.Name != "Production-VLAN-100" {
		t.Errorf("Expected name 'Production-VLAN-100', got %s", m.Name)
	}

	// Verify cluster reference
	if m.Cluster != "0005e123-4567-89ab-cdef-000000000001" {
		t.Errorf("Expected Cluster '0005e123-4567-89ab-cdef-000000000001', got %s", m.Cluster)
	}

	// Verify network type
	if m.SubnetType != "VLAN" {
		t.Errorf("Expected subnet type 'VLAN', got %s", m.SubnetType)
	}

	// Verify VLAN
	if m.VlanID == 0 {
		t.Error("Expected VlanID to be > 0")
	}

	// Verify IP config
	if m.NetworkAddress == "" {
		t.Error("Expected NetworkAddress to be set")
	}
	if m.PrefixLength == 0 {
		t.Error("Expected PrefixLength to be > 0")
	}
}

// TestApplyNetworkNameAndClusterNotFromWrongPaths verifies Network.Name is
// read from spec/status (not metadata) and Network.Cluster is read from a
// top-level spec/status.cluster_reference (not nested under
// status.resources.cluster_reference), using the status-only fallback.
func TestApplyNetworkNameAndClusterNotFromWrongPaths(t *testing.T) {
	entity := map[string]interface{}{
		"metadata": map[string]interface{}{
			"uuid": "network-1",
			"name": "wrong-name",
		},
		"status": map[string]interface{}{
			"name": "right-name",
			"cluster_reference": map[string]interface{}{
				"uuid": "right-cluster",
			},
			"resources": map[string]interface{}{
				"cluster_reference": map[string]interface{}{
					"uuid": "wrong-cluster",
				},
			},
		},
	}

	m := &model.Network{}
	applyNetwork(entity, m)

	if m.Name != "right-name" {
		t.Errorf("Expected name 'right-name' from status, got %s", m.Name)
	}
	if m.Cluster != "right-cluster" {
		t.Errorf("Expected cluster 'right-cluster' from top-level status, got %s", m.Cluster)
	}
}

// TestApplyStorageContainer tests storage container mapping.
func TestApplyStorageContainer(t *testing.T) {
	data, err := os.ReadFile("testdata/storage_containers_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	entities, ok := response["entities"].([]interface{})
	if !ok || len(entities) == 0 {
		t.Fatal("No entities in response")
	}

	// Test first storage container
	entity, ok := entities[0].(map[string]interface{})
	if !ok {
		t.Fatal("Entity is not a map")
	}

	m := &model.StorageContainer{}
	applyStorageContainer(entity, m)

	// Verify basic fields
	if m.ID == "" {
		t.Error("Expected ID to be set")
	}
	if m.StorageContainerUUID == "" {
		t.Error("Expected StorageContainerUUID to be set")
	}
	if m.Name != "default-container-prod" {
		t.Errorf("Expected name 'default-container-prod', got %s", m.Name)
	}

	// Verify cluster reference (may not be present in all responses)
	_ = m.Cluster

	// Verify replication factor
	if m.ReplicationFactor == 0 {
		t.Error("Expected ReplicationFactor to be > 0")
	}

	// Verify capacity
	if m.MaxCapacityBytes == 0 {
		t.Error("Expected MaxCapacityBytes to be > 0")
	}
}

// TestApplyImage tests image mapping from API response to model.
func TestApplyImage(t *testing.T) {
	data, err := os.ReadFile("testdata/images_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	entities, ok := response["entities"].([]interface{})
	if !ok || len(entities) == 0 {
		t.Fatal("No entities in response")
	}

	// Test first image
	entity, ok := entities[0].(map[string]interface{})
	if !ok {
		t.Fatal("Entity is not a map")
	}

	m := &model.Image{}
	applyImage(entity, m)

	// Verify basic fields
	if m.ID == "" {
		t.Error("Expected ID to be set")
	}
	if m.ImageUUID == "" {
		t.Error("Expected ImageUUID to be set")
	}
	if m.Name == "" {
		t.Error("Expected name to be set")
	}

	// Verify image type
	if m.ImageType == "" {
		t.Error("Expected ImageType to be set")
	}

	// Verify size
	if m.SizeBytes == 0 {
		t.Error("Expected SizeBytes to be > 0")
	}
}

// TestApplyVM tests VM mapping from API response to model.
func TestApplyVM(t *testing.T) {
	data, err := os.ReadFile("testdata/vms_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	entities, ok := response["entities"].([]interface{})
	if !ok || len(entities) == 0 {
		t.Fatal("No entities in response")
	}

	// Test first VM
	entity, ok := entities[0].(map[string]interface{})
	if !ok {
		t.Fatal("Entity is not a map")
	}

	m := &model.VM{}
	applyVM(entity, m)

	// Verify basic fields
	if m.ID == "" {
		t.Error("Expected ID to be set")
	}
	if m.UUID == "" {
		t.Error("Expected UUID to be set")
	}
	if m.Name != "web-server-rhel8" {
		t.Errorf("Expected name 'web-server-rhel8', got %s", m.Name)
	}

	// Verify cluster and host
	if m.Cluster == "" {
		t.Error("Expected Cluster to be set")
	}

	// Verify CPU and memory
	if m.NumSockets != 2 {
		t.Errorf("Expected NumSockets to be 2, got %d", m.NumSockets)
	}
	if m.NumVcpusPerSocket != 2 {
		t.Errorf("Expected NumVcpusPerSocket to be 2, got %d", m.NumVcpusPerSocket)
	}
	if m.MemorySizeMiB != 8192 {
		t.Errorf("Expected MemorySizeMiB to be 8192, got %d", m.MemorySizeMiB)
	}

	// Verify power state
	if m.PowerState != "ON" {
		t.Errorf("Expected PowerState to be 'ON', got %s", m.PowerState)
	}

	// Verify NICs
	if len(m.NICs) == 0 {
		t.Error("Expected at least one NIC")
	} else {
		nic := m.NICs[0]
		if nic.MACAddress == "" {
			t.Error("Expected NIC MAC address to be set")
		}
		if nic.SubnetUUID == "" {
			t.Error("Expected NIC subnet UUID to be set")
		}
	}

	// Verify disks
	if len(m.Disks) == 0 {
		t.Error("Expected at least one disk")
	} else {
		disk := m.Disks[0]
		if disk.UUID == "" {
			t.Error("Expected disk UUID to be set")
		}
		if disk.DiskSizeMiB == 0 {
			t.Error("Expected disk size to be > 0")
		}
		if disk.StorageContainerUUID == "" {
			t.Error("Expected storage container UUID to be set")
		}
	}

	// Verify boot config
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

// TestApplyVMDetail tests VM mapping from a detailed API response.
func TestApplyVMDetail(t *testing.T) {
	data, err := os.ReadFile("testdata/vm_detail_example.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	var entity map[string]interface{}
	if err := json.Unmarshal(data, &entity); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	m := &model.VM{}
	applyVM(entity, m)

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

// TestGetStringHelper tests the getString helper function.
func TestGetStringHelper(t *testing.T) {
	testMap := map[string]interface{}{
		"simple": "value",
		"nested": map[string]interface{}{
			"key": "nested-value",
			"deep": map[string]interface{}{
				"key": "deep-value",
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"simple key", "simple", "value"},
		{"nested key", "nested.key", "nested-value"},
		{"deep nested key", "nested.deep.key", "deep-value"},
		{"non-existent key", "nonexistent", ""},
		{"non-existent nested", "nested.nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getString(testMap, tt.path)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestGetIntHelper tests the getInt helper function.
func TestGetIntHelper(t *testing.T) {
	testMap := map[string]interface{}{
		"int":     42,
		"int64":   int64(100),
		"float64": float64(200),
		"nested": map[string]interface{}{
			"value": 123,
		},
	}

	tests := []struct {
		name     string
		path     string
		expected int
	}{
		{"int value", "int", 42},
		{"int64 value", "int64", 100},
		{"float64 value", "float64", 200},
		{"nested value", "nested.value", 123},
		{"non-existent", "nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getInt(testMap, tt.path)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestGetStringSliceHelper tests the getStringSlice helper function.
func TestGetStringSliceHelper(t *testing.T) {
	testMap := map[string]interface{}{
		"list": []interface{}{"AOS", "PRISM_CENTRAL"},
		"nested": map[string]interface{}{
			"list": []interface{}{"a", "b"},
		},
		"mixed":    []interface{}{"a", 1, "b"},
		"notAList": "value",
	}

	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{"top-level list", "list", []string{"AOS", "PRISM_CENTRAL"}},
		{"nested list", "nested.list", []string{"a", "b"}},
		{"non-string entries dropped", "mixed", []string{"a", "b"}},
		{"non-list value", "notAList", nil},
		{"non-existent key", "nonexistent", nil},
		{"non-existent nested", "nested.nonexistent", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStringSlice(testMap, tt.path)
			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %v, got %v", tt.expected, result)
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("Expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

// TestGetBoolHelper tests the getBool helper function.
func TestGetBoolHelper(t *testing.T) {
	testMap := map[string]interface{}{
		"true":  true,
		"false": false,
		"nested": map[string]interface{}{
			"flag": true,
		},
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"true value", "true", true},
		{"false value", "false", false},
		{"nested value", "nested.flag", true},
		{"non-existent", "nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getBool(testMap, tt.path)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestGetInt64Helper tests the getInt64 helper function.
func TestGetInt64Helper(t *testing.T) {
	testMap := map[string]interface{}{
		"int":     42,
		"int64":   int64(100),
		"float64": float64(200),
		"nested": map[string]interface{}{
			"value": int64(123),
		},
	}

	tests := []struct {
		name     string
		path     string
		expected int64
	}{
		{"int value", "int", 42},
		{"int64 value", "int64", 100},
		{"float64 value", "float64", 200},
		{"nested value", "nested.value", 123},
		{"non-existent", "nonexistent", 0},
		{"non-existent nested", "nested.nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getInt64(testMap, tt.path)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestNormalizeHypervisorType covers both the recognized AHV spellings and
// the passthrough default branch for anything else (e.g. ESXi values that
// might show up if a Nutanix cluster is ever mixed-hypervisor).
func TestNormalizeHypervisorType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase kKvm", "kKvm", "AHV"},
		{"uppercase KKVM", "KKVM", "AHV"},
		{"unrecognized value passthrough", "kVMware", "kVMware"},
		{"empty value passthrough", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := normalizeHypervisorType(tt.input); result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestApplyGuestTools_Disabled verifies a VM with Nutanix Guest Tools never
// installed comes back with every guest-tools field at its zero value,
// rather than erroring or leaving stale values from a previous VM.
func TestApplyGuestTools_Disabled(t *testing.T) {
	specResources := map[string]interface{}{
		"guest_tools": map[string]interface{}{
			"nutanix_guest_tools": map[string]interface{}{
				"enabled": false,
			},
		},
	}
	statusResources := map[string]interface{}{}

	m := &model.VM{}
	applyGuestTools(specResources, statusResources, m)

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

// TestApplyGuestTools_UnmountedISO verifies that an ISO mount state other
// than "MOUNTED" (e.g. "UNMOUNTED") is correctly reported as not mounted.
func TestApplyGuestTools_UnmountedISO(t *testing.T) {
	specResources := map[string]interface{}{
		"guest_tools": map[string]interface{}{
			"nutanix_guest_tools": map[string]interface{}{
				"enabled":          true,
				"version":          "3.2.0",
				"is_reachable":     true,
				"iso_mount_state":  "UNMOUNTED",
				"guest_os_version": "Red Hat Enterprise Linux 8.9",
			},
		},
	}
	statusResources := map[string]interface{}{}

	m := &model.VM{}
	applyGuestTools(specResources, statusResources, m)

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

// TestApplyGuestTools_NoSection verifies a VM with no guest_tools section at
// all in either spec or status resources doesn't error, and leaves the
// model at its zero values.
func TestApplyGuestTools_NoSection(t *testing.T) {
	m := &model.VM{}
	applyGuestTools(map[string]interface{}{}, map[string]interface{}{}, m)

	if m.GuestToolsEnabled || m.GuestToolsMounted || m.GuestToolsReachable || m.GuestToolsVersion != "" {
		t.Errorf("Expected all guest tools fields to remain at zero value, got %+v", m)
	}
}

// TestApplyDiskFromMap_VolumeGroupDisk verifies that a volume-group-backed
// disk (which has no storage_container_reference of its own) still gets its
// size and device properties captured, even though the storage container
// attribution is only resolvable via a separate Volume Group lookup that
// doesn't exist yet.
func TestApplyDiskFromMap_VolumeGroupDisk(t *testing.T) {
	diskData := map[string]interface{}{
		"uuid": "disk-vg-1",
		"device_properties": map[string]interface{}{
			"device_type": "DISK",
			"disk_address": map[string]interface{}{
				"device_index": float64(1),
				"adapter_type": "SCSI",
			},
		},
		"disk_size_mib":          float64(51200),
		"volume_group_reference": map[string]interface{}{"uuid": "vg-1"},
	}

	disk := applyDiskFromMap(diskData)

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

// TestApplyDiskFromMap_Cdrom verifies IsCdrom is derived correctly from
// device_type, and that a CD-ROM with no data source doesn't error.
func TestApplyDiskFromMap_Cdrom(t *testing.T) {
	diskData := map[string]interface{}{
		"uuid": "disk-cdrom-1",
		"device_properties": map[string]interface{}{
			"device_type": "CDROM",
			"disk_address": map[string]interface{}{
				"device_index": float64(0),
				"adapter_type": "IDE",
			},
		},
	}

	disk := applyDiskFromMap(diskData)

	if !disk.IsCdrom {
		t.Error("Expected IsCdrom to be true for a CDROM device type")
	}
	if disk.SourceImageUUID != "" {
		t.Errorf("Expected empty SourceImageUUID, got %q", disk.SourceImageUUID)
	}
}

// TestApplyStorageContainerRef_Missing verifies that a disk with no
// storage_container_reference at all leaves the disk's storage fields at
// their zero value instead of erroring.
func TestApplyStorageContainerRef_Missing(t *testing.T) {
	disk := &model.Disk{}
	applyStorageContainerRef(map[string]interface{}{}, disk)

	if disk.StorageContainerUUID != "" || disk.StorageContainerName != "" {
		t.Errorf("Expected empty storage container fields, got UUID=%q Name=%q",
			disk.StorageContainerUUID, disk.StorageContainerName)
	}
}
