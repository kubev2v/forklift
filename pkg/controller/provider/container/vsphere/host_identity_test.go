package vsphere

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

func TestHostIdentityPropertiesRequested(t *testing.T) {
	c := Collector{client: &govmomi.Client{Client: &vim25.Client{ServiceContent: types.ServiceContent{About: types.AboutInfo{ApiVersion: "6.5"}}}}}
	paths := make(map[string]bool)
	for _, spec := range c.propertySpec() {
		if spec.Type == Host {
			for _, path := range spec.PathSet {
				paths[path] = true
			}
		}
	}
	for _, path := range []string{"summary.hardware.cpuModel", "hardware.systemInfo"} {
		if !paths[path] {
			t.Errorf("host property %q was not requested", path)
		}
	}
}

func TestHostHardwareIdentity(t *testing.T) {
	tests := []struct {
		name    string
		info    any
		serial  string
		service string
	}{
		{"direct serial and service tag", types.HostSystemInfo{SerialNumber: " SN-1 ", OtherIdentifyingInfo: []types.HostSystemIdentificationInfo{{IdentifierType: &types.ElementDescription{Key: "ServiceTag"}, IdentifierValue: " TAG-1 "}}}, "SN-1", "TAG-1"},
		{"serial tag on older vSphere", types.HostSystemInfo{OtherIdentifyingInfo: []types.HostSystemIdentificationInfo{{IdentifierType: &types.ElementDescription{Key: "SerialNumberTag"}, IdentifierValue: "SN-2"}}}, "SN-2", ""},
		{"direct serial has priority", types.HostSystemInfo{SerialNumber: "SN-direct", OtherIdentifyingInfo: []types.HostSystemIdentificationInfo{{IdentifierType: &types.ElementDescription{Key: "SerialNumberTag"}, IdentifierValue: "SN-tag"}}}, "SN-direct", ""},
		{"unrelated and missing identifiers", types.HostSystemInfo{Uuid: "bios-uuid", OtherIdentifyingInfo: []types.HostSystemIdentificationInfo{{IdentifierType: &types.ElementDescription{Key: "AssetTag"}, IdentifierValue: "asset"}, {IdentifierValue: "untyped"}}}, "", ""},
		{"pointer value", &types.HostSystemInfo{SerialNumber: "SN-pointer"}, "SN-pointer", ""},
		{"typed nil", (*types.HostSystemInfo)(nil), "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := HostAdapter{}
			a.Apply(storageUpdate("summary.hardware.cpuModel", "Xeon Gold"))
			a.Apply(storageUpdate("hardware.systemInfo", tt.info))
			got := storageHostJSON(t, &a)
			storageFields(t, got, map[string]any{"cpuModel": "Xeon Gold", "serialNumber": optionalExpected(tt.serial), "serviceTag": optionalExpected(tt.service)})
		})
	}
}

func optionalExpected(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func TestHostIdentityUpdateAndRemoval(t *testing.T) {
	a := HostAdapter{}
	a.Apply(storageUpdate("summary.hardware.cpuModel", "CPU old"))
	a.Apply(storageUpdate("hardware.systemInfo", types.HostSystemInfo{SerialNumber: "SN-old", OtherIdentifyingInfo: []types.HostSystemIdentificationInfo{{IdentifierType: &types.ElementDescription{Key: "ServiceTag"}, IdentifierValue: "TAG-old"}}}))
	a.Apply(storageUpdate("hardware.systemInfo", types.HostSystemInfo{}))
	storageFields(t, storageHostJSON(t, &a), map[string]any{"serialNumber": nil, "serviceTag": nil})
	a.Apply(types.ObjectUpdate{ChangeSet: []types.PropertyChange{{Name: "summary.hardware.cpuModel", Op: types.PropertyChangeOpRemove}}})
	storageFields(t, storageHostJSON(t, &a), map[string]any{"cpuModel": nil})
}

func TestHostIdentitySQLiteToREST(t *testing.T) {
	a := HostAdapter{model: model.Host{Base: model.Base{ID: "host-1"}}}
	a.Apply(storageUpdate("summary.hardware.cpuModel", "Xeon Gold"))
	a.Apply(storageUpdate("hardware.systemInfo", types.HostSystemInfo{SerialNumber: "SN-1", OtherIdentifyingInfo: []types.HostSystemIdentificationInfo{{IdentifierType: &types.ElementDescription{Key: "ServiceTag"}, IdentifierValue: "TAG-1"}}}))
	db := libmodel.New(filepath.Join(t.TempDir(), "inventory.db"), &model.Host{})
	if err := db.Open(true); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close(true) }()
	if err := db.Insert(&a.model); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(false); err != nil {
		t.Fatal(err)
	}
	if err := db.Open(false); err != nil {
		t.Fatal(err)
	}
	server := storageAPIServer(t, db)
	storageCheckAPI(t, server, "hosts", "host-1", func(t *testing.T, got map[string]any) {
		storageFields(t, got, map[string]any{"cpuModel": "Xeon Gold", "serialNumber": "SN-1", "serviceTag": "TAG-1"})
	})
}

func TestHostIdentityFromVCSim(t *testing.T) {
	ctx := context.Background()
	sim := simulator.VPX()
	if err := sim.Create(); err != nil {
		t.Fatal(err)
	}
	defer sim.Remove()
	host := sim.Map().Any("HostSystem").(*simulator.HostSystem)
	host.Summary.Hardware.CpuModel = "Simulated CPU"
	host.Hardware.SystemInfo.SerialNumber = "SN-sim"
	host.Hardware.SystemInfo.OtherIdentifyingInfo = []types.HostSystemIdentificationInfo{{IdentifierType: &types.ElementDescription{Key: "ServiceTag"}, IdentifierValue: "TAG-sim"}}
	server := sim.Service.NewServer()
	defer server.Close()
	client, err := govmomi.NewClient(ctx, server.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	pc := property.DefaultCollector(client.Client)
	filter, err := pc.CreateFilter(ctx, types.CreateFilter{Spec: types.PropertyFilterSpec{
		ObjectSet: []types.ObjectSpec{{Obj: host.Reference()}},
		PropSet:   []types.PropertySpec{{Type: "HostSystem", PathSet: []string{"summary.hardware.cpuModel", "hardware.systemInfo", "hardware.systemInfo.model", "hardware.systemInfo.vendor"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = filter.Destroy(ctx) }()
	var hostUpdate types.ObjectUpdate
	err = pc.WaitForUpdatesEx(ctx, &property.WaitOptions{}, func(updates []types.ObjectUpdate) bool {
		if len(updates) == 0 {
			return false
		}
		hostUpdate = updates[0]
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	a := HostAdapter{}
	a.Apply(hostUpdate)
	storageFields(t, storageHostJSON(t, &a), map[string]any{"cpuModel": "Simulated CPU", "serialNumber": "SN-sim", "serviceTag": "TAG-sim"})
}
