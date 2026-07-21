package nutanix

import (
	"testing"

	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

func TestBase_Pk(t *testing.T) {
	b := &Base{ID: "vm-1"}
	if b.Pk() != "vm-1" {
		t.Errorf("expected Pk() to return %q, got %q", "vm-1", b.Pk())
	}
}

func TestBase_String(t *testing.T) {
	b := &Base{ID: "vm-1"}
	if b.String() != "vm-1" {
		t.Errorf("expected String() to return %q, got %q", "vm-1", b.String())
	}
}

func TestBase_Labels(t *testing.T) {
	b := &Base{ID: "vm-1"}
	if b.Labels() != nil {
		t.Errorf("expected Labels() to be nil, got %v", b.Labels())
	}
}

func TestBase_WithRef(t *testing.T) {
	b := &Base{}
	b.WithRef(Ref{ID: "vm-2"})
	if b.ID != "vm-2" {
		t.Errorf("expected WithRef() to set ID to %q, got %q", "vm-2", b.ID)
	}
}

func TestBase_GetName(t *testing.T) {
	b := &Base{Name: "my-vm"}
	if b.GetName() != "my-vm" {
		t.Errorf("expected GetName() to return %q, got %q", "my-vm", b.GetName())
	}
}

// TestBase_Equals documents the actual (narrow) behavior of Equals: it only
// matches when compared against another *Base with the same ID -- not
// against another model that merely embeds Base (e.g. *VM), since the type
// assertion in Equals() is against *Base specifically.
func TestBase_Equals(t *testing.T) {
	a := &Base{ID: "vm-1"}
	same := &Base{ID: "vm-1"}
	different := &Base{ID: "vm-2"}

	if !a.Equals(same) {
		t.Error("expected two *Base with the same ID to be equal")
	}
	if a.Equals(different) {
		t.Error("expected two *Base with different IDs to not be equal")
	}

	vm := &VM{Base: Base{ID: "vm-1"}}
	var asModel libmodel.Model = vm
	if a.Equals(asModel) {
		t.Error("expected Equals() to return false when compared against a type that merely embeds Base")
	}
}
