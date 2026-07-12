package hyperv

import (
	"encoding/json"
	"fmt"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv/types"
	"github.com/kubev2v/forklift/pkg/lib/hyperv/driver"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockDriver implements driver.HyperVDriver for unit tests.
type mockDriver struct {
	clusterData  *driver.ClusterData
	clusterNodes []driver.ClusterNodeData
	clusterVMs   []driver.ClusterGroupData
	runOnNodeFn  func(command, computerName string) (string, error)
}

func (m *mockDriver) Connect() error                                           { return nil }
func (m *mockDriver) Close() error                                             { return nil }
func (m *mockDriver) IsAlive() (bool, error)                                   { return true, nil }
func (m *mockDriver) ListAllDomains() ([]driver.Domain, error)                 { return nil, nil }
func (m *mockDriver) ListAllClusterDomains() ([]driver.Domain, error)          { return nil, nil }
func (m *mockDriver) LookupDomainByName(string) (driver.Domain, error)         { return nil, nil } //nolint:nilnil
func (m *mockDriver) LookupDomainByUUIDString(string) (driver.Domain, error)   { return nil, nil } //nolint:nilnil
func (m *mockDriver) ListAllNetworks() ([]driver.Network, error)               { return nil, nil }
func (m *mockDriver) LookupNetworkByUUIDString(string) (driver.Network, error) { return nil, nil } //nolint:nilnil
func (m *mockDriver) ExecuteCommand(string) (string, error)                    { return "", nil }
func (m *mockDriver) GetComputerInfo() (*driver.ComputerInfoData, error)       { return nil, nil } //nolint:nilnil
func (m *mockDriver) GetCluster() (*driver.ClusterData, error)                 { return m.clusterData, nil }
func (m *mockDriver) GetClusterNodes() ([]driver.ClusterNodeData, error)       { return m.clusterNodes, nil }
func (m *mockDriver) GetClusterVMGroups() ([]driver.ClusterGroupData, error) {
	return m.clusterVMs, nil
}
func (m *mockDriver) RunOnNode(command, computerName string) (string, error) {
	if m.runOnNodeFn != nil {
		return m.runOnNodeFn(command, computerName)
	}
	return "", nil
}

func newClusterProvider() *api.Provider {
	pt := api.HyperV
	return &api.Provider{
		ObjectMeta: v1.ObjectMeta{Name: "test-provider"},
		Spec: api.ProviderSpec{
			Type:     &pt,
			Settings: map[string]string{api.ManagementType: api.HyperVCluster},
		},
	}
}

func newStandaloneProvider() *api.Provider {
	pt := api.HyperV
	return &api.Provider{
		ObjectMeta: v1.ObjectMeta{Name: "test-provider"},
		Spec: api.ProviderSpec{
			Type:     &pt,
			Settings: map[string]string{api.ManagementType: api.HyperVStandalone},
		},
	}
}

// testLogger returns a LevelLogger suitable for unit tests.
func testLogger() logging.LevelLogger {
	return logging.WithName("test")
}

func TestParseBatchVMDetails(t *testing.T) {
	payload := `{
		"vm-linux-01": {
			"Security": {"TpmEnabled": false, "SecureBoot": false},
			"HasCheckpoint": true,
			"Disks": [
				{"Path": "C:\\VMs\\vm-linux-01\\disk0.vhdx", "Capacity": 42949672960, "RCTEnabled": true},
				{"Path": "C:\\VMs\\vm-linux-01\\disk1.vhdx", "Capacity": 10737418240, "RCTEnabled": false}
			],
			"GuestOS": "Red Hat Enterprise Linux 9",
			"GuestNetworks": [
				{"MAC": "00155D010101", "IPs": ["192.168.1.10"], "Subnets": ["255.255.255.0"], "DHCP": false, "GW": ["192.168.1.1"], "DNS": ["8.8.8.8"]}
			]
		},
		"vm-win-02": {
			"Security": {"TpmEnabled": true, "SecureBoot": true},
			"HasCheckpoint": false,
			"Disks": [
				{"Path": "C:\\VMs\\vm-win-02\\os.vhdx", "Capacity": 107374182400, "RCTEnabled": true}
			],
			"GuestOS": "Microsoft Windows Server 2022 Standard"
		}
	}`

	var result map[string]*batchVMDetail
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		t.Fatalf("Failed to parse batch payload: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 VMs, got %d", len(result))
	}

	linux := result["vm-linux-01"]
	if linux == nil {
		t.Fatal("vm-linux-01 not found in results")
	}
	if linux.Security.TpmEnabled {
		t.Error("Expected TpmEnabled=false for vm-linux-01")
	}
	if !linux.HasCheckpoint {
		t.Error("Expected HasCheckpoint=true for vm-linux-01")
	}
	if len(linux.Disks) != 2 {
		t.Fatalf("Expected 2 disks for vm-linux-01, got %d", len(linux.Disks))
	}
	if linux.Disks[0].Capacity != 42949672960 {
		t.Errorf("Expected disk0 capacity 42949672960, got %d", linux.Disks[0].Capacity)
	}
	if !linux.Disks[0].RCTEnabled {
		t.Error("Expected disk0 RCTEnabled=true")
	}
	if linux.GuestOS != "Red Hat Enterprise Linux 9" {
		t.Errorf("Expected GuestOS 'Red Hat Enterprise Linux 9', got '%s'", linux.GuestOS)
	}
	if len(linux.GuestNetworks) != 1 {
		t.Fatalf("Expected 1 guest network, got %d", len(linux.GuestNetworks))
	}
	if linux.GuestNetworks[0].MAC != "00155D010101" {
		t.Errorf("Expected MAC '00155D010101', got '%s'", linux.GuestNetworks[0].MAC)
	}

	win := result["vm-win-02"]
	if win == nil {
		t.Fatal("vm-win-02 not found in results")
	}
	if !win.Security.TpmEnabled || !win.Security.SecureBoot {
		t.Error("Expected TPM and SecureBoot enabled for vm-win-02")
	}
	if win.HasCheckpoint {
		t.Error("Expected HasCheckpoint=false for vm-win-02")
	}
	if len(win.Disks) != 1 {
		t.Errorf("Expected 1 disk for vm-win-02, got %d", len(win.Disks))
	}
}

