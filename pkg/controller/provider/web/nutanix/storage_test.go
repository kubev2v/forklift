package nutanix

import (
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestStorageContainer_With(t *testing.T) {
	m := &model.StorageContainer{
		Base:                 model.Base{ID: "sc-1", Name: "default-container-prod"},
		StorageContainerUUID: "sc-1",
		Cluster:              "cluster-1",
		ReplicationFactor:    2,
		MaxCapacityBytes:     1000,
		UsageBytes:           400,
		FreeBytes:            600,
		CompressionEnabled:   true,
		OnDiskDedup:          "POST_PROCESS",
		ErasureCode:          "off",
	}

	r := &StorageContainer{}
	r.With(m)

	if r.ID != m.ID || r.Name != m.Name {
		t.Errorf("expected base fields to be copied, got ID=%q Name=%q", r.ID, r.Name)
	}
	if r.StorageContainerUUID != m.StorageContainerUUID ||
		r.Cluster != m.Cluster ||
		r.ReplicationFactor != m.ReplicationFactor ||
		r.MaxCapacityBytes != m.MaxCapacityBytes ||
		r.UsageBytes != m.UsageBytes ||
		r.FreeBytes != m.FreeBytes ||
		r.CompressionEnabled != m.CompressionEnabled ||
		r.OnDiskDedup != m.OnDiskDedup ||
		r.ErasureCode != m.ErasureCode {
		t.Errorf("expected With() to copy every model field, got %+v", r)
	}
}

func TestStorageContainer_Link(t *testing.T) {
	provider := &api.Provider{ObjectMeta: meta.ObjectMeta{UID: types.UID("provider-1")}}
	r := &StorageContainer{}
	r.ID = "sc-1"
	r.Link(provider)

	if !strings.Contains(r.SelfLink, "provider-1") {
		t.Errorf("expected SelfLink to contain the provider UID, got %q", r.SelfLink)
	}
	if !strings.HasSuffix(r.SelfLink, "sc-1") {
		t.Errorf("expected SelfLink to end with the storage container ID, got %q", r.SelfLink)
	}
	if strings.Contains(r.SelfLink, ":") {
		t.Errorf("expected SelfLink to have all route placeholders substituted, got %q", r.SelfLink)
	}
}
