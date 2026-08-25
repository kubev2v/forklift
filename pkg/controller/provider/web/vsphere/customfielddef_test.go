package vsphere

import (
	"testing"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
)

func TestCustomFieldDefWith(t *testing.T) {
	m := &model.CustomFieldDef{
		Name:              "app-name",
		Key:               100,
		ManagedObjectType: "VirtualMachine",
	}
	var r CustomFieldDef
	r.With(m)

	if r.ID != "100" {
		t.Errorf("ID = %q, want %q", r.ID, "100")
	}
	if r.Key != 100 {
		t.Errorf("Key = %d, want 100", r.Key)
	}
	if r.Name != "app-name" {
		t.Errorf("Name = %q, want %q", r.Name, "app-name")
	}
	if r.ManagedObjectType != "VirtualMachine" {
		t.Errorf("ManagedObjectType = %q, want %q", r.ManagedObjectType, "VirtualMachine")
	}
}

func TestCustomFieldDefRouteConstants(t *testing.T) {
	if CustomFieldDefsRoot != ProviderRoot+"/"+CustomFieldDefCollection {
		t.Errorf("CustomFieldDefsRoot = %q, want %q", CustomFieldDefsRoot, ProviderRoot+"/"+CustomFieldDefCollection)
	}
	if CustomFieldDefRoot != CustomFieldDefsRoot+"/:"+CustomFieldDefParam {
		t.Errorf("CustomFieldDefRoot = %q, want %q", CustomFieldDefRoot, CustomFieldDefsRoot+"/:"+CustomFieldDefParam)
	}
}