func TestApplyBatchDetails(t *testing.T) {
	vms := []types.VM{
		{
			Name:      "vm-linux-01",
			Firmware:  "bios",
			OwnerNode: "node1",
			Disks: []types.Disk{
				{WindowsPath: `C:\VMs\vm-linux-01\disk0.vhdx`},
				{WindowsPath: `C:\VMs\vm-linux-01\disk1.vhdx`},
			},
			NICs: []types.NIC{
				{Name: "nic-0", MAC: "00:15:5D:01:01:01", DeviceIndex: 0},
			},
		},
		{
			Name:      "vm-win-02",
			Firmware:  "uefi",
			OwnerNode: "node1",
			Disks: []types.Disk{
				{WindowsPath: `C:\VMs\vm-win-02\os.vhdx`},
			},
		},
	}

	batchMap := map[string]*batchVMDetail{
		"vm-linux-01": {
			HasCheckpoint: true,
			Disks: []struct {
				Path           string `json:"Path"`
				Capacity       int64  `json:"Capacity"`
				RCTEnabled     bool   `json:"RCTEnabled"`
				ControllerType int    `json:"CT"`
				ControllerNum  int    `json:"CN"`
				ControllerLoc  int    `json:"CL"`
			}{
				{Path: `C:\VMs\vm-linux-01\disk0.vhdx`, Capacity: 42949672960, RCTEnabled: true, ControllerType: 1},
				{Path: `C:\VMs\vm-linux-01\disk1.vhdx`, Capacity: 10737418240, RCTEnabled: false, ControllerType: 1},
			},
			GuestOS: "Red Hat Enterprise Linux 9",
			GuestNetworks: []struct {
				MAC     string   `json:"MAC"`
				IPs     []string `json:"IPs"`
				Subnets []string `json:"Subnets"`
				DHCP    bool     `json:"DHCP"`
				GW      []string `json:"GW"`
				DNS     []string `json:"DNS"`
			}{
				{
					MAC:     "00155D010101",
					IPs:     []string{"192.168.1.10"},
					Subnets: []string{"255.255.255.0"},
					DHCP:    false,
					GW:      []string{"192.168.1.1"},
					DNS:     []string{"8.8.8.8"},
				},
			},
		},
		"vm-win-02": {
			Disks: []struct {
				Path           string `json:"Path"`
				Capacity       int64  `json:"Capacity"`
				RCTEnabled     bool   `json:"RCTEnabled"`
				ControllerType int    `json:"CT"`
				ControllerNum  int    `json:"CN"`
				ControllerLoc  int    `json:"CL"`
			}{
				{Path: `C:\VMs\vm-win-02\os.vhdx`, Capacity: 107374182400, RCTEnabled: true, ControllerType: 1},
			},
		},
	}
	batchMap["vm-win-02"].Security.TpmEnabled = true
	batchMap["vm-win-02"].Security.SecureBoot = true

	client := &Client{}
	allIndices := []int{0, 1}
	client.applyBatchDetails(vms, allIndices, batchMap, nil)

	// Verify vm-linux-01
	if !vms[0].HasCheckpoint {
		t.Error("vm-linux-01: expected HasCheckpoint=true")
	}
	if vms[0].GuestOS != "Red Hat Enterprise Linux 9" {
		t.Errorf("vm-linux-01: expected GuestOS 'Red Hat Enterprise Linux 9', got '%s'", vms[0].GuestOS)
	}
	if vms[0].Disks[0].Capacity != 42949672960 {
		t.Errorf("vm-linux-01 disk0: expected capacity 42949672960, got %d", vms[0].Disks[0].Capacity)
	}
	if !vms[0].Disks[0].RCTEnabled {
		t.Error("vm-linux-01 disk0: expected RCTEnabled=true")
	}
	if vms[0].Disks[1].Capacity != 10737418240 {
		t.Errorf("vm-linux-01 disk1: expected capacity 10737418240, got %d", vms[0].Disks[1].Capacity)
	}
	if vms[0].Disks[1].RCTEnabled {
		t.Error("vm-linux-01 disk1: expected RCTEnabled=false")
	}
	if len(vms[0].GuestNetworks) != 1 {
		t.Fatalf("vm-linux-01: expected 1 guest network, got %d", len(vms[0].GuestNetworks))
	}
	if vms[0].GuestNetworks[0].IP != "192.168.1.10" {
		t.Errorf("vm-linux-01: expected guest IP '192.168.1.10', got '%s'", vms[0].GuestNetworks[0].IP)
	}
	if vms[0].GuestNetworks[0].PrefixLength != 24 {
		t.Errorf("vm-linux-01: expected prefix length 24, got %d", vms[0].GuestNetworks[0].PrefixLength)
	}
	// MAC should be normalized to colon-separated format
	expectedMAC := "00:15:5D:01:01:01"
	if vms[0].GuestNetworks[0].MAC != expectedMAC {
		t.Errorf("vm-linux-01: expected MAC '%s', got '%s'", expectedMAC, vms[0].GuestNetworks[0].MAC)
	}

	// Verify vm-win-02
	if !vms[1].TpmEnabled {
		t.Error("vm-win-02: expected TpmEnabled=true")
	}
	if !vms[1].SecureBoot {
		t.Error("vm-win-02: expected SecureBoot=true")
	}
	if vms[1].Disks[0].Capacity != 107374182400 {
		t.Errorf("vm-win-02 disk0: expected capacity 107374182400, got %d", vms[1].Disks[0].Capacity)
	}
	if !vms[1].Disks[0].RCTEnabled {
		t.Error("vm-win-02 disk0: expected RCTEnabled=true")
	}
}

