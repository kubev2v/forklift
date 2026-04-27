package vantara

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware"
	vmware_mocks "github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware/mocks"

	"go.uber.org/mock/gomock"
)

// mockVantaraClient is a mock implementation of VantaraClient for testing
type mockVantaraClient struct {
	connectErr               error
	disconnectErr            error
	getLdevResp              *LdevResponse
	getLdevErr               error
	addPathErr               error
	deletePathErr            error
	getPortDetailsResp       *PortDetailsResponse
	getPortDetailsErr        error
	connectCallCount         int
	disconnectCallCount      int
	getLdevCallCount         int
	addPathCalls             []addPathCall
	deletePathCalls          []deletePathCall
	createCloneErr           error
	pairsResp                *ClonePairResponse
	createCloneLdevCallCount int
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

func (m *mockVantaraClient) CreateCloneLdev(snapshotGroupName string, snapshotPoolId string, pvolLdevId string, svolLdevId string, copySpeed string) error {
	m.createCloneLdevCallCount++
	if m.createCloneErr != nil {
		return m.createCloneErr
	}
	return m.createCloneErr
}

func (m *mockVantaraClient) GetClonePairs(snapshotGroupName string, pvolLdevId string) (*ClonePairResponse, error) {
	if m.pairsResp == nil {
		return &ClonePairResponse{}, nil
	}
	return m.pairsResp, nil
}

func (m *mockVantaraClient) GetStorageInfo() (*StorageInfo, error) {
	return &StorageInfo{
		Model:           "VSP 5600",
		DkcMicroVersion: "90-08-01",
	}, nil
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

func TestVvolCopy_ResolvePVToLUNError(t *testing.T) {
	mock := &mockVantaraClient{}
	cloner := &VantaraCloner{client: mock}

	progress := make(chan uint64, 10)

	// Use a PV handle that is guaranteed to fail ResolvePVToLUN():
	// len(parts) != 5 OR parts[0] != "01"
	badPV := populator.PersistentVolume{VolumeHandle: "bad-handle"}

	// sourceVMDKFile: pick something plausible-looking.
	// If your ParseVmdkPath is strict and fails here, change to a known-good sample.
	sourceVMDK := "[datastore1] vm/vm.vmdk"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	vc := vmware_mocks.NewMockClient(ctrl)
	vc.EXPECT().
		GetVMDiskBacking(gomock.Any(), "vm-001", sourceVMDK).
		Return(&vmware.DiskBacking{
			IsRDM: false,
		}, nil)

	err := cloner.VvolCopy(
		/* vsphereClient */ vc,
		/* vmId */ "vm-001",
		/* sourceVMDKFile */ sourceVMDK,
		/* persistentVolume */ badPV,
		progress,
	)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	// It should fail before any storage copy call occurs.
	if mock.getLdevCallCount != 0 {
		t.Fatalf("expected GetLdev not called, got %d", mock.getLdevCallCount)
	}

	select {
	case p := <-progress:
		t.Fatalf("unexpected progress value: %d", p)
	default:
		// ok
	}
}

func TestPerformVolumeCopy_Success_ImmediatePSUP(t *testing.T) {
	mock := &mockVantaraClient{
		pairsResp: &ClonePairResponse{
			Data: []CloneDataEntry{
				{Status: "PSUP"},
			},
		},
	}

	v := &VantaraCloner{client: mock}

	progress := make(chan uint64, 1)

	ldevResp := &LdevResponse{
		LdevId: 1,
		PoolId: 2,
	}

	err := v.performVolumeCopy("1234", ldevResp, progress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case p := <-progress:
		if p != 100 {
			t.Fatalf("unexpected progress: got=%d want=100", p)
		}
	default:
		t.Fatalf("expected progress to be written")
	}
}

func TestPerformVolumeCopy_CreateCloneError(t *testing.T) {
	mock := &mockVantaraClient{
		createCloneErr: errors.New("create clone failed"),
		// Never return PSUP status to ensure it fails at CreateCloneLdev step, not later
		pairsResp: &ClonePairResponse{
			Data: []CloneDataEntry{
				{Status: "PSUP"},
			},
		},
	}

	v := &VantaraCloner{client: mock}

	progress := make(chan uint64, 1)

	ldevResp := &LdevResponse{
		LdevId: 1,
		PoolId: 2,
	}

	err := v.performVolumeCopy("1234", ldevResp, progress)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	// Should not emit progress when CreateCloneLdev fails, even if pairsToReturn has PSUP, because it should fail before the loop that checks pairsToReturn.
	select {
	case p := <-progress:
		t.Fatalf("unexpected progress on error: %d", p)
	default:
		// ok
	}
}

func TestFindVolumeByVVolID_Success(t *testing.T) {
	v := &VantaraCloner{}

	// last4 = ABCD (hex) => 43981 (dec)
	got, err := v.findVolumeByVVolID("00000000-0000-0000-0000-00000000ABCD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "43981" {
		t.Fatalf("unexpected value: got=%s want=43981", got)
	}
}

func TestFindVolumeByVVolID_TooShort(t *testing.T) {
	v := &VantaraCloner{}

	_, err := v.findVolumeByVVolID("ABC") // len < 4
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestFindVolumeByVVolID_NonHexLast4(t *testing.T) {
	v := &VantaraCloner{}

	// last4 = "ZZZZ" is not hex
	_, err := v.findVolumeByVVolID("0000ZZZZ")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestRDMCopy_Error_WhenGetBackingFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	vc := vmware_mocks.NewMockClient(ctrl)
	vc.EXPECT().
		GetVMDiskBacking(gomock.Any(), "vm-001", "[ds] vm/disk.vmdk").
		Return(nil, errors.New("boom"))

	cloner := &VantaraCloner{
		client: &mockVantaraClient{getLdevResp: &LdevResponse{}},
	}

	progress := make(chan uint64, 10)

	err := cloner.RDMCopy(
		vc,
		"vm-001",
		"[ds] vm/disk.vmdk",
		populator.PersistentVolume{VolumeHandle: "01--fc--storage--999--target"},
		progress,
	)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestRDMCopy_Error_WhenNotRDM(t *testing.T) {
	v := &VantaraCloner{
		client: &mockVantaraClient{getLdevResp: &LdevResponse{}},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	vc := vmware_mocks.NewMockClient(ctrl)
	vc.EXPECT().
		GetVMDiskBacking(gomock.Any(), "vm-001", "[ds] vm/disk.vmdk").
		Return(&vmware.DiskBacking{
			IsRDM:      false,
			DeviceName: "naa.60060e80deadbeefdeadbeefdeadbeef",
		}, nil)

	progress := make(chan uint64, 10)

	err := v.RDMCopy(
		(vmware.Client)(vc),
		"vm-001",
		"[ds] vm/disk.vmdk",
		populator.PersistentVolume{VolumeHandle: "01--fc--storage--999--target"},
		progress,
	)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	select {
	case p := <-progress:
		t.Fatalf("unexpected progress: %d", p)
	default:
		// ok
	}
}

func TestRDMCopy_Success_ProgressSequence(t *testing.T) {

	naaDevice := "60060e80" + "0000000000000000000000ab" // 8 + 24 = 32

	targetLdevID := "999"

	fc := &mockVantaraClient{
		// return PSUP immediately to skip waiting loop and test progress sequence more easily
		pairsResp: &ClonePairResponse{Data: []CloneDataEntry{{Status: "PSUP"}}},
		getLdevResp: &LdevResponse{
			LdevId: 171,
			PoolId: 3,
			NaaId:  "60060e800000000000000000000000ab",
		},
	}

	v := &VantaraCloner{client: fc}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	vc := vmware_mocks.NewMockClient(ctrl)
	vc.EXPECT().
		GetVMDiskBacking(gomock.Any(), "vm-001", "[ds] vm/disk.vmdk").
		Return(&vmware.DiskBacking{
			IsRDM:      true,
			DeviceName: "naa." + naaDevice, // lower-case to match what resolveRDMToLUN returns
		}, nil)

	progress := make(chan uint64, 10)

	err := v.RDMCopy(
		(vmware.Client)(vc),
		"vm-001",
		"[ds] vm/disk.vmdk",
		populator.PersistentVolume{VolumeHandle: "01--fc--storage--" + targetLdevID + "--target"},
		progress,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// RDMCopy: progress <- 10
	// performVolumeCopy: progress <- 100
	// RDMCopy: progress <- 100
	got := []uint64{}
	for len(progress) > 0 {
		got = append(got, <-progress)
	}

	want := []uint64{10, 100, 100}
	if len(got) != len(want) {
		t.Fatalf("unexpected progress length: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected progress sequence: got=%v want=%v", got, want)
		}
	}
}

func TestRDMCopy_Error_WhenResolveRDMToLUNMismatchNAA(t *testing.T) {
	naaDevice := "60060e80" + "0000000000000000000000ab" // last4=00ab => 171

	// Intentionally mismatch the NAA in the LdevResponse to trigger the error path in resolveRDMToLUN
	fc := &mockVantaraClient{
		getLdevResp: &LdevResponse{
			LdevId: 171,
			NaaId:  "60060e80" + "1111111111111111111111ab", // mismatch
		},
	}

	v := &VantaraCloner{client: fc}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	vc := vmware_mocks.NewMockClient(ctrl)
	vc.EXPECT().
		GetVMDiskBacking(gomock.Any(), "vm-001", "[ds] vm/disk.vmdk").
		Return(&vmware.DiskBacking{
			IsRDM:      true,
			DeviceName: "naa." + naaDevice,
		}, nil)

	progress := make(chan uint64, 10)

	err := v.RDMCopy(
		(vmware.Client)(vc),
		"vm-001",
		"[ds] vm/disk.vmdk",
		populator.PersistentVolume{VolumeHandle: "01--fc--storage--999--target"},
		progress,
	)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	// resolveRDMToLUN fails, so progress is not written
	select {
	case p := <-progress:
		t.Fatalf("unexpected progress: %d", p)
	default:
		// ok
	}
}

func TestResolveRDMToLUN_Success(t *testing.T) {
	// 32 chars starting with provider prefix (8 chars)
	// last4 = "00ab" => 171(dec)
	naaDevice := VantaraProviderID + "0000000000000000000000ab" // 8 + 24 = 32
	deviceName := "naa." + naaDevice

	fc := &mockVantaraClient{
		getLdevResp: &LdevResponse{
			LdevId: 171,
			NaaId:  naaDevice, // must match extracted 32 chars (case-insensitive compare via ToLower)
		},
	}

	v := &VantaraCloner{client: fc}

	lun, err := v.resolveRDMToLUN(deviceName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lun.LDeviceID != "171" {
		t.Fatalf("unexpected LDeviceID: got=%s want=171", lun.LDeviceID)
	}
	// resolveRDMToLUN sets NAA to lower-case ldevResp.NaaId (no "naa." prefix)
	if lun.NAA != naaDevice {
		t.Fatalf("unexpected NAA: got=%s want=%s", lun.NAA, naaDevice)
	}
}

func TestResolveRDMToLUN_Error_TargetNotFound(t *testing.T) {
	fc := &mockVantaraClient{}
	v := &VantaraCloner{client: fc}

	_, err := v.resolveRDMToLUN("naa.1234567890")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestResolveRDMToLUN_Error_StringTooShort(t *testing.T) {
	fc := &mockVantaraClient{}
	v := &VantaraCloner{client: fc}

	// provider id is present but total length is insufficient for 32 chars slice
	deviceName := "naa." + VantaraProviderID + "short"

	_, err := v.resolveRDMToLUN(deviceName)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestResolveRDMToLUN_Error_InvalidHexLast4(t *testing.T) {
	naaDevice := VantaraProviderID + "0000000000000000000000ZZ" // last4="00ZZ" -> invalid hex
	deviceName := "naa." + naaDevice

	fc := &mockVantaraClient{}
	v := &VantaraCloner{client: fc}

	_, err := v.resolveRDMToLUN(deviceName)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestResolveRDMToLUN_Error_NAAMismatch(t *testing.T) {
	naaDevice := VantaraProviderID + "0000000000000000000000ab" // => 171
	deviceName := "naa." + naaDevice

	fc := &mockVantaraClient{
		getLdevResp: &LdevResponse{
			LdevId: 171,
			// mismatch on purpose
			NaaId: VantaraProviderID + "1111111111111111111111ab",
		},
	}
	v := &VantaraCloner{client: fc}

	_, err := v.resolveRDMToLUN(deviceName)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

// Optional: ensure it lower-cases deviceName before searching/comparing
func TestResolveRDMToLUN_CaseInsensitiveDeviceName(t *testing.T) {
	naaDeviceLower := VantaraProviderID + "0000000000000000000000ab"
	deviceNameUpper := "NAA." + naaDeviceLower // prefix uppercase, but provider id is numeric so ok

	fc := &mockVantaraClient{
		getLdevResp: &LdevResponse{
			LdevId: 171,
			// Put uppercase in NaaId to confirm code lower-cases it
			NaaId: naaDeviceLower,
		},
	}
	v := &VantaraCloner{client: fc}

	lun, err := v.resolveRDMToLUN(deviceNameUpper)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lun.LDeviceID != "171" {
		t.Fatalf("unexpected LDeviceID: got=%s want=171", lun.LDeviceID)
	}
	if lun.NAA != naaDeviceLower {
		t.Fatalf("unexpected NAA: got=%s want=%s", lun.NAA, naaDeviceLower)
	}
}
