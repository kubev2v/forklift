package vmware

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vmware/govmomi/vim25/types"
)

func TestBuildHBAMap(t *testing.T) {
	storageDevice := &types.HostStorageDeviceInfo{
		HostBusAdapter: []types.BaseHostHostBusAdapter{
			&types.HostInternetScsiHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key:    "key-vim.host.InternetScsiHba-vmhba64",
					Device: "vmhba64",
					Driver: "bnx2i",
				},
				IScsiName: "iqn.1998-01.com.vmware:esxi-host-1",
			},
			&types.HostFibreChannelHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key:    "key-vim.host.FibreChannelHba-vmhba2",
					Device: "vmhba2",
					Driver: "lpfc",
				},
				NodeWorldWideName: 0x2000000000000001,
				PortWorldWideName: 0x2100000000000001,
			},
			&types.HostBlockHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key:    "key-vim.host.BlockHba-vmhba0",
					Device: "vmhba0",
					Driver: "ahci",
				},
			},
		},
	}

	hbaMap := buildHBAMap(storageDevice)

	assert.Len(t, hbaMap, 3)

	iscsi := hbaMap["key-vim.host.InternetScsiHba-vmhba64"]
	assert.Equal(t, "iqn.1998-01.com.vmware:esxi-host-1", iscsi.Id)
	assert.Equal(t, "vmhba64", iscsi.Name)

	fc := hbaMap["key-vim.host.FibreChannelHba-vmhba2"]
	assert.Equal(t, "fc.2000000000000001:2100000000000001", fc.Id)
	assert.Equal(t, "vmhba2", fc.Name)

	block := hbaMap["key-vim.host.BlockHba-vmhba0"]
	assert.Equal(t, "vmhba0", block.Id)
}

func TestPickFirstSANAdapter_FC(t *testing.T) {
	hbaByKey := map[string]HostAdapter{
		"key-block": {Name: "vmhba0", Id: "vmhba0", Driver: "ahci"},
		"key-fc":    {Name: "vmhba2", Id: "fc.2000000000000001:2100000000000001", Driver: "lpfc"},
	}

	result, err := pickFirstSANAdapter(hbaByKey, "", "test-ds")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "fc.2000000000000001:2100000000000001", result[0].Id)
}

func TestPickFirstSANAdapter_ISCSI(t *testing.T) {
	hbaByKey := map[string]HostAdapter{
		"key-block": {Name: "vmhba0", Id: "vmhba0", Driver: "ahci"},
		"key-iscsi": {Name: "vmhba64", Id: "iqn.1998-01.com.vmware:esxi-host-1", Driver: "bnx2i"},
	}

	result, err := pickFirstSANAdapter(hbaByKey, "", "test-ds")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "iqn.1998-01.com.vmware:esxi-host-1", result[0].Id)
}

func TestPickFirstSANAdapter_NoSAN(t *testing.T) {
	hbaByKey := map[string]HostAdapter{
		"key-block": {Name: "vmhba0", Id: "vmhba0", Driver: "ahci"},
	}

	_, err := pickFirstSANAdapter(hbaByKey, "", "test-ds")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no FC or iSCSI adapters found")
}

func TestPickFirstSANAdapter_FCPreferredOverISCSI(t *testing.T) {
	hbaByKey := map[string]HostAdapter{
		"key-iscsi": {Name: "vmhba64", Id: "iqn.1998-01.com.vmware:esxi-host-1", Driver: "bnx2i"},
		"key-fc":    {Name: "vmhba2", Id: "fc.2000000000000001:2100000000000001", Driver: "lpfc"},
	}

	result, err := pickFirstSANAdapter(hbaByKey, "", "test-ds")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "fc.2000000000000001:2100000000000001", result[0].Id)
}

func TestPickFirstSANAdapter_SciniGuidOverride(t *testing.T) {
	hbaByKey := map[string]HostAdapter{
		"key-fc": {Name: "vmhba2", Id: "fc.2000000000000001:2100000000000001", Driver: "scini"},
	}

	result, err := pickFirstSANAdapter(hbaByKey, "powerflex-guid-123", "test-ds")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "powerflex-guid-123", result[0].Id)
}

func TestVvolPEUUIDs(t *testing.T) {
	hostRef := types.ManagedObjectReference{Type: "HostSystem", Value: "host-12"}
	otherHostRef := types.ManagedObjectReference{Type: "HostSystem", Value: "host-99"}

	info := &types.VvolDatastoreInfo{
		VvolDS: &types.HostVvolVolume{
			HostPE: []types.VVolHostPE{
				{
					Key: otherHostRef,
					ProtocolEndpoint: []types.HostProtocolEndpoint{
						{Uuid: "naa.wrong-host-pe", Type: "scsi"},
					},
				},
				{
					Key: hostRef,
					ProtocolEndpoint: []types.HostProtocolEndpoint{
						{Uuid: "naa.68ccf098001e55443ef305ec9860e6d5", Type: "scsi"},
						{Uuid: "naa.68ccf098002aabbccddeeff000111222", Type: "scsi"},
					},
				},
			},
		},
	}

	uuids := vvolPEUUIDs(info, hostRef)
	assert.Len(t, uuids, 2)
	assert.Contains(t, uuids, "naa.68ccf098001e55443ef305ec9860e6d5")
	assert.Contains(t, uuids, "naa.68ccf098002aabbccddeeff000111222")
}