func TestApplyBatchDetails_DiskPathNormalization(t *testing.T) {
	vms := []types.VM{
		{
			Name: "test-vm",
			Disks: []types.Disk{
				{WindowsPath: `C:\VMs\test\disk.vhdx`},
			},
		},
	}

	batchMap := map[string]*batchVMDetail{
		"test-vm": {
			Disks: []struct {
				Path           string `json:"Path"`
				Capacity       int64  `json:"Capacity"`
				RCTEnabled     bool   `json:"RCTEnabled"`
				ControllerType int    `json:"CT"`
				ControllerNum  int    `json:"CN"`
				ControllerLoc  int    `json:"CL"`
			}{
				// Path uses forward slashes — should still match
				{Path: "C:/VMs/test/disk.vhdx", Capacity: 5368709120, RCTEnabled: true},
			},
		},
	}

	client := &Client{}
	client.applyBatchDetails(vms, []int{0}, batchMap, nil)

	if vms[0].Disks[0].Capacity != 5368709120 {
		t.Errorf("Expected capacity 5368709120, got %d (path normalization failed)", vms[0].Disks[0].Capacity)
	}
}

func TestApplyBatchDetails_ClusterModeBuildDisksAndNICs(t *testing.T) {
	networks := []types.Network{
		{UUID: "switch-uuid-1", Name: "External Switch"},
	}
	// Cluster mode: VMs have no disks/NICs pre-populated
	vms := []types.VM{
		{Name: "vm-cluster-01", UUID: "uuid-01"},
	}

	batchMap := map[string]*batchVMDetail{
		"vm-cluster-01": {
			Disks: []struct {
				Path           string `json:"Path"`
				Capacity       int64  `json:"Capacity"`
				RCTEnabled     bool   `json:"RCTEnabled"`
				ControllerType int    `json:"CT"`
				ControllerNum  int    `json:"CN"`
				ControllerLoc  int    `json:"CL"`
			}{
				{Path: `C:\VMs\vm-cluster-01\os.vhdx`, Capacity: 42949672960, RCTEnabled: true, ControllerType: 1, ControllerNum: 0, ControllerLoc: 0},
				{Path: `C:\VMs\vm-cluster-01\data.vhd`, Capacity: 10737418240, RCTEnabled: false, ControllerType: 1, ControllerNum: 0, ControllerLoc: 1},
			},
			NICs: []struct {
				Name       string `json:"Name"`
				MACAddress string `json:"MAC"`
				SwitchName string `json:"Switch"`
				VlanId     int    `json:"Vlan"`
			}{
				{Name: "Network Adapter", MACAddress: "00155D010101", SwitchName: "External Switch", VlanId: 100},
			},
		},
	}

	client := &Client{Log: testLogger()}
	client.applyBatchDetails(vms, []int{0}, batchMap, networks)

	if len(vms[0].Disks) != 2 {
		t.Fatalf("Expected 2 disks, got %d", len(vms[0].Disks))
	}
	if vms[0].Disks[0].ID != "uuid-01-disk-0" {
		t.Errorf("Expected disk ID 'uuid-01-disk-0', got '%s'", vms[0].Disks[0].ID)
	}
	if vms[0].Disks[0].Capacity != 42949672960 {
		t.Errorf("Expected capacity 42949672960, got %d", vms[0].Disks[0].Capacity)
	}
	if !vms[0].Disks[0].RCTEnabled {
		t.Error("Expected RCTEnabled=true for disk 0")
	}
	if vms[0].Disks[0].Format != "vhdx" {
		t.Errorf("Expected format 'vhdx', got '%s'", vms[0].Disks[0].Format)
	}
	if vms[0].Disks[1].Format != "vhd" {
		t.Errorf("Expected format 'vhd' for .vhd file, got '%s'", vms[0].Disks[1].Format)
	}

	if len(vms[0].NICs) != 1 {
		t.Fatalf("Expected 1 NIC, got %d", len(vms[0].NICs))
	}
	if vms[0].NICs[0].MAC != "00:15:5D:01:01:01" {
		t.Errorf("Expected normalized MAC '00:15:5D:01:01:01', got '%s'", vms[0].NICs[0].MAC)
	}
	if vms[0].NICs[0].NetworkUUID != "switch-uuid-1" {
		t.Errorf("Expected NetworkUUID 'switch-uuid-1', got '%s'", vms[0].NICs[0].NetworkUUID)
	}
	if vms[0].NICs[0].VlanId != 100 {
		t.Errorf("Expected VlanId 100, got %d", vms[0].NICs[0].VlanId)
	}
}

