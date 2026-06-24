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
