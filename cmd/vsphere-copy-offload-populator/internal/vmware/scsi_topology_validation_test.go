//go:build validation

// This file is a standalone validation test for vSphere's HostScsiTopology.
// Run it against a real ESXi host to confirm the struct shape this design
// depends on, in particular whether a target can appear with zero mapped
// LUNs (zoned via fabric login / iSCSI session, nothing masked yet).
//
// Usage:
//   ESXI_HOST=<ip-or-hostname> ESXI_USER=root ESXI_PASS=<password> \
//     go test -tags=validation -run TestScsiTopologyShape -v ./internal/vmware/

package vmware

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func TestScsiTopologyShape(t *testing.T) {
	hostAddr := os.Getenv("ESXI_HOST")
	user := os.Getenv("ESXI_USER")
	pass := os.Getenv("ESXI_PASS")
	if hostAddr == "" || user == "" || pass == "" {
		t.Skip("Set ESXI_HOST, ESXI_USER, ESXI_PASS to run this test")
	}

	client, err := NewClient(hostAddr, user, pass)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	ctx := context.Background()
	vsClient := client.(*VSphereClient)
	finder := find.NewFinder(vsClient.Client.Client, true)
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		t.Fatalf("failed to find datacenter: %v", err)
	}
	finder.SetDatacenter(dc)

	hosts, err := finder.HostSystemList(ctx, "*")
	if err != nil || len(hosts) == 0 {
		t.Fatalf("failed to list hosts: %v", err)
	}
	host := hosts[0]
	t.Logf("using host: %s", host.Reference().Value)

	var hostMo mo.HostSystem
	err = host.Properties(ctx, host.Reference(), []string{"config.storageDevice"}, &hostMo)
	if err != nil {
		t.Fatalf("failed to fetch storage device info: %v", err)
	}

	sd := hostMo.Config.StorageDevice
	if sd == nil {
		t.Fatalf("config.storageDevice is nil")
	}

	t.Logf("HostBusAdapters: %d", len(sd.HostBusAdapter))
	for _, hba := range sd.HostBusAdapter {
		h := hba.GetHostHostBusAdapter()
		t.Logf("  HBA key=%s device=%s driver=%s", h.Key, h.Device, h.Driver)
	}

	if sd.ScsiTopology == nil {
		t.Fatalf("ScsiTopology is nil on this host — cannot validate the design's core assumption")
	}

	t.Logf("ScsiTopology adapters: %d", len(sd.ScsiTopology.Adapter))
	zeroLunTargets := 0
	totalTargets := 0
	for _, iface := range sd.ScsiTopology.Adapter {
		t.Logf("  Adapter key=%s targets=%d", iface.Adapter, len(iface.Target))
		for _, target := range iface.Target {
			totalTargets++
			desc := fmt.Sprintf("unhandled transport type %T", target.Transport)
			switch tr := target.Transport.(type) {
			case *types.HostFibreChannelTargetTransport:
				desc = fmt.Sprintf("FC portWWN=%016x nodeWWN=%016x", uint64(tr.PortWorldWideName), uint64(tr.NodeWorldWideName))
			case *types.HostInternetScsiTargetTransport:
				desc = fmt.Sprintf("iSCSI target=%s address=%v", tr.IScsiName, tr.Address)
			case *types.HostBlockAdapterTargetTransport:
				desc = "block adapter transport (no identity fields)"
			}
			t.Logf("    target key=%s luns=%d transport=%s", target.Key, len(target.Lun), desc)
			if len(target.Lun) == 0 {
				zeroLunTargets++
			}
		}
	}
	t.Logf("RESULT: %d/%d targets have zero mapped LUNs (zoned-but-unmapped case)", zeroLunTargets, totalTargets)
}

// realArrayPorts is a storage.ArrayIdentifier stand-in returning ports independently
// confirmed against the array's own API (see TestSelectAdaptersForArray_RealPureConnectivity).
type realArrayPorts struct {
	ports []string
}

func (r *realArrayPorts) TargetPorts() ([]string, error) { return r.ports, nil }

// TestSelectAdaptersForArray_RealPureConnectivity runs the actual matching
// logic against a live host's real ScsiTopology, using Pure's real iSCSI
// target IQN (confirmed independently via Pure's own /api/1.19/port endpoint:
// iqn.2010-06.com.purestorage:flasharray.60b9ab521cc4b868 — byte-for-byte the
// same string ESXi reports for vmhba64 in ScsiTopology). This closes the loop
// on the "already-connected" case end to end with real data on both sides;
// it does not touch the still-unconfirmed zero-LUN case.
//
// Usage: same env vars as TestScsiTopologyShape, run against host-3078
// (10.46.29.208), which is zoned to ONTAP (FC, vmhba1/vmhba2), Pure (iSCSI,
// vmhba64) and 3PAR (iSCSI, vmhba64) simultaneously.
func TestSelectAdaptersForArray_RealPureConnectivity(t *testing.T) {
	hostAddr := os.Getenv("ESXI_HOST")
	user := os.Getenv("ESXI_USER")
	pass := os.Getenv("ESXI_PASS")
	if hostAddr == "" || user == "" || pass == "" {
		t.Skip("Set ESXI_HOST, ESXI_USER, ESXI_PASS to run this test")
	}

	client, err := NewClient(hostAddr, user, pass)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	ctx := context.Background()
	vsClient := client.(*VSphereClient)
	finder := find.NewFinder(vsClient.Client.Client, true)
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		t.Fatalf("failed to find datacenter: %v", err)
	}
	finder.SetDatacenter(dc)

	hosts, err := finder.HostSystemList(ctx, "*")
	if err != nil || len(hosts) == 0 {
		t.Fatalf("failed to list hosts: %v", err)
	}
	host := hosts[0]

	var hostMo mo.HostSystem
	if err := host.Properties(ctx, host.Reference(), []string{"config.storageDevice"}, &hostMo); err != nil {
		t.Fatalf("failed to fetch storage device info: %v", err)
	}
	sd := hostMo.Config.StorageDevice
	if sd == nil || sd.ScsiTopology == nil {
		t.Fatalf("config.storageDevice or ScsiTopology is nil")
	}

	hbaByKey := make(map[string]HostAdapter, len(sd.HostBusAdapter))
	for _, hba := range sd.HostBusAdapter {
		h := hba.GetHostHostBusAdapter()
		hbaByKey[h.Key] = HostAdapter{Name: h.Device, Driver: h.Driver}
	}

	const pureRealIQN = "iqn.2010-06.com.purestorage:flasharray.60b9ab521cc4b868"
	dst := &realArrayPorts{ports: []string{pureRealIQN}}

	result, err := selectAdaptersForArray(hbaByKey, sd.ScsiTopology, dst, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := make([]string, 0, len(result))
	for _, a := range result {
		names = append(names, a.Name)
	}
	t.Logf("adapters matched for Pure destination: %v", names)

	if len(result) != 1 || result[0].Name != "vmhba64" {
		t.Fatalf("expected exactly [vmhba64] (the only HBA zoned to Pure), got %v", names)
	}
}
