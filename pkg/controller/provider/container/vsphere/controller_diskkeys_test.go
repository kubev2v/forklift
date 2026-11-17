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

func TestControllerDiskKeysExcludeOtherDevices(t *testing.T) {
	for _, bus := range []string{IDE, SATA} {
		for _, diskPresent := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/diskPresent=%t", bus, diskPresent), func(t *testing.T) {
				members := []int32{3000}
				devices := []types.BaseVirtualDevice{&types.VirtualCdrom{VirtualDevice: types.VirtualDevice{Key: 3000, ControllerKey: 1000}}}
				if diskPresent {
					members = append(members, 2000)
					devices = append(devices, &types.VirtualDisk{VirtualDevice: types.VirtualDevice{Key: 2000, ControllerKey: 1000}})
				}
				controller := types.VirtualController{VirtualDevice: types.VirtualDevice{Key: 1000}, Device: members}
				if bus == IDE {
					devices = append(devices, &types.VirtualIDEController{VirtualController: controller})
				} else {
					devices = append(devices, &types.VirtualAHCIController{VirtualSATAController: types.VirtualSATAController{VirtualController: controller}})
				}
				v := VmAdapter{model: model.VM{Base: model.Base{ID: "vm-1"}, Host: "host-1"}}
				v.updateControllers(&types.ArrayOfVirtualDevice{VirtualDevice: devices})
				if len(v.model.Controllers) != 1 || !reflect.DeepEqual(v.model.Controllers[0].Disks, members) {
					t.Fatalf("legacy raw device list changed: %+v", v.model.Controllers)
				}
				db := libmodel.New(filepath.Join(t.TempDir(), "controllers.db"), &model.VM{})
				if err := db.Open(true); err != nil {
					t.Fatal(err)
				}
				defer func() { _ = db.Close(true) }()
				if err := db.Insert(&v.model); err != nil {
					t.Fatal(err)
				}
				loaded := model.VM{Base: model.Base{ID: "vm-1"}}
				if err := db.Get(&loaded); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(loaded.Controllers[0].Disks, members) {
					t.Fatalf("stored raw controller list changed: %+v", loaded.Controllers)
				}
				// No model.Disks fallback: fresh collection retains typed disk identity itself.
				var r web.VM
				r.With(&loaded)
				raw, err := json.Marshal(r.Controllers)
				if err != nil {
					t.Fatal(err)
				}
				var records []map[string]any
				if err := json.Unmarshal(raw, &records); err != nil {
					t.Fatal(err)
				}
				want := []any{}
				if diskPresent {
					want = append(want, float64(2000))
				}
				if len(records) != 1 || !reflect.DeepEqual(records[0]["diskKeys"], want) {
					t.Fatalf("diskKeys must contain disks only, including []: got %s, want %v", raw, want)
				}
			})
		}
	}
}

func TestHistoricalControllerDiskKeysUseFullDiskInventory(t *testing.T) {
	m := model.VM{Controllers: []model.Controller{{Key: 1000, Bus: SATA, Disks: []int32{3000, 2000, 2001}}}, Disks: []model.Disk{{Key: 2000, Bus: SATA, BusAddress: "sata0:0"}, {Key: 2001, Bus: SATA, BusAddress: "sata0:1"}}}
	var r web.VM
	r.With(&m)
	r.RemoveExcludedDisks([]string{"sata0:1"})
	if len(r.Disks) != 1 {
		t.Fatalf("expected existing disk filter to remove one disk, got %+v", r.Disks)
	}
	if len(r.Controllers) != 1 || !reflect.DeepEqual(r.Controllers[0].DiskKeys, []int32{2000, 2001}) {
		t.Fatalf("historical association includes non-disk or loses filtered disk: %+v", r.Controllers)
	}
}
