package vsphere

import (
	"testing"

	forklift "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/storage/resolver/hpe"
	"github.com/kubev2v/forklift/pkg/storage/resolver/ontap"
)

func TestNewCsiImportPluginUnknownVendor(t *testing.T) {
	_, err := newCsiImportPlugin("nonexistent-vendor", "host", "user", "pass", false, nil)
	if err == nil {
		t.Fatal("expected error for unknown vendor, got nil")
	}
}

func TestNewCsiImportPluginHpe(t *testing.T) {
	plugin, err := newCsiImportPlugin(forklift.StorageVendorProductPrimera3Par, "https://host:8080", "user", "pass", false, nil)
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

func TestNewCsiImportPluginOntap(t *testing.T) {
	secretData := map[string][]byte{
		"ONTAP_SVM":            []byte("test-svm"),
		"TRIDENT_BACKEND_UUID": []byte("test-uuid"),
	}
	plugin, err := newCsiImportPlugin(forklift.StorageVendorProductOntap, "https://host", "user", "pass", true, secretData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plugin == nil {
		t.Fatal("expected non-nil plugin")
	}
	if _, ok := plugin.(*ontap.OntapImporter); !ok {
		t.Errorf("expected *ontap.OntapImporter, got %T", plugin)
	}
}

func TestNewCsiImportPluginOntapMissingKeys(t *testing.T) {
	_, err := newCsiImportPlugin(forklift.StorageVendorProductOntap, "https://host", "user", "pass", true, map[string][]byte{})
	if err == nil {
		t.Fatal("expected error for missing ONTAP keys, got nil")
	}
}
