package hyperv

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kubev2v/forklift/pkg/lib/hyperv/driver"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
)

func staticIPGuestNetworks() []model.GuestNetwork {
	return []model.GuestNetwork{{
		MAC:          "00:15:5d:01:01:01",
		IP:           "192.168.1.10",
		DeviceIndex:  0,
		Origin:       model.OriginManual,
		PrefixLength: 24,
		DNS:          []string{"8.8.8.8"},
		Gateway:      "192.168.1.1",
	}}
}

func TestPreserveVMFields_GuestDataPreservedOutsideLightMode(t *testing.T) {
	prev := &model.VM{
		GuestOS:       "Red Hat Enterprise Linux 9",
		GuestNetworks: staticIPGuestNetworks(),
	}

	m := &model.VM{
		GuestOS:       "",
		GuestNetworks: nil,
	}

	preserveVMFields(m, prev, false /* lightMode */)

	if !reflect.DeepEqual(m.GuestNetworks, prev.GuestNetworks) {
		t.Errorf("GuestNetworks should be preserved on a non-LightMode cycle, got %+v, want %+v",
			m.GuestNetworks, prev.GuestNetworks)
	}
	if m.GuestOS != prev.GuestOS {
		t.Errorf("GuestOS should be preserved on a non-LightMode cycle, got %q, want %q",
			m.GuestOS, prev.GuestOS)
	}
}

func TestPreserveVMFields_GuestDataPreservedInLightMode(t *testing.T) {
	prev := &model.VM{
		GuestOS:       "Red Hat Enterprise Linux 9",
		GuestNetworks: staticIPGuestNetworks(),
	}

	m := &model.VM{
		GuestOS:       "",
		GuestNetworks: nil,
	}

	preserveVMFields(m, prev, true /* lightMode */)

	if !reflect.DeepEqual(m.GuestNetworks, prev.GuestNetworks) {
		t.Errorf("GuestNetworks should be preserved in LightMode, got %+v, want %+v",
			m.GuestNetworks, prev.GuestNetworks)
	}
	if m.GuestOS != prev.GuestOS {
		t.Errorf("GuestOS should be preserved in LightMode, got %q, want %q",
			m.GuestOS, prev.GuestOS)
	}
}

func TestPreserveVMFields_NewGuestDataOverwritesExisting(t *testing.T) {
	tests := []struct {
		name      string
		lightMode bool
	}{
		{name: "non-LightMode cycle", lightMode: false},
		{name: "LightMode cycle", lightMode: true},
	}

	prev := &model.VM{
		GuestOS:       "Red Hat Enterprise Linux 9",
		GuestNetworks: staticIPGuestNetworks(),
	}

	newGuestNetworks := []model.GuestNetwork{{
		MAC:          "00:15:5d:02:02:02",
		IP:           "192.168.1.20",
		DeviceIndex:  0,
		Origin:       model.OriginManual,
		PrefixLength: 24,
		DNS:          []string{"1.1.1.1"},
		Gateway:      "192.168.1.1",
	}}
	newGuestOS := "Microsoft Windows Server 2022 Standard"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model.VM{
				GuestOS:       newGuestOS,
				GuestNetworks: newGuestNetworks,
			}

			preserveVMFields(m, prev, tt.lightMode)

			if !reflect.DeepEqual(m.GuestNetworks, newGuestNetworks) {
				t.Errorf("GuestNetworks should not be overwritten by stale data, got %+v, want %+v",
					m.GuestNetworks, newGuestNetworks)
			}
			if m.GuestOS != newGuestOS {
				t.Errorf("GuestOS should not be overwritten by stale data, got %q, want %q",
					m.GuestOS, newGuestOS)
			}
		})
	}
}

func TestPreserveVMFields_SecurityAndDiskOnlyPreservedInLightMode(t *testing.T) {
	prev := &model.VM{
		TpmEnabled: true,
		SecureBoot: true,
		Disks: []model.Disk{
			{Base: model.Base{ID: "disk-1"}, Capacity: 107374182400, RCTEnabled: true},
		},
	}

	t.Run("LightMode preserves TPM/SecureBoot/disk data", func(t *testing.T) {
		m := &model.VM{
			TpmEnabled: false,
			SecureBoot: false,
			Disks: []model.Disk{
				{Base: model.Base{ID: "disk-1"}, Capacity: 0, RCTEnabled: false},
			},
		}

		preserveVMFields(m, prev, true)

		if !m.TpmEnabled {
			t.Error("TpmEnabled should be preserved in LightMode")
		}
		if !m.SecureBoot {
			t.Error("SecureBoot should be preserved in LightMode")
		}
		if m.Disks[0].Capacity != 107374182400 {
			t.Errorf("Disk capacity should be preserved in LightMode, got %d", m.Disks[0].Capacity)
		}
		if !m.Disks[0].RCTEnabled {
			t.Error("Disk RCTEnabled should be preserved in LightMode")
		}
	})

	t.Run("non-LightMode does not preserve stale TPM/SecureBoot/disk data", func(t *testing.T) {
		m := &model.VM{
			TpmEnabled: false,
			SecureBoot: false,
			Disks: []model.Disk{
				{Base: model.Base{ID: "disk-1"}, Capacity: 0, RCTEnabled: false},
			},
		}

		preserveVMFields(m, prev, false)

		if m.TpmEnabled {
			t.Error("TpmEnabled should reflect the full-refresh value, not be preserved")
		}
		if m.SecureBoot {
			t.Error("SecureBoot should reflect the full-refresh value, not be preserved")
		}
		if m.Disks[0].Capacity != 0 {
			t.Error("Disk capacity should reflect the full-refresh value, not be preserved")
		}
		if m.Disks[0].RCTEnabled {
			t.Error("Disk RCTEnabled should reflect the full-refresh value, not be preserved")
		}
	})
}

