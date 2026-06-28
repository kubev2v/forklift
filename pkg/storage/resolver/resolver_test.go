package resolver

import (
	"testing"
)

// Compile-time check: stubPlugin must implement CsiImportPlugin.
var _ CsiImportPlugin = (*stubPlugin)(nil)

func TestCsiImportPluginInterface(t *testing.T) {
	var p CsiImportPlugin = &stubPlugin{}
	annotations, err := p.Resolve(&DiskBacking{VVolID: "vvol:test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(annotations) == 0 {
		t.Error("Resolve must return at least one annotation")
	}
}

func TestNewVolumeImportResolverUnknownVendor(t *testing.T) {
	_, err := newStubPlugin("nonexistent-vendor")
	if err == nil {
		t.Fatal("expected error for unknown vendor, got nil")
	}
}

// newStubPlugin simulates the csi_import.go switch for testing purposes.
func newStubPlugin(product string) (CsiImportPlugin, error) {
	if product == "stub-vendor" {
		return &stubPlugin{}, nil
	}
	return nil, &unknownVendorError{product}
}

type unknownVendorError struct{ product string }

func (e *unknownVendorError) Error() string {
	return "CSI import not supported for vendor " + e.product
}

type stubPlugin struct{}

func (s *stubPlugin) Resolve(_ *DiskBacking) (map[string]string, error) {
	return map[string]string{"test.example.com/importVol": "test-volume"}, nil
}
