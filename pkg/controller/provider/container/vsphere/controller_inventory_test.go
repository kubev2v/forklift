package vsphere

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	web "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/vmware/govmomi/vim25/types"
)

func TestControllerInventoryCollection(t *testing.T) {
	for _, kind := range []string{"VirtualBusLogicController", "VirtualLsiLogicController", "VirtualLsiLogicSASController", "ParaVirtualSCSIController"} {
		for _, sharing := range []string{"", "noSharing", "virtualSharing", "physicalSharing", "futureSharing"} {
			t.Run(kind+"/"+sharing, func(t *testing.T) {
				scsi := types.VirtualSCSIController{VirtualController: types.VirtualController{VirtualDevice: types.VirtualDevice{Key: 1000}, BusNumber: 0, Device: []int32{2000, 2001}}, SharedBus: types.VirtualSCSISharing(sharing)}
				var device types.BaseVirtualDevice
				switch kind {
				case "VirtualBusLogicController":
					device = &types.VirtualBusLogicController{VirtualSCSIController: scsi}
				case "VirtualLsiLogicController":
					device = &types.VirtualLsiLogicController{VirtualSCSIController: scsi}
				case "VirtualLsiLogicSASController":
					device = &types.VirtualLsiLogicSASController{VirtualSCSIController: scsi}
				case "ParaVirtualSCSIController":
					device = &types.ParaVirtualSCSIController{VirtualSCSIController: scsi}
				}
				v := VmAdapter{}
				v.updateControllers(&types.ArrayOfVirtualDevice{VirtualDevice: []types.BaseVirtualDevice{device, &types.VirtualDisk{VirtualDevice: types.VirtualDevice{Key: 2000}}, &types.VirtualDisk{VirtualDevice: types.VirtualDevice{Key: 2001}}, &types.VirtualVmxnet3{}}})
				if len(v.model.Controllers) != 1 {
					t.Fatalf("got %d controllers including non-controller devices", len(v.model.Controllers))
				}
				c := v.model.Controllers[0]
				if c.Key != 1000 || c.Bus != SCSI || c.BusNumber != 0 || !reflect.DeepEqual(c.Disks, []int32{2000, 2001}) {
					t.Fatalf("controller identity changed: %+v", c)
				}
				data, err := json.Marshal(c)
				if err != nil {
					t.Fatal(err)
				}
				var got map[string]any
				if err := json.Unmarshal(data, &got); err != nil {
					t.Fatal(err)
				}
				if got["type"] != kind {
					t.Errorf("type = %v, want %s", got["type"], kind)
				}
				if sharing == "" {
					if _, exists := got["busSharing"]; exists {
						t.Errorf("unknown sharing emitted: %s", data)
					}
				} else if got["busSharing"] != sharing {
					t.Errorf("busSharing = %v, want %s", got["busSharing"], sharing)
				}
				v.updateControllers(&types.ArrayOfVirtualDevice{})
				if len(v.model.Controllers) != 0 {
					t.Fatal("removed controllers retained")
				}
			})
		}
	}
}

func TestControllerInventoryNonSCSIAndRefresh(t *testing.T) {
	for _, tc := range []struct {
		name, bus string
		device    types.BaseVirtualDevice
	}{
		{"IDE", IDE, &types.VirtualIDEController{}},
		{"SATA", SATA, &types.VirtualAHCIController{}},
		{"USB", USB, &types.VirtualUSBController{}},
		{"NVME", NVME, &types.VirtualNVMEController{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := VmAdapter{}
			v.updateControllers(&types.ArrayOfVirtualDevice{VirtualDevice: []types.BaseVirtualDevice{&types.ParaVirtualSCSIController{VirtualSCSIController: types.VirtualSCSIController{SharedBus: types.VirtualSCSISharingPhysicalSharing}}}})
			v.updateControllers(&types.ArrayOfVirtualDevice{VirtualDevice: []types.BaseVirtualDevice{tc.device}})
			if len(v.model.Controllers) != 1 || v.model.Controllers[0].Bus != tc.bus {
				t.Fatalf("controller lost: %+v", v.model.Controllers)
			}
			data, err := json.Marshal(v.model.Controllers[0])
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got["type"] != nil || got["busSharing"] != nil {
				t.Fatalf("SCSI facts leaked on refresh: %s", data)
			}
		})
	}
}

