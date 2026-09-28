package v1beta1

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
)

func TestShouldUseV2vForTransfer_ExcludeDisks(t *testing.T) {
	vsphere, openshift := VSphere, OpenShift
	newPlan := func(exclude []string) *Plan {
		return &Plan{
			Spec: PlanSpec{
				MigrateSharedDisks: true,
				VMs: []plan.VM{{
					Ref:          ref.Ref{ID: "vm-1"},
					ExcludeDisks: exclude,
				}},
			},
			Referenced: Referenced{
				Provider: struct {
					Source, Destination *Provider
				}{
					Source:      &Provider{Spec: ProviderSpec{Type: &vsphere, URL: "https://vc"}},
					Destination: &Provider{Spec: ProviderSpec{Type: &openshift}},
				},
			},
		}
	}

	ok, err := newPlan([]string{"scsi0:1"}).ShouldUseV2vForTransfer(ref.Ref{ID: "vm-1"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected CDI path when excludeDisks is set")
	}

	ok, err = newPlan(nil).ShouldUseV2vForTransfer(ref.Ref{ID: "vm-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected virt-v2v when excludeDisks is empty")
	}
}
