package vsphere

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	web "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/vmware/govmomi/vim25/types"
)

func TestGuestDiskSQLitePreservesReportedZero(t *testing.T) {
	for _, tc := range []struct {
		name           string
		capacity, free int64
	}{
		{"full disk", 1048576, 0},
		{"zero capacity", 0, 0},
		{"nonzero values", 2097152, 1048576},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := VmAdapter{model: model.VM{Base: model.Base{ID: "vm-1"}}}
			vm.Apply(storageUpdate(fGuestDisk, types.ArrayOfGuestDiskInfo{GuestDiskInfo: []types.GuestDiskInfo{{DiskPath: "/", Capacity: tc.capacity, FreeSpace: tc.free}}}))
			path := filepath.Join(t.TempDir(), "guest-disks.db")
			db := libmodel.New(path, &model.VM{})
			if err := db.Open(true); err != nil {
				t.Fatal(err)
			}
			if err := db.Insert(&vm.model); err != nil {
				_ = db.Close(true)
				t.Fatal(err)
			}
			if err := db.Close(false); err != nil {
				t.Fatal(err)
			}
			// Read the actual stored JSON; decoding into int64 fields would hide a lost zero.
			rawDB, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = rawDB.Close() }()
			var raw string
			if err := rawDB.QueryRow(`SELECT GuestDisks FROM VM WHERE ID='vm-1'`).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var disks []map[string]json.RawMessage
			if err := json.Unmarshal([]byte(raw), &disks); err != nil {
				t.Fatal(err)
			}
			if len(disks) != 1 {
				t.Fatalf("guest disk not retained: %s", raw)
			}
			for field, want := range map[string]int64{"capacity": tc.capacity, "freeSpace": tc.free} {
				value, present := disks[0][field]
				if !present {
					t.Errorf("reported %s=%d omitted from stored JSON: %s", field, want, raw)
					continue
				}
				var got *int64
				if err := json.Unmarshal(value, &got); err != nil {
					t.Fatalf("decode %s from stored JSON: %v", field, err)
				}
				if got == nil {
					t.Errorf("reported %s=%d stored as null: %s", field, want, raw)
					continue
				}
				if *got != want {
					t.Errorf("%s=%d, want %d", field, *got, want)
				}
			}
		})
	}
}

func TestGuestDiskHistoricalPresenceSurvivesUnrelatedUpdate(t *testing.T) {
	for _, tc := range []struct{ name, fields string }{
		{"both missing", ``},
		{"free missing", `,"capacity":1048576`},
		{"capacity missing", `,"freeSpace":0`},
		{"reported zero", `,"capacity":0,"freeSpace":0`},
		{"nonzero", `,"capacity":2097152,"freeSpace":1048576`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := `[{"key":2000,"diskPath":"/"` + tc.fields + `}]`
			path := filepath.Join(t.TempDir(), "historical.db")
			db := libmodel.New(path, &model.VM{})
			if err := db.Open(true); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close(true) }()
			vm := model.VM{Base: model.Base{ID: "vm-1"}, Host: "host-1"}
			if err := db.Insert(&vm); err != nil {
				t.Fatal(err)
			}
			if err := db.With(func(tx *libmodel.Tx) error {
				_, err := tx.Execute(`UPDATE VM SET GuestDisks='` + stored + `' WHERE ID='vm-1'`)
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
			// Exercise the real collector transaction for an unrelated property update.
			collector := Collector{}
			update := storageUpdate(fAnnotation, "unrelated update")
			update.Obj = types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-1"}
			if err := db.With(func(tx *libmodel.Tx) error {
				return collector.applyModify(tx, update)
			}); err != nil {
				t.Fatal(err)
			}

			var want any
			if err := json.Unmarshal([]byte(stored), &want); err != nil {
				t.Fatal(err)
			}
			loaded := model.VM{Base: model.Base{ID: "vm-1"}}
			if err := db.Get(&loaded); err != nil {
				t.Fatal(err)
			}
			if loaded.Annotation != "unrelated update" {
				t.Fatal("update did not persist")
			}
			var resource web.VM
			resource.With(&loaded)
			response, err := json.Marshal(resource.GuestDisks)
			if err != nil {
				t.Fatal(err)
			}
			var got any
			if err := json.Unmarshal(response, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("REST invented missing fields: got %s, want %s", response, stored)
			}
			rawDB, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = rawDB.Close() }()
			var raw string
			if err := rawDB.QueryRow(`SELECT GuestDisks FROM VM WHERE ID='vm-1'`).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(raw), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unrelated update invented stored fields: got %s, want %s", raw, stored)
			}
		})
	}
}
