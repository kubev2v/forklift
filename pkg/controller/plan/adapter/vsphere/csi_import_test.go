package vsphere

import (
	"testing"

	forklift "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/storage/resolver/hpe"
)

func TestNewCsiImportPluginUnknownVendor(t *testing.T) {
	_, err := newCsiImportPlugin("nonexistent-vendor", "host", "user", "pass", false)
	if err == nil {
		t.Fatal("expected error for unknown vendor, got nil")
	}
}

func TestNewCsiImportPluginHpe(t *testing.T) {
	plugin, err := newCsiImportPlugin(forklift.StorageVendorProductPrimera3Par, "https://host:8080", "user", "pass", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plugin == nil {
		t.Fatal("expected non-nil plugin")
	}
	if _, ok := plugin.(*hpe.HpeImporter); !ok {
		t.Errorf("expected *hpe.HpeImporter, got %T", plugin)
	}
}