func TestVvolPEUUIDs_NoMatchingHost(t *testing.T) {
	hostRef := types.ManagedObjectReference{Type: "HostSystem", Value: "host-12"}

	info := &types.VvolDatastoreInfo{
		VvolDS: &types.HostVvolVolume{
			HostPE: []types.VVolHostPE{
				{
					Key: types.ManagedObjectReference{Type: "HostSystem", Value: "host-99"},
					ProtocolEndpoint: []types.HostProtocolEndpoint{
						{Uuid: "naa.somepe", Type: "scsi"},
					},
				},
			},
		},
	}

	uuids := vvolPEUUIDs(info, hostRef)
	assert.Empty(t, uuids)
}

func TestVVolActiveAdapterViaPE(t *testing.T) {
	// Simulates the full VVol flow: PE UUID → ScsiLun key → multipath → active adapter
	peUUID := "naa.68ccf098001e55443ef305ec9860e6d5"

	storageDevice := &types.HostStorageDeviceInfo{
		HostBusAdapter: []types.BaseHostHostBusAdapter{
			&types.HostBlockHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key:    "key-block",
					Device: "vmhba0",
					Driver: "ahci",
				},
			},
			&types.HostInternetScsiHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key:    "key-iscsi",
					Device: "vmhba64",
					Driver: "bnx2i",
				},
				IScsiName: "iqn.1998-01.com.vmware:esxi-host-1",
			},
			&types.HostFibreChannelHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key:    "key-fc-disconnected",
					Device: "vmhba3",
					Driver: "lpfc",
				},
				NodeWorldWideName: 0x2000000000000099,
				PortWorldWideName: 0x2100000000000099,
			},
		},
		ScsiLun: []types.BaseScsiLun{
			&types.ScsiLun{
				Key:           "key-pe-lun",
				CanonicalName: peUUID,
			},
		},
		MultipathInfo: &types.HostMultipathInfo{
			Lun: []types.HostMultipathInfoLogicalUnit{
				{
					Lun: "key-pe-lun",
					Path: []types.HostMultipathInfoPath{
						{
							Name:    "iscsi-path-to-pe",
							State:   "active",
							Adapter: "key-iscsi",
						},
						{
							Name:    "fc-path-dead",
							State:   "dead",
							Adapter: "key-fc-disconnected",
						},
					},
				},
			},
		},
	}

	// Step 1: vvolPEUUIDs would return [peUUID]
	deviceNames := []string{peUUID}

	// Step 2: find ScsiLun keys matching PE UUIDs
	deviceNameSet := make(map[string]bool)
	for _, d := range deviceNames {
		deviceNameSet[d] = true
	}
	scsiLunKeys := make(map[string]bool)
	for _, lun := range storageDevice.ScsiLun {
		if deviceNameSet[lun.GetScsiLun().CanonicalName] {
			scsiLunKeys[lun.GetScsiLun().Key] = true
		}
	}
	assert.True(t, scsiLunKeys["key-pe-lun"])

	// Step 3: find active adapters via multipath
	hbaByKey := buildHBAMap(storageDevice)
	activeAdapters := make(map[string]HostAdapter)
	for _, lun := range storageDevice.MultipathInfo.Lun {
		if !scsiLunKeys[lun.Lun] {
			continue
		}
		for _, path := range lun.Path {
			if path.State == "active" {
				if adapter, ok := hbaByKey[path.Adapter]; ok {
					activeAdapters[adapter.Name] = adapter
				}
			}
		}
	}

	// Only the iSCSI adapter should be found (FC path is dead)
	assert.Len(t, activeAdapters, 1)
	adapter, ok := activeAdapters["vmhba64"]
	assert.True(t, ok)
	assert.Equal(t, "iqn.1998-01.com.vmware:esxi-host-1", adapter.Id)
}

func TestGetDatastoreActiveAdapters_LocalDSWithFCFallback(t *testing.T) {
	storageDevice := &types.HostStorageDeviceInfo{
		HostBusAdapter: []types.BaseHostHostBusAdapter{
			&types.HostBlockHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key:    "key-block",
					Device: "vmhba0",
					Driver: "ahci",
				},
			},
			&types.HostFibreChannelHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key:    "key-fc",
					Device: "vmhba2",
					Driver: "lpfc",
				},
				NodeWorldWideName: 0x2000000000000001,
				PortWorldWideName: 0x2100000000000001,
			},
		},
		MultipathInfo: &types.HostMultipathInfo{
			Lun: []types.HostMultipathInfoLogicalUnit{
				{
					Lun: "key-scsilun-local",
					Path: []types.HostMultipathInfoPath{
						{
							Name:    "local-path",
							State:   "active",
							Adapter: "key-block",
						},
					},
				},
			},
		},
		ScsiLun: []types.BaseScsiLun{
			&types.ScsiLun{
				Key:           "key-scsilun-local",
				CanonicalName: "naa.localdevice123",
			},
		},
	}

	hbaByKey := buildHBAMap(storageDevice)

	// Simulate what GetDatastoreActiveAdapters does for a local datastore:
	// active paths only have block adapters, no SAN adapters found,
	// so it falls back to first available FC adapter.
	activeAdapters := make(map[string]HostAdapter)
	for _, lun := range storageDevice.MultipathInfo.Lun {
		if lun.Lun == "key-scsilun-local" {
			for _, path := range lun.Path {
				if path.State == "active" {
					if adapter, ok := hbaByKey[path.Adapter]; ok {
						activeAdapters[adapter.Name] = adapter
					}
				}
			}
		}
	}

	// Verify active paths only have block adapter
	assert.Len(t, activeAdapters, 1)
	_, hasBlock := activeAdapters["vmhba0"]
	assert.True(t, hasBlock)

	// The fallback should find the FC adapter
	result, err := pickFirstSANAdapter(hbaByKey, "", "local-ds")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "fc.2000000000000001:2100000000000001", result[0].Id)
}