func TestControllerInventorySQLiteToREST(t *testing.T) {
	for _, tc := range []struct {
		name, stored string
		want         []any
	}{
		{"current", `[{"key":1000,"bus":"scsi","busNumber":0,"Disks":[2000],"type":"ParaVirtualSCSIController","busSharing":"noSharing"},{"key":1001,"bus":"scsi","busNumber":2,"Disks":[2001],"type":"VirtualLsiLogicController","busSharing":"physicalSharing"}]`, []any{map[string]any{"key": float64(1000), "bus": "scsi", "busNumber": float64(0), "diskKeys": []any{float64(2000)}, "type": "ParaVirtualSCSIController", "busSharing": "noSharing"}, map[string]any{"key": float64(1001), "bus": "scsi", "busNumber": float64(2), "diskKeys": []any{float64(2001)}, "type": "VirtualLsiLogicController", "busSharing": "physicalSharing"}}},
		{"historical", `[{"key":1000,"bus":"scsi","busNumber":0,"Disks":[2000]}]`, []any{map[string]any{"key": float64(1000), "bus": "scsi", "busNumber": float64(0), "diskKeys": []any{float64(2000)}}}},
		{"empty", `[]`, nil},
		{"null", `null`, nil},
		{"historical synthetic device", `[{"key":0,"bus":"","busNumber":0,"Disks":null}]`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := libmodel.New(filepath.Join(t.TempDir(), "controllers.db"), &model.VM{})
			if err := db.Open(true); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close(true) }()
			vm := model.VM{Base: model.Base{ID: "vm-1", Name: "controller-vm"}, Host: "host-1", Disks: []model.Disk{{Key: 2000}, {Key: 2001}}}
			if err := db.Insert(&vm); err != nil {
				t.Fatal(err)
			}
			// Seed the on-disk JSON directly, including the historical spelling of Disks.
			if err := db.With(func(tx *libmodel.Tx) error {
				_, err := tx.Execute(fmt.Sprintf("UPDATE VM SET Controllers = '%s' WHERE ID = 'vm-1'", tc.stored))
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(false); err != nil {
				t.Fatal(err)
			}
			if err := db.Open(false); err != nil {
				t.Fatal(err)
			}
			loaded := model.VM{Base: model.Base{ID: "vm-1"}}
			if err := db.Get(&loaded); err != nil {
				t.Fatal(err)
			}
			var resource web.VM
			resource.With(&loaded)
			data, err := json.Marshal(resource)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got["controllers"], any(tc.want)) {
				// A nil slice is omitted rather than emitted as null.
				if tc.want != nil || got["controllers"] != nil {
					t.Fatalf("controllers = %#v, want %#v; JSON %s", got["controllers"], tc.want, data)
				}
			}
			if len(loaded.Controllers) > 0 && loaded.Controllers[0].Bus != "" {
				b, err := json.Marshal(loaded.Controllers)
				if err != nil {
					t.Fatal(err)
				}
				var old []struct {
					Key       int32  `json:"key"`
					BusNumber int32  `json:"busNumber"`
					Bus       string `json:"bus"`
					Disks     []int32
				}
				if err := json.Unmarshal(b, &old); err != nil {
					t.Fatal(err)
				}
				if old[0].Key != 1000 || old[0].Bus != "scsi" || !reflect.DeepEqual(old[0].Disks, []int32{2000}) {
					t.Fatalf("old reader lost fields: %+v", old)
				}
			}
		})
	}
}

func TestControllerInventoryCollectedSQLiteHTTP(t *testing.T) {
	v := VmAdapter{model: model.VM{Base: model.Base{ID: "vm-1", Name: "controller-vm"}, Host: "host-1"}}
	v.updateControllers(&types.ArrayOfVirtualDevice{VirtualDevice: []types.BaseVirtualDevice{
		&types.VirtualDisk{VirtualDevice: types.VirtualDevice{Key: 2000}},
		&types.VirtualDisk{VirtualDevice: types.VirtualDevice{Key: 2001}},
		&types.ParaVirtualSCSIController{VirtualSCSIController: types.VirtualSCSIController{VirtualController: types.VirtualController{VirtualDevice: types.VirtualDevice{Key: 1000}, BusNumber: 0, Device: []int32{2000}}, SharedBus: types.VirtualSCSISharingNoSharing}},
		&types.VirtualLsiLogicSASController{VirtualSCSIController: types.VirtualSCSIController{VirtualController: types.VirtualController{VirtualDevice: types.VirtualDevice{Key: 1001}, BusNumber: 2, Device: []int32{2001}}, SharedBus: types.VirtualSCSISharingPhysicalSharing}},
	}})
	db := libmodel.New(filepath.Join(t.TempDir(), "collected.db"), &model.VM{})
	if err := db.Open(true); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close(true) }()
	if err := db.Insert(&v.model); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(false); err != nil {
		t.Fatal(err)
	}
	if err := db.Open(false); err != nil {
		t.Fatal(err)
	}
	server := storageAPIServer(t, db)
	storageCheckAPI(t, server, "vms", "vm-1", func(t *testing.T, got map[string]any) {
		controllers, ok := got["controllers"].([]any)
		if !ok || len(controllers) != 2 {
			t.Fatalf("controllers missing after persistence: %#v", got["controllers"])
		}
		storageFields(t, controllers[0].(map[string]any), map[string]any{"key": float64(1000), "bus": "scsi", "busNumber": float64(0), "type": "ParaVirtualSCSIController", "busSharing": "noSharing", "diskKeys": []any{float64(2000)}})
		storageFields(t, controllers[1].(map[string]any), map[string]any{"key": float64(1001), "bus": "scsi", "busNumber": float64(2), "type": "VirtualLsiLogicSASController", "busSharing": "physicalSharing", "diskKeys": []any{float64(2001)}})
	})
}
