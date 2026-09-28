package provider

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func hypervProvider(settings map[string]string) *api.Provider {
	pt := api.HyperV
	return &api.Provider{
		ObjectMeta: v1.ObjectMeta{Name: "test"},
		Spec:       api.ProviderSpec{Type: &pt, Settings: settings},
	}
}

func TestValidateHyperVSettings_ValidCluster(t *testing.T) {
	p := hypervProvider(map[string]string{api.ManagementType: api.HyperVCluster})
	r := Reconciler{}
	if err := r.ValidateHyperVSettings(p); err != nil {
		t.Fatal(err)
	}
	if p.Status.HasCondition(SettingsNotValid) {
		t.Error("Expected no SettingsNotValid condition for cluster mode")
	}
}

func TestValidateHyperVSettings_ValidStandalone(t *testing.T) {
	p := hypervProvider(map[string]string{api.ManagementType: api.HyperVStandalone})
	r := Reconciler{}
	if err := r.ValidateHyperVSettings(p); err != nil {
		t.Fatal(err)
	}
	if p.Status.HasCondition(SettingsNotValid) {
		t.Error("Expected no SettingsNotValid condition for standalone mode")
	}
}

func TestValidateHyperVSettings_EmptyDefaultsToValid(t *testing.T) {
	p := hypervProvider(map[string]string{})
	r := Reconciler{}
	if err := r.ValidateHyperVSettings(p); err != nil {
		t.Fatal(err)
	}
	if p.Status.HasCondition(SettingsNotValid) {
		t.Error("Expected no SettingsNotValid condition when managementType is empty")
	}
}

func TestValidateHyperVSettings_InvalidType(t *testing.T) {
	p := hypervProvider(map[string]string{api.ManagementType: "bogus"})
	r := Reconciler{}
	if err := r.ValidateHyperVSettings(p); err != nil {
		t.Fatal(err)
	}
	if !p.Status.HasCondition(SettingsNotValid) {
		t.Error("Expected SettingsNotValid condition for invalid managementType")
	}
	if p.Status.Phase != ValidationFailed {
		t.Errorf("Expected phase '%s', got '%s'", ValidationFailed, p.Status.Phase)
	}
}

func TestValidateHyperVSettings_NonHyperVSkipped(t *testing.T) {
	pt := api.VSphere
	p := &api.Provider{
		ObjectMeta: v1.ObjectMeta{Name: "test"},
		Spec:       api.ProviderSpec{Type: &pt, Settings: map[string]string{api.ManagementType: "bogus"}},
	}
	r := Reconciler{}
	if err := r.ValidateHyperVSettings(p); err != nil {
		t.Fatal(err)
	}
	if p.Status.HasCondition(SettingsNotValid) {
		t.Error("Expected validation to be skipped for non-HyperV provider")
	}
}
