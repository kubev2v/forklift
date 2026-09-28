package vsphere

import (
	"testing"

	forklift "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/storage/resolver/hpe"
	"github.com/kubev2v/forklift/pkg/storage/resolver/ontap"
)

func TestNewCsiImportPluginUnknownVendor(t *testing.T) {
	_, err := newCsiImportPlugin("nonexistent-vendor", map[string][]byte{
		"STORAGE_HOSTNAME": []byte("https://host"),
		"STORAGE_USERNAME": []byte("user"),
		"STORAGE_PASSWORD": []byte("pass"),
	}, nil, "")
	if err == nil {
		t.Fatal("expected error for unknown vendor, got nil")
	}
}

func TestNewCsiImportPluginHpe(t *testing.T) {
	plugin, err := newCsiImportPlugin(forklift.StorageVendorProductPrimera3Par, map[string][]byte{
		"STORAGE_HOSTNAME": []byte("https://host:8080"),
		"STORAGE_USERNAME": []byte("user"),
		"STORAGE_PASSWORD": []byte("pass"),
	}, nil, "")
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
	plugin, err := newCsiImportPlugin(forklift.StorageVendorProductOntap, map[string][]byte{
		"STORAGE_HOSTNAME":              []byte("https://host"),
		"STORAGE_USERNAME":              []byte("user"),
		"STORAGE_PASSWORD":              []byte("pass"),
		"STORAGE_SKIP_SSL_VERIFICATION": []byte("true"),
		"ONTAP_SVM":                     []byte("test-svm"),
		"TRIDENT_BACKEND_UUID":          []byte("test-uuid"),
	}, nil, "")
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

func TestNewCsiImportPluginMissingStorageKeys(t *testing.T) {
	_, err := newCsiImportPlugin(forklift.StorageVendorProductOntap, map[string][]byte{}, nil, "")
	if err == nil {
		t.Fatal("expected error for missing storage keys, got nil")
	}
}
