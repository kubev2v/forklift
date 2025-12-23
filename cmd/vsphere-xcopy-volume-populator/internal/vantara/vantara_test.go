package vantara

import (
	"fmt"
	"os"
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
)

// mockVantaraClient is a mock implementation of VantaraClient for testing
type mockVantaraClient struct {
	connectErr          error
	disconnectErr       error
	getLdevResp         *LdevResponse
	getLdevErr          error
	addPathErr          error
	deletePathErr       error
	getPortDetailsResp  *PortDetailsResponse
	getPortDetailsErr   error
	connectCallCount    int
	disconnectCallCount int
	getLdevCallCount    int
	addPathCalls        []addPathCall
	deletePathCalls     []deletePathCall
}

type addPathCall struct {
	ldevId          string
	portId          string
	hostGroupNumber string
}

type deletePathCall struct {
	ldevId          string
	portId          string
	hostGroupNumber string
	lunId           string
}

func (m *mockVantaraClient) Connect() error {
	m.connectCallCount++
	return m.connectErr
}

func (m *mockVantaraClient) Disconnect() error {
	m.disconnectCallCount++
	return m.disconnectErr
}

func (m *mockVantaraClient) GetLdev(ldevId string) (*LdevResponse, error) {
	m.getLdevCallCount++
	return m.getLdevResp, m.getLdevErr
}

func (m *mockVantaraClient) AddPath(ldevId string, portId string, hostGroupNumber string) error {
	m.addPathCalls = append(m.addPathCalls, addPathCall{ldevId, portId, hostGroupNumber})
	return m.addPathErr
}

func (m *mockVantaraClient) DeletePath(ldevId string, portId string, hostGroupNumber string, lunId string) error {
	m.deletePathCalls = append(m.deletePathCalls, deletePathCall{ldevId, portId, hostGroupNumber, lunId})
	return m.deletePathErr
}

func (m *mockVantaraClient) GetPortDetails() (*PortDetailsResponse, error) {
	return m.getPortDetailsResp, m.getPortDetailsErr
}

// TestResolvePVToLUN tests PV to LUN resolution
func TestResolvePVToLUN(t *testing.T) {
	cloner := VantaraCloner{
		client: &mockVantaraClient{},
	}

	tests := []struct {
		name          string
		volumeHandle  string
		expectedLDev  string
		expectedProto string
		expectError   bool
	}{
		{
			name:          "Valid FC volume handle",
			volumeHandle:  "01--fc--ABC123DEF456--100--vol1",
			expectedLDev:  "100",
			expectedProto: "fc",
			expectError:   false,
		},
		{
			name:          "Valid iSCSI volume handle",
			volumeHandle:  "01--iscsi--XYZ789GHI012--200--vol2",
			expectedLDev:  "200",
			expectedProto: "iscsi",
			expectError:   false,
		},
		{
			name:         "Invalid format - missing parts",
			volumeHandle: "01--fc--ABC123",
			expectError:  true,
		},
		{
			name:         "Invalid format - wrong prefix",
			volumeHandle: "02--fc--ABC123DEF456--100--vol1",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv := populator.PersistentVolume{
				Name:         "test-pv",
				VolumeHandle: tt.volumeHandle,
			}

			lun, err := cloner.ResolvePVToLUN(pv)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if lun.LDeviceID != tt.expectedLDev {
					t.Errorf("Expected LDeviceID=%s, got %s", tt.expectedLDev, lun.LDeviceID)
				}
				if lun.Protocol != tt.expectedProto {
					t.Errorf("Expected Protocol=%s, got %s", tt.expectedProto, lun.Protocol)
				}
			}
		})
	}
}

// TestCurrentMappedGroups tests retrieving current mappings
func TestCurrentMappedGroups(t *testing.T) {
	mock := &mockVantaraClient{
		getLdevResp: &LdevResponse{
			LdevId: 100,
			NaaId:  "60060E8012345678",
			Ports: []PortMapping{
				{PortId: "CL1-A", HostGroupNumber: 1, Lun: 0},
				{PortId: "CL2-B", HostGroupNumber: 2, Lun: 1},
			},
		},
	}

	cloner := VantaraCloner{
		client: mock,
	}

	lun := populator.LUN{
		LDeviceID: "100",
	}

	hgids, err := cloner.CurrentMappedGroups(lun, nil)
	if err != nil {
		t.Fatalf("CurrentMappedGroups() failed: %v", err)
	}

	if len(hgids) != 2 {
		t.Fatalf("Expected 2 host group IDs, got %d", len(hgids))
	}

	expectedHgids := []string{"CL1-A,1", "CL2-B,2"}
	for i, expected := range expectedHgids {
		if hgids[i] != expected {
			t.Errorf("Expected hgid[%d]=%s, got %s", i, expected, hgids[i])
		}
	}

	if mock.getLdevCallCount != 1 {
		t.Errorf("Expected 1 GetLdev call, got %d", mock.getLdevCallCount)
	}
}

