package copyappliance

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/nbd-container/runner"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
)

// testContext is an ApplianceContext that reports the given vCenter instance
// UUID without connecting to anything.
func testContext(appliance *api.CopyAppliance, instanceUUID string) *ApplianceContext {
	vcenter := &govmomi.Client{Client: new(vim25.Client)}
	vcenter.ServiceContent.About.InstanceUuid = instanceUUID
	return &ApplianceContext{
		Appliance: appliance,
		VCenter:   vcenter,
		Log:       testLog(),
	}
}

// guestNIC is one adapter as VMware Tools reports it. A nil ips means the
// guest has told us about the adapter but not about any address on it.
func guestNIC(network, mac string, ips ...string) types.GuestNicInfo {
	nic := types.GuestNicInfo{
		Network:    network,
		MacAddress: mac,
	}
	if ips == nil {
		return nic
	}
	nic.IpConfig = &types.NetIpConfigInfo{}
	for _, ip := range ips {
		nic.IpConfig.IpAddress = append(
			nic.IpConfig.IpAddress,
			types.NetIpConfigInfoIpAddress{IpAddress: ip})
	}
	return nic
}

// A runner that acted on a moRef recorded against another vCenter would be
// operating on a stranger's VM.
func TestCheckInstance(t *testing.T) {
	tests := []struct {
		name      string
		recorded  string
		connected string
		wantErr   bool
	}{
		{
			name:      "the same vCenter is accepted",
			recorded:  "uuid-a",
			connected: "uuid-a",
			wantErr:   false,
		},
		{
			name:      "a different vCenter is refused",
			recorded:  "uuid-a",
			connected: "uuid-b",
			wantErr:   true,
		},
		{
			name:      "an appliance recorded before the UUID was tracked is adopted",
			recorded:  "",
			connected: "uuid-b",
			wantErr:   false,
		},
		{
			name:      "an unreadable connection UUID is not evidence of a different vCenter",
			recorded:  "uuid-a",
			connected: "",
			wantErr:   false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			appliance := testAppliance()
			appliance.Status.VCenterInstanceUUID = tc.recorded

			err := testContext(appliance, tc.connected).CheckInstance()

			if (err != nil) != tc.wantErr {
				t.Errorf("CheckInstance = %v, want error: %v", err, tc.wantErr)
			}
		})
	}
}

func TestMatchExports(t *testing.T) {
	attached := []api.AttachedDisk{
		{
			VMDKPath: "[ds] vm/disk-0.vmdk",
			DiskKey:  2000,
			Serial:   "6000C297-7d53-fad7-e8b4-5194193802f7",
		},
		{
			VMDKPath: "[ds] vm/disk-1.vmdk",
			DiskKey:  2001,
			Serial:   "6000C290-2b72-f55a-2146-435072abcdef01",
		},
	}
	announced := []runner.Export{
		{WWID: "36000c2902b72f55a2146435072abcdef01", Port: 10810, Device: "/dev/sdc"},
		{WWID: "36000c2977d53fad7e8b45194193802f7", Port: 10809, Device: "/dev/sdb"},
		// Extra export (e.g. something discovery also saw) is ignored.
		{WWID: "36000c2999999999999999999999999999", Port: 10811, Device: "/dev/sdd"},
	}

	matched, err := matchExports(attached, announced)
	if err != nil {
		t.Fatalf("matchExports: %v", err)
	}
	if len(matched) != 2 {
		t.Fatalf("matched %d exports, want 2", len(matched))
	}
	if matched[0].DiskKey != 2000 || matched[0].Port != 10809 {
		t.Errorf("first export = %+v, want disk 2000 on port 10809", matched[0])
	}
	if matched[1].DiskKey != 2001 || matched[1].Port != 10810 {
		t.Errorf("second export = %+v, want disk 2001 on port 10810", matched[1])
	}
}

func TestMatchExportsRequiresAMatch(t *testing.T) {
	_, err := matchExports(
		[]api.AttachedDisk{{Serial: "6000C297-7d53-fad7-e8b4-5194193802f7", VMDKPath: "[ds] a.vmdk"}},
		[]runner.Export{{WWID: "36000c2900000000000000000000000000", Port: 10809}},
	)
	if err == nil {
		t.Fatal("matchExports succeeded with no matching export")
	}
}