func TestPreserveVMFields_UnmatchedDiskIDIsUnaffected(t *testing.T) {
	prev := &model.VM{
		Disks: []model.Disk{
			{Base: model.Base{ID: "disk-1"}, Capacity: 107374182400, RCTEnabled: true},
		},
	}

	m := &model.VM{
		Disks: []model.Disk{
			{Base: model.Base{ID: "disk-1"}, Capacity: 0, RCTEnabled: false},
			{Base: model.Base{ID: "disk-2"}, Capacity: 0, RCTEnabled: false},
		},
	}

	preserveVMFields(m, prev, true /* lightMode */)

	if m.Disks[0].Capacity != 107374182400 || !m.Disks[0].RCTEnabled {
		t.Errorf("disk-1 should still be preserved from prev, got %+v", m.Disks[0])
	}
	if m.Disks[1].Capacity != 0 {
		t.Errorf("disk-2 (no match in prev) should be left as freshly collected, got capacity %d", m.Disks[1].Capacity)
	}
	if m.Disks[1].RCTEnabled {
		t.Error("disk-2 (no match in prev) should be left as freshly collected, got RCTEnabled=true")
	}
}

type mockVMDomain struct {
	uuid string
	name string
}

func (d *mockVMDomain) GetName() (string, error)       { return d.name, nil }
func (d *mockVMDomain) GetUUIDString() (string, error) { return d.uuid, nil }
func (d *mockVMDomain) GetState() (driver.DomainState, int, error) {
	return driver.DOMAIN_SHUTOFF, 0, nil
}
func (d *mockVMDomain) GetInfo() (*driver.DomainInfo, error) {
	return &driver.DomainInfo{State: driver.DOMAIN_SHUTOFF, NrVirtCpu: 2, Memory: 4 * 1024 * 1024}, nil
}
func (d *mockVMDomain) GetGeneration() (int, error)          { return 1, nil }
func (d *mockVMDomain) GetDisks() ([]driver.DiskInfo, error) { return nil, nil }
func (d *mockVMDomain) GetNICs() ([]driver.NICInfo, error)   { return nil, nil }
func (d *mockVMDomain) GetComputerName() string              { return "" }
func (d *mockVMDomain) Shutdown(ctx context.Context) error   { return nil }
func (d *mockVMDomain) Free() error                          { return nil }

func newTestHyperVDB(t *testing.T) libmodel.DB {
	t.Helper()
	db := libmodel.New(filepath.Join(t.TempDir(), "test.db"), &model.VM{})
	if err := db.Open(true); err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close(true) })
	return db
}

func TestVMAdapter_GetUpdates_PreservesGuestDataOutsideLightMode(t *testing.T) {
	const vmUUID = "11111111-1111-1111-1111-111111111111"

	db := newTestHyperVDB(t)

	existingGuestNetworks := staticIPGuestNetworks()
	seed := &model.VM{
		Base:          model.Base{ID: vmUUID, Name: "test-vm"},
		UUID:          vmUUID,
		GuestOS:       "Red Hat Enterprise Linux 9",
		GuestNetworks: existingGuestNetworks,
	}
	if err := db.Insert(seed); err != nil {
		t.Fatalf("failed to seed VM record: %v", err)
	}

	batchJSON := `{"test-vm":{"Security":{"TpmEnabled":false,"SecureBoot":false},"HasCheckpoint":false,"Disks":[],"GuestOS":"","GuestNetworks":null}}`
	md := &mockDriver{
		listAllDomainsFn: func() ([]driver.Domain, error) {
			return []driver.Domain{&mockVMDomain{uuid: vmUUID, name: "test-vm"}}, nil
		},
		runOnNodeFn: func(command, computerName string) (string, error) {
			return batchJSON, nil
		},
	}
	client := &Client{
		driver:    md,
		provider:  newStandaloneProvider(),
		Log:       testLogger(),
		LightMode: false,
	}

	ctx := &Context{client: client, db: db, log: testLogger(), ctx: context.Background()}
	adapter := &VMAdapter{}

	updates, err := adapter.GetUpdates(ctx)
	if err != nil {
		t.Fatalf("GetUpdates returned error: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 updater, got %d", len(updates))
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("db.Begin returned error: %v", err)
	}
	for _, u := range updates {
		if err := u(tx); err != nil {
			t.Fatalf("updater returned error: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("tx.Commit returned error: %v", err)
	}

	got := &model.VM{Base: model.Base{ID: vmUUID}}
	if err := db.Get(got); err != nil {
		t.Fatalf("db.Get returned error: %v", err)
	}

	if got.GuestOS != seed.GuestOS {
		t.Errorf("GuestOS should be preserved end-to-end, got %q, want %q", got.GuestOS, seed.GuestOS)
	}
	if !reflect.DeepEqual(got.GuestNetworks, existingGuestNetworks) {
		t.Errorf("GuestNetworks should be preserved end-to-end, got %+v, want %+v", got.GuestNetworks, existingGuestNetworks)
	}
}