// TestGetNaaID tests NAA ID retrieval
func TestGetNaaID(t *testing.T) {
	mock := &mockVantaraClient{
		getLdevResp: &LdevResponse{
			LdevId: 100,
			NaaId:  "60060E8012345678",
		},
	}

	cloner := VantaraCloner{
		client: mock,
	}

	lun := populator.LUN{
		LDeviceID: "100",
	}

	result := cloner.GetNaaID(lun)

	if result.ProviderID != "60060E" {
		t.Errorf("Expected ProviderID=60060E, got %s", result.ProviderID)
	}
	if result.SerialNumber != "8012345678" {
		t.Errorf("Expected SerialNumber=8012345678, got %s", result.SerialNumber)
	}
	if result.NAA != "naa.60060E8012345678" {
		t.Errorf("Expected NAA=naa.60060E8012345678, got %s", result.NAA)
	}
}

// TestMap tests LUN mapping
func TestMap(t *testing.T) {
	mock := &mockVantaraClient{
		getLdevResp: &LdevResponse{
			LdevId: 100,
			NaaId:  "60060E8012345678",
		},
	}

	cloner := VantaraCloner{
		client: mock,
	}

	lun := populator.LUN{
		LDeviceID: "100",
	}

	context := populator.MappingContext{
		"hostGroupIds": []string{"CL1-A,1", "CL2-B,2"},
	}

	result, err := cloner.Map("xcopy-ig", lun, context)
	if err != nil {
		t.Fatalf("Map() failed: %v", err)
	}

	// Verify AddPath was called for each hostGroupId
	if len(mock.addPathCalls) != 2 {
		t.Fatalf("Expected 2 AddPath calls, got %d", len(mock.addPathCalls))
	}

	expectedCalls := []addPathCall{
		{"100", "CL1-A", "1"},
		{"100", "CL2-B", "2"},
	}

	for i, expected := range expectedCalls {
		actual := mock.addPathCalls[i]
		if actual != expected {
			t.Errorf("AddPath call %d: expected %+v, got %+v", i, expected, actual)
		}
	}

	// Verify NAA ID was set
	if result.NAA != "naa.60060E8012345678" {
		t.Errorf("Expected NAA to be set, got %s", result.NAA)
	}
}

// TestMapInvalidHostGroupId tests Map with invalid format
func TestMapInvalidHostGroupId(t *testing.T) {
	mock := &mockVantaraClient{}

	cloner := VantaraCloner{
		client: mock,
	}

	lun := populator.LUN{
		LDeviceID: "100",
	}

	context := populator.MappingContext{
		"hostGroupIds": []string{"invalid-format"},
	}

	_, err := cloner.Map("xcopy-ig", lun, context)
	if err == nil {
		t.Error("Expected error for invalid hostGroupId format, got nil")
	}
}

// TestUnMap tests LUN unmapping
func TestUnMap(t *testing.T) {
	mock := &mockVantaraClient{
		getLdevResp: &LdevResponse{
			LdevId: 100,
			NaaId:  "60060E8012345678",
			Ports: []PortMapping{
				{PortId: "CL1-A", HostGroupNumber: 1, Lun: 0},
				{PortId: "CL2-B", HostGroupNumber: 2, Lun: 1},
			},
		},
	}

	cloner := VantaraCloner{
		client: mock,
	}

	lun := populator.LUN{
		LDeviceID: "100",
	}

	context := populator.MappingContext{
		"hostGroupIds": []string{"CL1-A,1", "CL2-B,2"},
	}

	err := cloner.UnMap("xcopy-ig", lun, context)
	if err != nil {
		t.Fatalf("UnMap() failed: %v", err)
	}

	// Verify DeletePath was called for each hostGroupId
	if len(mock.deletePathCalls) != 2 {
		t.Fatalf("Expected 2 DeletePath calls, got %d", len(mock.deletePathCalls))
	}

	expectedCalls := []deletePathCall{
		{"100", "CL1-A", "1", "0"},
		{"100", "CL2-B", "2", "1"},
	}

	for i, expected := range expectedCalls {
		actual := mock.deletePathCalls[i]
		if actual != expected {
			t.Errorf("DeletePath call %d: expected %+v, got %+v", i, expected, actual)
		}
	}
}

