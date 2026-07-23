package vmware

import (
	"context"
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

func TestPrepareAdapters(t *testing.T) {
	const (
		sciniGUID  = "1a2b3c4d-5e6f-7890-abcd-ef1234567890"
		fcId       = "fc.2000f4e9d45532da:2100f4e9d45532da"
		iscsiId    = "iqn.1998-01.com.vmware:esxi01:12345"
		sciniRawId = "fc.0000000000000000:0000000000000001"
		blockId    = "vmhba0"
	)

	tests := []struct {
		name      string
		adapters  map[string]HostAdapter
		sciniGUID string
		wantErr   bool
		wantCount int
		check     func(t *testing.T, result []HostAdapter)
	}{
		{
			name: "scini adapter gets GUID override",
			adapters: map[string]HostAdapter{
				"vmhba67": {Name: "vmhba67", Id: sciniRawId, Driver: "scini"},
			},
			sciniGUID: sciniGUID,
			wantCount: 1,
			check: func(t *testing.T, result []HostAdapter) {
				if result[0].Id != sciniGUID {
					t.Errorf("expected GUID %q, got %q", sciniGUID, result[0].Id)
				}
			},
		},
		{
			name: "scini without GUID keeps raw ID",
			adapters: map[string]HostAdapter{
				"vmhba67": {Name: "vmhba67", Id: sciniRawId, Driver: "scini"},
			},
			sciniGUID: "",
			wantCount: 1,
			check: func(t *testing.T, result []HostAdapter) {
				if result[0].Id != sciniRawId {
					t.Errorf("expected raw ID %q, got %q", sciniRawId, result[0].Id)
				}
			},
		},
		{
			name: "FC adapter passes through unchanged",
			adapters: map[string]HostAdapter{
				"vmhba2": {Name: "vmhba2", Id: fcId, Driver: "qlnativefc"},
			},
			sciniGUID: "",
			wantCount: 1,
			check: func(t *testing.T, result []HostAdapter) {
				if result[0].Id != fcId {
					t.Errorf("expected %q, got %q", fcId, result[0].Id)
				}
			},
		},
		{
			name: "mixed adapters: scini gets GUID, others pass through",
			adapters: map[string]HostAdapter{
				"vmhba67": {Name: "vmhba67", Id: sciniRawId, Driver: "scini"},
				"vmhba2":  {Name: "vmhba2", Id: fcId, Driver: "qlnativefc"},
				"vmhba64": {Name: "vmhba64", Id: iscsiId, Driver: "iscsi_vmk"},
			},
			sciniGUID: sciniGUID,
			wantCount: 3,
			check: func(t *testing.T, result []HostAdapter) {
				for _, a := range result {
					if a.Driver == "scini" && a.Id != sciniGUID {
						t.Errorf("scini should have GUID %q, got %q", sciniGUID, a.Id)
					}
					if a.Driver == "qlnativefc" && a.Id != fcId {
						t.Errorf("FC should be unchanged %q, got %q", fcId, a.Id)
					}
					if a.Driver == "iscsi_vmk" && a.Id != iscsiId {
						t.Errorf("iSCSI should be unchanged %q, got %q", iscsiId, a.Id)
					}
				}
			},
		},
		{
			name:      "empty adapters returns error",
			adapters:  map[string]HostAdapter{},
			sciniGUID: "",
			wantErr:   true,
		},
		{
			name: "all host adapters (different-array case)",
			adapters: map[string]HostAdapter{
				"key-vmhba0":  {Name: "vmhba0", Id: blockId, Driver: "smartpqi"},
				"key-vmhba67": {Name: "vmhba67", Id: sciniRawId, Driver: "scini"},
				"key-vmhba2":  {Name: "vmhba2", Id: fcId, Driver: "qlnativefc"},
				"key-vmhba64": {Name: "vmhba64", Id: iscsiId, Driver: "iscsi_vmk"},
			},
			sciniGUID: sciniGUID,
			wantCount: 4,
			check: func(t *testing.T, result []HostAdapter) {
				for _, a := range result {
					if a.Driver == "scini" && a.Id != sciniGUID {
						t.Errorf("scini should have GUID, got %q", a.Id)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := prepareAdapters(tt.adapters, tt.sciniGUID, "test-datastore")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("expected %d adapters, got %d: %+v", tt.wantCount, len(result), result)
			}
			if tt.check != nil {
				tt.check(t, result)
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
