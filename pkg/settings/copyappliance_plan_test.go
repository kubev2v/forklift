package settings

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
)

func TestCopyApplianceEnabledForPlan(t *testing.T) {
	Settings.Features.CopyAppliance = true
	Settings.ContainerImage = "copy-appliance:latest"
	Settings.VddkImage = ""

	vsphere, openshift := api.VSphere, api.OpenShift
	source := &api.Provider{Spec: api.ProviderSpec{Type: &vsphere}}
	p := &api.Plan{
		Spec: api.PlanSpec{
			Type:               api.MigrationCold,
			MigrateSharedDisks: true,
			VMs: []plan.VM{{
				Ref: ref.Ref{ID: "vm-1", Name: "vm-1"},
			}},
		},
		Referenced: api.Referenced{
			Provider: struct {
				Source, Destination *api.Provider
			}{
				Source:      source,
				Destination: &api.Provider{Spec: api.ProviderSpec{Type: &openshift, URL: "https://remote.example.com"}},
			},
		},
	}

	if !Settings.EnabledForPlan(p) {
		t.Fatal("expected copy appliance path for vSphere cold migration with copyApplianceTemplate enabled")
	}

	source.Spec.Settings = map[string]string{api.VDDK: "quay.io/example/vddk:latest"}
	if Settings.EnabledForPlan(p) {
		t.Fatal("expected VDDK to take priority over copyApplianceTemplate")
	}

	// Global VDDK_IMAGE still wins when the provider has no vddkInitImage setting.
	source.Spec.Settings = nil
	Settings.VddkImage = "quay.io/example/vddk-global:latest"
	if Settings.EnabledForPlan(p) {
		t.Fatal("expected global VDDK image to take priority over copyApplianceTemplate")
	}
	Settings.VddkImage = ""

	Settings.Features.CopyAppliance = false
	if Settings.EnabledForPlan(p) {
		t.Fatal("expected copy appliance disabled when copyApplianceTemplate feature is off")
	}
}