// TestEnsureClonnerIgroupWithEnvVar tests using environment variable
func TestEnsureClonnerIgroupWithEnvVar(t *testing.T) {
	mock := &mockVantaraClient{}

	cloner := VantaraCloner{
		client:          mock,
		envHostGroupIds: []string{"CL1-A,1", "CL2-B,2"},
	}

	context, err := cloner.EnsureClonnerIgroup("xcopy-ig", []string{"fc.0:21000024FF123456"})
	if err != nil {
		t.Fatalf("EnsureClonnerIgroup() failed: %v", err)
	}

	hgids := context["hostGroupIds"].([]string)
	if len(hgids) != 2 {
		t.Fatalf("Expected 2 host group IDs, got %d", len(hgids))
	}

	// Verify GetPortDetails was NOT called (used env var instead)
	if mock.getPortDetailsResp != nil {
		t.Error("Expected GetPortDetails to not be called when using env var")
	}
}

// TestEnsureClonnerIgroupFromStorage tests fetching from storage
func TestEnsureClonnerIgroupFromStorage(t *testing.T) {
	mock := &mockVantaraClient{
		getPortDetailsResp: &PortDetailsResponse{
			Data: []DataEntry{
				{
					PortID: "CL1-A",
					WWN:    "50060E801234ABCD",
					Logins: []Logins{
						{
							HostGroupId: "CL1-A,1",
							Islogin:     "true",
							LoginWWN:    "21000024FF123456",
						},
					},
				},
			},
		},
	}

	cloner := VantaraCloner{
		client:          mock,
		envHostGroupIds: []string{}, // Empty - force fetching from storage
	}

	context, err := cloner.EnsureClonnerIgroup("xcopy-ig", []string{"fc.0:21000024FF123456"})
	if err != nil {
		t.Fatalf("EnsureClonnerIgroup() failed: %v", err)
	}

	hgids := context["hostGroupIds"].([]string)
	if len(hgids) != 1 {
		t.Fatalf("Expected 1 host group ID, got %d", len(hgids))
	}

	if hgids[0] != "CL1-A,1" {
		t.Errorf("Expected hostGroupId=CL1-A,1, got %s", hgids[0])
	}
}

// TestGetStorageEnvVars tests environment variable parsing
func TestGetStorageEnvVars(t *testing.T) {
	// Set up test environment variables
	os.Setenv("STORAGE_ID", "test-storage-123")
	os.Setenv("STORAGE_HOSTNAME", "192.0.2.0")
	os.Setenv("STORAGE_PORT", "8443")
	os.Setenv("STORAGE_USERNAME", "admin")
	os.Setenv("STORAGE_PASSWORD", "secret")
	os.Setenv("HOSTGROUP_ID_LIST", "CL1-A,1 : CL2-B,2")
	defer func() {
		os.Unsetenv("STORAGE_ID")
		os.Unsetenv("STORAGE_HOSTNAME")
		os.Unsetenv("STORAGE_PORT")
		os.Unsetenv("STORAGE_USERNAME")
		os.Unsetenv("STORAGE_PASSWORD")
		os.Unsetenv("HOSTGROUP_ID_LIST")
	}()

	envVars, err := getStorageEnvVars()
	if err != nil {
		t.Fatalf("getStorageEnvVars() failed: %v", err)
	}

	if envVars["storageId"] != "test-storage-123" {
		t.Errorf("Expected storageId=test-storage-123, got %v", envVars["storageId"])
	}

	if envVars["restServerIP"] != "192.0.2.0" {
		t.Errorf("Expected restServerIP=192.0.2.0, got %v", envVars["restServerIP"])
	}

	hgids := envVars["hostGroupIds"].([]string)
	if len(hgids) != 2 {
		t.Fatalf("Expected 2 host group IDs, got %d", len(hgids))
	}

	expectedHgids := []string{"CL1-A,1", "CL2-B,2"}
	for i, expected := range expectedHgids {
		if hgids[i] != expected {
			t.Errorf("Expected hgid[%d]=%s, got %s", i, expected, hgids[i])
		}
	}
}

// TestGetLunIdFromPorts tests the helper function
func TestGetLunIdFromPorts(t *testing.T) {
	ports := []PortMapping{
		{PortId: "CL1-A", HostGroupNumber: 1, Lun: 0},
		{PortId: "CL2-B", HostGroupNumber: 2, Lun: 1},
		{PortId: "CL1-A", HostGroupNumber: 3, Lun: 5},
	}

	tests := []struct {
		portId          string
		hostGroupNumber string
		expectedLun     string
		expectError     bool
	}{
		{"CL1-A", "1", "0", false},
		{"CL2-B", "2", "1", false},
		{"CL1-A", "3", "5", false},
		{"CL3-C", "1", "", true},  // Port not found
		{"CL1-A", "99", "", true}, // Host group not found
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s,%s", tt.portId, tt.hostGroupNumber), func(t *testing.T) {
			lunId, err := getLunIdFromPorts(ports, tt.portId, tt.hostGroupNumber)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if lunId != tt.expectedLun {
					t.Errorf("Expected lunId=%s, got %s", tt.expectedLun, lunId)
				}
			}
		})
	}
}
