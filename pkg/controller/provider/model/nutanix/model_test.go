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

// TestBase_Equals verifies that Equals compares by the promoted Pk(), so it
// correctly matches concrete models (e.g. *VM) that embed Base and share the
// same primary key, not just other *Base values directly.
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
	if a.Equals(nil) {
		t.Error("expected Equals() to return false when compared against nil")
	}

	vm := &VM{Base: Base{ID: "vm-1"}}
	var asModel libmodel.Model = vm
	if !a.Equals(asModel) {
		t.Error("expected Equals() to return true when Pk() matches, even for a concrete model that merely embeds Base")
	}

	otherVM := &VM{Base: Base{ID: "vm-2"}}
	if a.Equals(otherVM) {
		t.Error("expected Equals() to return false when Pk() differs, even for a concrete model")
	}
}

// TestAll verifies that All() registers exactly the six expected concrete
// Nutanix model types. All() controls which inventory tables get created and
// persisted, so an accidental omission here would only otherwise surface
// later, during inventory persistence.
func TestAll(t *testing.T) {
	models := All()
	if len(models) != 6 {
		t.Fatalf("expected All() to return 6 models, got %d", len(models))
	}

	seen := make(map[string]bool)
	for _, m := range models {
		switch m.(type) {
		case *Cluster:
			seen["Cluster"] = true
		case *Host:
			seen["Host"] = true
		case *Network:
			seen["Network"] = true
		case *StorageContainer:
			seen["StorageContainer"] = true
		case *VM:
			seen["VM"] = true
		case *Image:
			seen["Image"] = true
		default:
			t.Errorf("unexpected model type registered in All(): %T", m)
		}
	}

	for _, name := range []string{"Cluster", "Host", "Network", "StorageContainer", "VM", "Image"} {
		if !seen[name] {
			t.Errorf("expected All() to register %s, but it was missing", name)
		}
	}
}
