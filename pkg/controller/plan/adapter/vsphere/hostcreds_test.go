package vsphere

import (
	"reflect"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	inventorymodel "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
)

func TestHostIDsForCopy(t *testing.T) {
	tests := []struct {
		name       string
		datastores []string
		host       string
		want       []string
	}{
		{"mixed mappings need both hosts", []string{"dedicated", "local"}, "vm-host", []string{"copy-host", "vm-host"}},
		{"unused local mapping needs no VM host", []string{"dedicated"}, "vm-host", []string{"copy-host"}},
		{"unused dedicated mapping needs no dedicated host", []string{"local"}, "vm-host", []string{"vm-host"}},
		{"unresolved VM host is reported", []string{"local"}, "", []string{""}},
		{"unresolved VM host is irrelevant with dedicated host", []string{"dedicated"}, "", []string{"copy-host"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := createBuilder()
			vm := model.VM{VM1: model.VM1{Host: tt.host}}
			for i, ds := range tt.datastores {
				vm.Disks = append(vm.Disks, inventorymodel.Disk{Datastore: inventorymodel.Ref{ID: ds}, Bus: inventorymodel.SCSI, Key: int32(i)})
			}
			builder.Source.Inventory = &mockInventory{datastores: map[string]model.Datastore{
				"dedicated": {Resource: model.Resource{ID: "dedicated"}},
				"local":     {Resource: model.Resource{ID: "local"}},
			}}
			builder.Map.Storage = &api.StorageMap{Spec: api.StorageMapSpec{Map: []api.StoragePair{
				{Source: ref.Ref{ID: "dedicated"}, OffloadPlugin: &api.OffloadPlugin{VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{DedicatedMigrationHosts: []string{"copy-host"}}}},
				{Source: ref.Ref{ID: "local"}, OffloadPlugin: &api.OffloadPlugin{VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{}}},
			}}}
			got, err := builder.HostIDsForCopy(&vm)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("hosts = %v, want %v", got, tt.want)
			}
		})
	}
}
