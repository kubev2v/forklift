package vmware

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"
)

func flatBacking(fileName, backingObjectId string, parent *types.VirtualDiskFlatVer2BackingInfo) *types.VirtualDiskFlatVer2BackingInfo {
	return &types.VirtualDiskFlatVer2BackingInfo{
		VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
			FileName:        fileName,
			BackingObjectId: backingObjectId,
		},
		Parent: parent,
	}
}

func TestFindMatchingFlatBacking(t *testing.T) {
	tests := []struct {
		name           string
		backing        *types.VirtualDiskFlatVer2BackingInfo
		vmdkPath       string
		wantMatch      bool
		wantFileName   string
		wantBackingObj string
	}{
		{
			name:           "direct match at top (cold migration, no active snapshot)",
			backing:        flatBacking("[ds1] vm1/disk.vmdk", "rfc4122.top", nil),
			vmdkPath:       "[ds1] vm1/disk.vmdk",
			wantMatch:      true,
			wantFileName:   "[ds1] vm1/disk.vmdk",
			wantBackingObj: "rfc4122.top",
		},
		{
			name: "parent-chain match (warm precopy: top is child disk, base is parent)",
			backing: flatBacking("[ds1] vm1/disk-000001.vmdk", "rfc4122.base",
				flatBacking("[ds1] vm1/disk.vmdk", "rfc4122.snap", nil)),
			vmdkPath:       "[ds1] vm1/disk.vmdk",
			wantMatch:      true,
			wantFileName:   "[ds1] vm1/disk.vmdk",
			wantBackingObj: "rfc4122.snap",
		},
		{
			name: "deep chain match (two snapshots active)",
			backing: flatBacking("[ds1] vm1/disk-000002.vmdk", "rfc4122.top",
				flatBacking("[ds1] vm1/disk-000001.vmdk", "rfc4122.mid",
					flatBacking("[ds1] vm1/disk.vmdk", "rfc4122.base", nil))),
			vmdkPath:       "[ds1] vm1/disk.vmdk",
			wantMatch:      true,
			wantFileName:   "[ds1] vm1/disk.vmdk",
			wantBackingObj: "rfc4122.base",
		},
		{
			name:      "no match in chain",
			backing:   flatBacking("[ds1] vm1/other.vmdk", "rfc4122.x", nil),
			vmdkPath:  "[ds1] vm1/disk.vmdk",
			wantMatch: false,
		},
		{
			name:      "nil backing",
			backing:   nil,
			vmdkPath:  "[ds1] vm1/disk.vmdk",
			wantMatch: false,
		},
		{
			name:           "bracket/case differences are tolerated",
			backing:        flatBacking("[DS1] VM1/Disk.vmdk", "rfc4122.top", nil),
			vmdkPath:       "[ds1] vm1/disk.vmdk",
			wantMatch:      true,
			wantFileName:   "[DS1] VM1/Disk.vmdk",
			wantBackingObj: "rfc4122.top",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMatchingFlatBacking(tt.backing, strings.ToLower(tt.vmdkPath), tt.vmdkPath)
			if !tt.wantMatch {
				if got != nil {
					t.Errorf("expected nil, got FileName=%q BackingObjectId=%q", got.FileName, got.BackingObjectId)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected match with FileName=%q, got nil", tt.wantFileName)
			}
			if got.FileName != tt.wantFileName {
				t.Errorf("FileName: got %q, want %q", got.FileName, tt.wantFileName)
			}
			if got.BackingObjectId != tt.wantBackingObj {
				t.Errorf("BackingObjectId: got %q, want %q", got.BackingObjectId, tt.wantBackingObj)
			}
		})
	}
}

func TestNewClientWithSimulator(t *testing.T) {
	model := simulator.VPX()
	defer model.Remove()

	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	_, err = NewClient(s.URL.String(), "user", "pass")
	if err != nil {
		t.Errorf("NewClient() error = %v, wantErr %v", err, false)
	}
}

func TestVSphereClient_GetEsxByVm(t *testing.T) {
	model := simulator.VPX()
	defer model.Remove()

	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	client, err := NewClient(s.URL.String(), "user", "pass")
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetEsxByVm(context.TODO(), "vm-1")
	if err == nil {
		t.Errorf("GetEsxByVm() error = %v, wantErr %v", err, true)
	}
}

