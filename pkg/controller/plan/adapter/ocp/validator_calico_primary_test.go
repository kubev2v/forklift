package ocp

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
)

func newValidatorWithMap(pairs []api.NetworkPair) *Validator {
	plan := &api.Plan{}
	plan.Referenced.Map.Network = &api.NetworkMap{Spec: api.NetworkMapSpec{Map: pairs}}
	return &Validator{Context: &plancontext.Context{Plan: plan}}
}

func TestValidateCalicoPrimary_RejectsCalicoEntry(t *testing.T) {
	v := newValidatorWithMap([]api.NetworkPair{{
		Destination: api.DestinationNetwork{Type: planbase.Pod, Calico: &api.CalicoDestination{}},
	}})
	result, err := v.ValidateCalicoPrimary(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Issues) != 1 || result.Issues[0].Kind != planbase.CalicoIssuePrimaryProviderUnsupported {
		t.Errorf("expected one PrimaryProviderUnsupported issue, got %+v", result.Issues)
	}
}

func TestValidateCalicoPrimary_AllowsNonCalicoMap(t *testing.T) {
	v := newValidatorWithMap([]api.NetworkPair{{
		Destination: api.DestinationNetwork{Type: planbase.Pod},
	}})
	result, err := v.ValidateCalicoPrimary(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Issues) != 0 {
		t.Errorf("expected no issues, got %+v", result.Issues)
	}
}
