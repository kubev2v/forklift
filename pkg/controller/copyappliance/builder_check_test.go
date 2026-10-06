package copyappliance

import (
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

func TestBuildCheck(t *testing.T) {
	withSettings(t, testSettings())
	provider := testProvider()
	builder := &Builder{Provider: provider, Inventory: testInventory()}

	appliance, err := builder.Check(testToehold())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	// There is no source VM to take disks from, and the template's own root
	// vmdk is not something a check appliance serves.
	if len(appliance.Spec.AttachDisks) != 0 {
		t.Errorf("AttachDisks = %+v, want none", appliance.Spec.AttachDisks)
	}
	if appliance.Spec.Template != "/DC0/vm/templates/vcenter-toehold" {
		t.Errorf("Template = %q, want the toehold template's inventory path", appliance.Spec.Template)
	}
	// The same placement a migration appliance gets, which is the point: the
	// check proves that placement works.
	if appliance.Spec.Folder != "/DC0/vm/templates" || appliance.Spec.Datastore != "templates" {
		t.Errorf("placement = (%q, %q), want the template's folder and datastore",
			appliance.Spec.Folder, appliance.Spec.Datastore)
	}
	if appliance.Spec.Secret.Name != "toehold-ssh-keys-vcenter-private" {
		t.Errorf("Secret = %v, want the toehold private secret", appliance.Spec.Secret)
	}

	// Generated, and found again by its labels rather than by its name.
	if appliance.Name != "" {
		t.Errorf("Name = %q, want it left for the API server", appliance.Name)
	}
	if appliance.GenerateName != checkPrefix {
		t.Errorf("GenerateName = %q, want %q", appliance.GenerateName, checkPrefix)
	}
	if _, found := appliance.Labels[api.LabelVM]; found {
		t.Errorf("label %q is set, but a check appliance has no source VM", api.LabelVM)
	}
	if appliance.Labels[api.LabelSubapp] != api.SubappCheck {
		t.Errorf("label %q = %q, want %q",
			api.LabelSubapp, appliance.Labels[api.LabelSubapp], api.SubappCheck)
	}
	if appliance.Labels[api.LabelProvider] != string(provider.UID) {
		t.Errorf("label %q = %q, want the provider's UID",
			api.LabelProvider, appliance.Labels[api.LabelProvider])
	}
}

// The CopyAppliance's name is what its VM is cloned as, and vCenter rejects a
// VM name over 80 characters. The API server appends its own suffix to the
// prefix the builder sets, so the prefix has to leave room for it.
func TestCheckPrefixFitsAVMName(t *testing.T) {
	if len(checkPrefix)+generatedNameSuffixLength > maxVMNameLength {
		t.Errorf("%q generates a %d character name, want at most %d",
			checkPrefix, len(checkPrefix)+generatedNameSuffixLength, maxVMNameLength)
	}
}

// Placement has nothing to work from without the template's moref, and a
// blank one would send the finder looking for "the default" object.
func TestBuildCheckRejectsAnUnimportedTemplate(t *testing.T) {
	withSettings(t, testSettings())
	toehold := testToehold()
	toehold.Status.Template.Moref = ""
	builder := &Builder{Provider: testProvider(), Inventory: testInventory()}

	_, err := builder.Check(toehold)
	if err == nil {
		t.Fatal("Check succeeded without a template moref")
	}
	if !strings.Contains(err.Error(), "moref") {
		t.Errorf("error = %v, want it to name the missing moref", err)
	}
}
