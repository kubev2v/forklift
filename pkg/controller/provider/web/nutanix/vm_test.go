package nutanix

import (
	"testing"

	modelbase "github.com/kubev2v/forklift/pkg/controller/provider/model/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
)

// Compile-time assertion that the VM resource satisfies ConcernHolder, so
// plan validation's concern aggregation (aggregateCriticalConcerns /
// aggregateWarningConcerns) recognizes Nutanix VMs instead of silently
// skipping them.
var _ modelbase.ConcernHolder = &VM{}

// TestVM_PolicyFieldsRoundTrip verifies that RevisionValidated, PolicyVersion,
// and Concerns are copied from the model onto the web resource, and that
// GetConcerns() exposes them. No policy-agent submission loop exists yet for
// Nutanix, so these are always zero/empty in practice today -- this test
// only confirms the plumbing works once something does populate them.
func TestVM_PolicyFieldsRoundTrip(t *testing.T) {
	m := &model.VM{
		RevisionValidated: 5,
		PolicyVersion:     2,
		Concerns: []model.Concern{
			{Id: "test.concern", Label: "Test concern", Category: "Critical"},
		},
	}

	r := &VM{}
	r.With(m)

	if r.RevisionValidated != m.RevisionValidated {
		t.Errorf("expected RevisionValidated %d, got %d", m.RevisionValidated, r.RevisionValidated)
	}
	if r.PolicyVersion != m.PolicyVersion {
		t.Errorf("expected PolicyVersion %d, got %d", m.PolicyVersion, r.PolicyVersion)
	}
	if len(r.GetConcerns()) != 1 || r.GetConcerns()[0].Id != "test.concern" {
		t.Errorf("expected GetConcerns() to expose the model's concerns, got %+v", r.GetConcerns())
	}
}

// TestVM_PolicyFieldsRespectDetailLevel verifies RevisionValidated/Concerns
// (VM1, detail>=1) and PolicyVersion (VM, detail>=2) are hidden below their
// respective detail levels, matching every other provider's convention.
func TestVM_PolicyFieldsRespectDetailLevel(t *testing.T) {
	m := &model.VM{
		RevisionValidated: 5,
		PolicyVersion:     2,
		Concerns:          []model.Concern{{Id: "test.concern"}},
	}
	r := &VM{}
	r.With(m)

	if _, ok := r.Content(0).(*VM0); !ok {
		t.Fatalf("expected Content(0) to be *VM0, got %T", r.Content(0))
	}
	vm1, ok := r.Content(1).(*VM1)
	if !ok {
		t.Fatalf("expected Content(1) to be *VM1, got %T", r.Content(1))
	}
	if vm1.RevisionValidated != m.RevisionValidated {
		t.Errorf("expected Content(1) to include RevisionValidated %d, got %d", m.RevisionValidated, vm1.RevisionValidated)
	}
	if len(vm1.Concerns) != 1 {
		t.Errorf("expected Content(1) to include Concerns, got %+v", vm1.Concerns)
	}
	full, ok := r.Content(2).(*VM)
	if !ok {
		t.Fatalf("expected Content(2) to be *VM, got %T", r.Content(2))
	}
	if full.PolicyVersion != m.PolicyVersion {
		t.Errorf("expected Content(2) to include PolicyVersion %d, got %d", m.PolicyVersion, full.PolicyVersion)
	}
}