func TestApplyBatchDetails_EmptyBatch(t *testing.T) {
	vms := []types.VM{
		{Name: "orphan-vm", Disks: []types.Disk{{WindowsPath: `D:\vhd\x.vhdx`}}},
	}

	batchMap := map[string]*batchVMDetail{}

	client := &Client{}
	client.applyBatchDetails(vms, []int{0}, batchMap, nil)

	if vms[0].Disks[0].Capacity != 0 {
		t.Error("Expected no change for VM not in batch")
	}
}

func TestBatchVMDetailsJSON_RealFormat(t *testing.T) {
	// Simulates the actual PowerShell ConvertTo-Json output format
	payload := `{"vm-01":{"Security":{"TpmEnabled":false,"SecureBoot":false},"HasCheckpoint":false,"Disks":[],"GuestOS":"","GuestNetworks":null}}`

	var result map[string]*batchVMDetail
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	if result["vm-01"] == nil {
		t.Fatal("vm-01 not found")
	}
	if result["vm-01"].HasCheckpoint {
		t.Error("Expected HasCheckpoint=false")
	}
	if len(result["vm-01"].Disks) != 0 {
		t.Error("Expected empty disk array")
	}
}

func TestMapWindowsPathToSMB(t *testing.T) {
	client := &Client{
		smbMountPath: "/mnt/smb/hyperv-share",
		Log:          testLogger(),
	}

	tests := []struct {
		name             string
		windowsPath      string
		smbWindowsPrefix string
		expected         string
	}{
		{
			name:             "standard path mapping",
			windowsPath:      `C:\ClusterStorage\VMs\test\disk.vhdx`,
			smbWindowsPrefix: `C:\ClusterStorage\VMs`,
			expected:         "/mnt/smb/hyperv-share/test/disk.vhdx",
		},
		{
			name:             "case insensitive prefix matching",
			windowsPath:      `c:\clusterstorage\vms\test\disk.vhdx`,
			smbWindowsPrefix: `C:\ClusterStorage\VMs`,
			expected:         "/mnt/smb/hyperv-share/test/disk.vhdx",
		},
		{
			name:             "no match returns empty",
			windowsPath:      `D:\Other\disk.vhdx`,
			smbWindowsPrefix: `C:\ClusterStorage\VMs`,
			expected:         "",
		},
		{
			name:             "empty prefix returns empty",
			windowsPath:      `C:\VMs\disk.vhdx`,
			smbWindowsPrefix: "",
			expected:         "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := client.mapWindowsPathToSMB(tc.windowsPath, tc.smbWindowsPrefix)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestMapWindowsPathToSMB_UNC(t *testing.T) {
	client := &Client{
		smbMountPath: "/hyperv",
		smbUrl:       "//10.0.0.1/VMShare",
		Log:          testLogger(),
	}

	tests := []struct {
		name             string
		windowsPath      string
		smbWindowsPrefix string
		expected         string
	}{
		{
			name:             "UNC backslash path matching share name",
			windowsPath:      `\\WIN-SERVER\VMShare\vm1.vhdx`,
			smbWindowsPrefix: `C:\Hyper-V\Virtual_Hard_Disks`,
			expected:         "/hyperv/vm1.vhdx",
		},
		{
			name:             "UNC forward slash path matching share name",
			windowsPath:      "//WIN-SERVER/VMShare/subdir/disk.vhdx",
			smbWindowsPrefix: `C:\Hyper-V\Virtual_Hard_Disks`,
			expected:         "/hyperv/subdir/disk.vhdx",
		},
		{
			name:             "UNC case insensitive share name match",
			windowsPath:      `\\SERVER\vmshare\disk.vhdx`,
			smbWindowsPrefix: `C:\Hyper-V\Virtual_Hard_Disks`,
			expected:         "/hyperv/disk.vhdx",
		},
		{
			name:             "UNC different share name falls through to local",
			windowsPath:      `\\SERVER\OtherShare\disk.vhdx`,
			smbWindowsPrefix: `C:\Hyper-V\Virtual_Hard_Disks`,
			expected:         "",
		},
		{
			name:             "UNC share root without trailing file",
			windowsPath:      `\\SERVER\VMShare`,
			smbWindowsPrefix: `C:\Hyper-V\Virtual_Hard_Disks`,
			expected:         "/hyperv/",
		},
		{
			name:             "local path still works when smbUrl is set",
			windowsPath:      `C:\Hyper-V\Virtual_Hard_Disks\vm2.vhdx`,
			smbWindowsPrefix: `C:\Hyper-V\Virtual_Hard_Disks`,
			expected:         "/hyperv/vm2.vhdx",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := client.mapWindowsPathToSMB(tc.windowsPath, tc.smbWindowsPrefix)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestFormatMAC(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"00155D010101", "00:15:5D:01:01:01"},
		{"00-15-5D-01-01-01", "00:15:5D:01:01:01"},
		{"00:15:5D:01:01:01", "00:15:5D:01:01:01"},
		{"short", "SHORT"},
	}

	for _, tc := range tests {
		result := formatMAC(tc.input)
		if result != tc.expected {
			t.Errorf("formatMAC(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestListCluster_ClusterMode(t *testing.T) {
	md := &mockDriver{
		clusterData: &driver.ClusterData{Name: "cluster01", Domain: "lab.local"},
		clusterNodes: []driver.ClusterNodeData{
			{Name: "node-a", State: 0, Id: "id-a"},
			{Name: "node-b", State: 0, Id: "id-b"},
		},
	}
	client := &Client{driver: md, provider: newClusterProvider(), Log: testLogger()}

	cluster, err := client.ListCluster()
	if err != nil {
		t.Fatalf("ListCluster error: %v", err)
	}
	if cluster == nil {
		t.Fatal("Expected non-nil cluster")
	}
	if cluster.Name != "cluster01" {
		t.Errorf("Expected cluster name 'cluster01', got '%s'", cluster.Name)
	}
	if cluster.Domain != "lab.local" {
		t.Errorf("Expected domain 'lab.local', got '%s'", cluster.Domain)
	}
	if len(cluster.Nodes) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(cluster.Nodes))
	}
	if cluster.Nodes[0] != "node-a" || cluster.Nodes[1] != "node-b" {
		t.Errorf("Unexpected node names: %v", cluster.Nodes)
	}
}

func TestListCluster_StandaloneReturnsNil(t *testing.T) {
	client := &Client{provider: newStandaloneProvider(), Log: testLogger()}

	cluster, err := client.ListCluster()
	if err != nil {
		t.Fatalf("ListCluster error: %v", err)
	}
	if cluster != nil {
		t.Error("Expected nil cluster for standalone provider")
	}
}

func TestListHosts_ClusterMode(t *testing.T) {
	computerInfoJSON := `{"CsDNSHostName":"node-a","CsDomain":"lab.local","CsNumberOfProcessors":2,"CsNumberOfLogicalProcessors":8,"OsTotalVisibleMemorySize":16777216}`
	md := &mockDriver{
		clusterData: &driver.ClusterData{Name: "cluster01", Domain: "lab.local"},
		clusterNodes: []driver.ClusterNodeData{
			{Name: "node-a", State: 0, Id: "id-a"},
		},
		runOnNodeFn: func(command, computerName string) (string, error) {
			return computerInfoJSON, nil
		},
	}
	client := &Client{driver: md, provider: newClusterProvider(), Log: testLogger()}

	hosts, err := client.ListHosts()
	if err != nil {
		t.Fatalf("ListHosts error: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host, got %d", len(hosts))
	}
	if hosts[0].Name != "node-a" {
		t.Errorf("Expected host name 'node-a', got '%s'", hosts[0].Name)
	}
	if hosts[0].State != "Up" {
		t.Errorf("Expected state 'Up', got '%s'", hosts[0].State)
	}
	if hosts[0].CpuCount != 2 {
		t.Errorf("Expected 2 CPU sockets, got %d", hosts[0].CpuCount)
	}
	if hosts[0].CpuCores != 8 {
		t.Errorf("Expected 8 logical processors, got %d", hosts[0].CpuCores)
	}
	if hosts[0].MemoryMB != 16384 {
		t.Errorf("Expected 16384 MB memory, got %d", hosts[0].MemoryMB)
	}
	if hosts[0].ClusterName != "cluster01" {
		t.Errorf("Expected cluster 'cluster01', got '%s'", hosts[0].ClusterName)
	}
}

func TestListHosts_StandaloneReturnsNil(t *testing.T) {
	client := &Client{provider: newStandaloneProvider(), Log: testLogger()}

	hosts, err := client.ListHosts()
	if err != nil {
		t.Fatalf("ListHosts error: %v", err)
	}
	if hosts != nil {
		t.Error("Expected nil hosts for standalone provider")
	}
}

func TestGetClusterCache_Caching(t *testing.T) {
	callCount := 0
	md := &mockDriver{
		clusterData:  &driver.ClusterData{Name: "c1"},
		clusterNodes: []driver.ClusterNodeData{{Name: "n1", State: 0, Id: "1"}},
	}
	origGetCluster := md.GetCluster
	_ = origGetCluster
	client := &Client{driver: md, provider: newClusterProvider(), Log: testLogger()}

	// Wrap to count calls
	md2 := &countingMockDriver{mockDriver: md, getClusterCalls: &callCount}
	client.driver = md2

	cc1, err := client.getClusterCache()
	if err != nil {
		t.Fatal(err)
	}
	cc2, err := client.getClusterCache()
	if err != nil {
		t.Fatal(err)
	}
	if cc1 != cc2 {
		t.Error("Expected same cache object on second call")
	}
	if callCount != 1 {
		t.Errorf("Expected 1 GetCluster call (cached), got %d", callCount)
	}

	client.InvalidateClusterCache()
	_, err = client.getClusterCache()
	if err != nil {
		t.Fatal(err)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 GetCluster calls after invalidation, got %d", callCount)
	}
}

type countingMockDriver struct {
	*mockDriver
	getClusterCalls *int
}

func (c *countingMockDriver) GetCluster() (*driver.ClusterData, error) {
	*c.getClusterCalls++
	return c.mockDriver.GetCluster()
}

func TestEnrichVMsWithOwnerNode(t *testing.T) {
	md := &mockDriver{
		clusterVMs: []driver.ClusterGroupData{
			{Name: "vm-web", OwnerNode: "node-a", State: 0, Id: "g1"},
			{Name: "vm-db", OwnerNode: "node-b", State: 0, Id: "g2"},
		},
	}
	client := &Client{driver: md, Log: testLogger()}

	vms := []types.VM{
		{Name: "vm-web"},
		{Name: "vm-db"},
		{Name: "vm-orphan"},
	}
	client.enrichVMsWithOwnerNode(vms)

	if vms[0].OwnerNode != "node-a" {
		t.Errorf("vm-web: expected OwnerNode 'node-a', got '%s'", vms[0].OwnerNode)
	}
	if !vms[0].IsClusterRole {
		t.Error("vm-web: expected IsClusterRole=true")
	}
	if vms[1].OwnerNode != "node-b" {
		t.Errorf("vm-db: expected OwnerNode 'node-b', got '%s'", vms[1].OwnerNode)
	}
	if !vms[1].IsClusterRole {
		t.Error("vm-db: expected IsClusterRole=true")
	}
	if vms[2].OwnerNode != "" {
		t.Errorf("vm-orphan: expected empty OwnerNode, got '%s'", vms[2].OwnerNode)
	}
	if vms[2].IsClusterRole {
		t.Error("vm-orphan: expected IsClusterRole=false")
	}
}

func TestCollectBatchVMDetails_MergesHardwareAndGuest(t *testing.T) {
	hwJSON := `{"vm-01":{"Security":{"TpmEnabled":true,"SecureBoot":true},"HasCheckpoint":false,"Disks":[{"Path":"C:\\disk.vhdx","Capacity":1000,"RCTEnabled":true}]}}`
	guestJSON := `{"vm-01":{"GuestOS":"RHEL 9","GuestNetworks":[{"MAC":"00155D010101","IPs":["10.0.0.1"],"Subnets":["255.255.255.0"],"DHCP":false,"GW":["10.0.0.1"],"DNS":["8.8.8.8"]}]}}`
	callNum := 0
	md := &mockDriver{
		runOnNodeFn: func(command, computerName string) (string, error) {
			callNum++
			if callNum == 1 {
				return hwJSON, nil
			}
			return guestJSON, nil
		},
	}
	client := &Client{driver: md, Log: testLogger()}

	result, err := client.collectBatchVMDetails("node-a")
	if err != nil {
		t.Fatalf("collectBatchVMDetails error: %v", err)
	}
	vm, found := result["vm-01"]
	if !found {
		t.Fatal("vm-01 not found")
	}
	if !vm.Security.TpmEnabled {
		t.Error("Expected TpmEnabled=true")
	}
	if vm.GuestOS != "RHEL 9" {
		t.Errorf("Expected GuestOS 'RHEL 9', got '%s'", vm.GuestOS)
	}
	if len(vm.Disks) != 1 || vm.Disks[0].Capacity != 1000 {
		t.Error("Disk data not preserved after merge")
	}
	if len(vm.GuestNetworks) != 1 || vm.GuestNetworks[0].MAC != "00155D010101" {
		t.Error("Guest network data not merged")
	}
}

func TestCollectBatchVMDetails_GuestFailureStillReturnsHardware(t *testing.T) {
	hwJSON := `{"vm-01":{"Security":{"TpmEnabled":false,"SecureBoot":false},"HasCheckpoint":true,"Disks":[]}}`
	callNum := 0
	md := &mockDriver{
		runOnNodeFn: func(command, computerName string) (string, error) {
			callNum++
			if callNum == 1 {
				return hwJSON, nil
			}
			return "", fmt.Errorf("WinRM timeout")
		},
	}
	client := &Client{driver: md, Log: testLogger()}

	result, err := client.collectBatchVMDetails("")
	if err != nil {
		t.Fatalf("Expected no error when guest fails: %v", err)
	}
	if result["vm-01"] == nil {
		t.Fatal("Expected hardware data even though guest failed")
	}
	if !result["vm-01"].HasCheckpoint {
		t.Error("Expected HasCheckpoint=true from hardware data")
	}
}

func TestBuildGuestNetworks(t *testing.T) {
	cfgs := []guestNetCfg{
		{
			MAC:     "00155D010101",
			IPs:     []string{"192.168.1.10", "fe80::1"},
			Subnets: []string{"255.255.255.0", "64"},
			DHCP:    true,
			GW:      []string{"192.168.1.1", "fe80::gw"},
			DNS:     []string{"8.8.8.8", "2001:4860:4860::8888"},
		},
	}
	nics := []types.NIC{
		{Name: "nic-0", MAC: "00:15:5D:01:01:01", DeviceIndex: 0},
	}

	result := buildGuestNetworks(cfgs, nics)

	if len(result) != 2 {
		t.Fatalf("Expected 2 guest networks (one per IP), got %d", len(result))
	}

	ipv4 := result[0]
	if ipv4.IP != "192.168.1.10" {
		t.Errorf("Expected IPv4 '192.168.1.10', got '%s'", ipv4.IP)
	}
	if ipv4.MAC != "00:15:5D:01:01:01" {
		t.Errorf("Expected normalized MAC, got '%s'", ipv4.MAC)
	}
	if ipv4.Origin != "Dhcp" {
		t.Errorf("Expected origin 'Dhcp', got '%s'", ipv4.Origin)
	}
	if ipv4.PrefixLength != 24 {
		t.Errorf("Expected prefix 24, got %d", ipv4.PrefixLength)
	}
	if ipv4.Gateway != "192.168.1.1" {
		t.Errorf("Expected gateway '192.168.1.1', got '%s'", ipv4.Gateway)
	}
	if len(ipv4.DNS) != 1 || ipv4.DNS[0] != "8.8.8.8" {
		t.Errorf("Expected IPv4 DNS [8.8.8.8], got %v", ipv4.DNS)
	}

	ipv6 := result[1]
	if ipv6.IP != "fe80::1" {
		t.Errorf("Expected IPv6 'fe80::1', got '%s'", ipv6.IP)
	}
	if ipv6.PrefixLength != 64 {
		t.Errorf("Expected prefix 64, got %d", ipv6.PrefixLength)
	}
}

func TestBuildGuestNetworks_DashedMAC(t *testing.T) {
	cfgs := []guestNetCfg{
		{MAC: "00-15-5D-01-01-01", IPs: []string{"10.0.0.1"}, Subnets: []string{"255.255.255.0"}},
	}
	result := buildGuestNetworks(cfgs, nil)
	if len(result) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(result))
	}
	if result[0].MAC != "00:15:5D:01:01:01" {
		t.Errorf("Expected colon-separated MAC from dashed input, got '%s'", result[0].MAC)
	}
}

func TestBuildGuestNetworks_Empty(t *testing.T) {
	result := buildGuestNetworks(nil, nil)
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d entries", len(result))
	}
}

func TestApplyClusterTo(t *testing.T) {
	cluster := &types.Cluster{
		Name:   "cluster01",
		Domain: "lab.local",
		Nodes:  []string{"node-a", "node-b"},
	}
	m := &model.Cluster{}
	applyClusterTo(cluster, m)

	if m.ID != "cluster01" || m.Name != "cluster01" {
		t.Errorf("Expected ID/Name 'cluster01', got '%s'/'%s'", m.ID, m.Name)
	}
	if m.Domain != "lab.local" {
		t.Errorf("Expected Domain 'lab.local', got '%s'", m.Domain)
	}
	if len(m.Nodes) != 2 {
		t.Fatalf("Expected 2 node refs, got %d", len(m.Nodes))
	}
	if m.Nodes[0].Kind != model.HostKind || m.Nodes[0].ID != "node-a" {
		t.Errorf("Node[0] = %+v, want Kind=%s ID=node-a", m.Nodes[0], model.HostKind)
	}
	if m.Nodes[1].ID != "node-b" {
		t.Errorf("Node[1].ID = '%s', want 'node-b'", m.Nodes[1].ID)
	}
}

func TestApplyHostTo(t *testing.T) {
	host := &types.Host{
		ID:          "id-a",
		Name:        "node-a",
		State:       "Up",
		ClusterName: "cluster01",
		CpuCount:    2,
		CpuCores:    16,
		MemoryMB:    32768,
	}
	m := &model.Host{}
	applyHostTo(host, m)

	if m.ID != "node-a" || m.Name != "node-a" {
		t.Errorf("Expected ID/Name 'node-a', got '%s'/'%s'", m.ID, m.Name)
	}
	if m.State != "Up" {
		t.Errorf("Expected State 'Up', got '%s'", m.State)
	}
	if m.Cluster != "cluster01" {
		t.Errorf("Expected Cluster 'cluster01', got '%s'", m.Cluster)
	}
	if m.CpuSockets != 2 {
		t.Errorf("Expected CpuSockets 2, got %d", m.CpuSockets)
	}
	if m.CpuCores != 16 {
		t.Errorf("Expected CpuCores 16, got %d", m.CpuCores)
	}
	expectedMemBytes := int64(32768) * 1024 * 1024
	if m.MemoryBytes != expectedMemBytes {
		t.Errorf("Expected MemoryBytes %d, got %d", expectedMemBytes, m.MemoryBytes)
	}
}