func TestVSphereClient_GetDatastore(t *testing.T) {
	model := simulator.VPX()
	defer model.Remove()

	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	client, err := NewClient(s.URL.String(), "user", "pass")
	if err != nil {
		t.Fatal(err)
	}

	finder := find.NewFinder(client.(*VSphereClient).Client.Client, false)
	dc, err := finder.DefaultDatacenter(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetDatastore(context.TODO(), dc, "LocalDS_0")
	if err != nil {
		t.Errorf("GetDatastore() error = %v, wantErr %v", err, false)
	}
}

func TestVSphereClient_GetEsxById_ReturnsBareMoRef(t *testing.T) {
	model := simulator.VPX()
	defer model.Remove()

	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	client, err := NewClient(s.URL.String(), "user", "pass")
	if err != nil {
		t.Fatal(err)
	}

	vsClient := client.(*VSphereClient)
	finder := find.NewFinder(vsClient.Client.Client, true)
	dc, err := finder.DefaultDatacenter(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	finder.SetDatacenter(dc)

	hosts, err := finder.HostSystemList(context.TODO(), "*")
	if err != nil || len(hosts) == 0 {
		t.Fatal("no hosts in simulator")
	}
	hostId := hosts[0].Reference().Value

	host, err := client.GetEsxById(context.TODO(), hostId)
	if err != nil {
		t.Fatal(err)
	}

	hostStr := host.String()
	if strings.Contains(hostStr, "@") {
		t.Errorf("GetEsxById() returned host with inventory path in String(): %q, must not contain '@'", hostStr)
	}
	if strings.Contains(hostStr, "/") {
		t.Errorf("GetEsxById() returned host with slashes in String(): %q, must not contain '/'", hostStr)
	}
	if host.Reference().Value != hostId {
		t.Errorf("GetEsxById() returned wrong host: got %s, want %s", host.Reference().Value, hostId)
	}
}

type fakeTargetPortArrayIdentifier struct {
	ports []string
}

func (f *fakeTargetPortArrayIdentifier) TargetPorts() ([]string, error) { return f.ports, nil }

// TestSelectAdaptersForArray_IgnoresRDMDescriptorArray is the regression test for the
// original bug: an RDM disk's real backing LUN can live on an array entirely different
// from the array backing the RDM's own descriptor .vmdk file (whatever datastore that
// happens to sit on). The old GetDatastoreActiveAdapters resolved "the source array" from
// that descriptor's datastore via a datastoreName parameter, so it could pick adapters
// zoned to the wrong array. GetAdaptersForArray has no datastoreName parameter at all
// anymore -- this proves the fix by modeling exactly that scenario: HBA1 is zoned to
// "Array A" (the descriptor's array), HBA2 is zoned to the real migration destination.
// Selection must depend only on the destination, never on Array A.
func TestSelectAdaptersForArray_IgnoresRDMDescriptorArray(t *testing.T) {
	const destinationTargetWWN = 0x6000000000000001

	hbaByKey := map[string]HostAdapter{
		"key-hba-to-descriptor-array": {Name: "vmhba1", Driver: "qlnativefc", Id: "fc.1000000000000001:5000000000000001"},
		"key-hba-to-destination":      {Name: "vmhba2", Driver: "qlnativefc", Id: "fc.1000000000000002:6000000000000001"},
	}
	topology := &types.HostScsiTopology{
		Adapter: []types.HostScsiTopologyInterface{
			{
				Adapter: "key-hba-to-descriptor-array",
				Target: []types.HostScsiTopologyTarget{{
					Transport: &types.HostFibreChannelTargetTransport{PortWorldWideName: 0x5000000000000001},
				}},
			},
			{
				Adapter: "key-hba-to-destination",
				Target: []types.HostScsiTopologyTarget{{
					Transport: &types.HostFibreChannelTargetTransport{PortWorldWideName: destinationTargetWWN},
				}},
			},
		},
	}
	destination := &fakeTargetPortArrayIdentifier{ports: []string{fmt.Sprintf("fc.%016x", uint64(destinationTargetWWN))}}

	result, err := selectAdaptersForArray(hbaByKey, topology, destination, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].Name != "vmhba2" {
		t.Fatalf("expected only vmhba2 (zoned to the destination), got %+v -- selection must never depend on the RDM descriptor's array", result)
	}
}

type fakeSciniArrayIdentifier struct{}

func (f *fakeSciniArrayIdentifier) TargetPorts() ([]string, error) {
	return nil, fmt.Errorf("TargetPorts must not be called when SciniRequired")
}
func (f *fakeSciniArrayIdentifier) SciniRequired() bool { return true }

func TestSelectAdaptersForArray_ScinigGUIDOverride(t *testing.T) {
	const (
		sciniGUID  = "1a2b3c4d-5e6f-7890-abcd-ef1234567890"
		sciniRawId = "fc.0000000000000000:0000000000000001"
	)

	tests := []struct {
		name      string
		hbaByKey  map[string]HostAdapter
		sciniGUID string
		wantErr   bool
		wantId    string
	}{
		{
			name:      "scini adapter gets GUID override",
			hbaByKey:  map[string]HostAdapter{"key-vmhba67": {Name: "vmhba67", Id: sciniRawId, Driver: "scini"}},
			sciniGUID: sciniGUID,
			wantId:    sciniGUID,
		},
		{
			name:      "scini without GUID keeps raw ID",
			hbaByKey:  map[string]HostAdapter{"key-vmhba67": {Name: "vmhba67", Id: sciniRawId, Driver: "scini"}},
			sciniGUID: "",
			wantId:    sciniRawId,
		},
		{
			name:      "no scini adapter on host returns error",
			hbaByKey:  map[string]HostAdapter{"key-vmhba2": {Name: "vmhba2", Id: "fc.2000f4e9d45532da:2100f4e9d45532da", Driver: "qlnativefc"}},
			sciniGUID: sciniGUID,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := selectAdaptersForArray(tt.hbaByKey, &types.HostScsiTopology{}, &fakeSciniArrayIdentifier{}, tt.sciniGUID)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != 1 || result[0].Id != tt.wantId {
				t.Errorf("expected adapter with Id %q, got %+v", tt.wantId, result)
			}
		})
	}
}

func TestVSphereClient_RunEsxCommand(t *testing.T) {
	t.Skip("Skipping test that requires esxcli executor on simulator")
	model := simulator.VPX()
	defer model.Remove()

	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	client, err := NewClient(s.URL.String(), "user", "pass")
	if err != nil {
		t.Fatal(err)
	}

	finder := find.NewFinder(client.(*VSphereClient).Client.Client, false)
	dc, err := finder.DefaultDatacenter(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	finder.SetDatacenter(dc)

	host, err := finder.HostSystem(context.TODO(), "host-21")
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.RunEsxCommand(context.TODO(), host, []string{"echo", "hello"})
	if err != nil {
		t.Errorf("RunEsxCommand() error = %v, wantErr %v", err, false)
	}
}
