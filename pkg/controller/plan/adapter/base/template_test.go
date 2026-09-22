package base

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

func TestResolveTargetVmName(t *testing.T) {
	const vmID = "358ce831-f790-4ad9-8ee0-ee272b0ecac4"

	t.Run("nil plan returns vmName as-is when valid", func(t *testing.T) {
		got := ResolveTargetVmName(nil, vmID, "valid-name")
		if got != "valid-name" {
			t.Fatalf("got %q, want valid-name", got)
		}
	})

	t.Run("sanitizes invalid DNS1123 name", func(t *testing.T) {
		p := &api.Plan{}
		got := ResolveTargetVmName(p, vmID, "rhel9-node_2")
		if got != "rhel9-node-2" {
			t.Fatalf("got %q, want rhel9-node-2", got)
		}
	})

	t.Run("spec targetName wins over sanitization and is trimmed", func(t *testing.T) {
		p := &api.Plan{
			Spec: api.PlanSpec{
				VMs: []planapi.VM{{
					Ref:        ref.Ref{ID: vmID},
					TargetName: "  custom-target  ",
				}},
			},
		}
		got := ResolveTargetVmName(p, vmID, "rhel9-node_2")
		if got != "custom-target" {
			t.Fatalf("got %q, want %q (leading/trailing spaces must be trimmed)", got, "custom-target")
		}
	})

	t.Run("status newName wins over sanitization", func(t *testing.T) {
		p := &api.Plan{}
		p.Status.Migration.VMs = []*planapi.VMStatus{
			{VM: planapi.VM{Ref: ref.Ref{ID: vmID}}, NewName: "migrated-name"},
		}
		got := ResolveTargetVmName(p, vmID, "rhel9-node_2")
		if got != "migrated-name" {
			t.Fatalf("got %q, want migrated-name", got)
		}
	})

	t.Run("spec targetName wins over status newName", func(t *testing.T) {
		p := &api.Plan{
			Spec: api.PlanSpec{
				VMs: []planapi.VM{{
					Ref:        ref.Ref{ID: vmID},
					TargetName: "explicit-target",
				}},
			},
		}
		p.Status.Migration.VMs = []*planapi.VMStatus{
			{VM: planapi.VM{Ref: ref.Ref{ID: vmID}}, NewName: "migrated-name"},
		}
		got := ResolveTargetVmName(p, vmID, "rhel9-node_2")
		if got != "explicit-target" {
			t.Fatalf("got %q, want explicit-target", got)
		}
	})

	t.Run("valid name returned as-is", func(t *testing.T) {
		p := &api.Plan{}
		got := ResolveTargetVmName(p, vmID, "already-valid")
		if got != "already-valid" {
			t.Fatalf("got %q, want already-valid", got)
		}
	})

	t.Run("whitespace-only targetName falls through to sanitization", func(t *testing.T) {
		p := &api.Plan{
			Spec: api.PlanSpec{
				VMs: []planapi.VM{{
					Ref:        ref.Ref{ID: vmID},
					TargetName: "   ",
				}},
			},
		}
		got := ResolveTargetVmName(p, vmID, "rhel9-node_2")
		if got != "rhel9-node-2" {
			t.Fatalf("got %q, want rhel9-node-2", got)
		}
	})

	t.Run("mismatched vmID falls through to sanitization", func(t *testing.T) {
		p := &api.Plan{
			Spec: api.PlanSpec{
				VMs: []planapi.VM{{
					Ref:        ref.Ref{ID: "other-id"},
					TargetName: "should-not-match",
				}},
			},
		}
		got := ResolveTargetVmName(p, vmID, "My_VM+Name")
		if got != "my-vm-name" {
			t.Fatalf("got %q, want my-vm-name", got)
		}
	})

	t.Run("all-invalid name produces deterministic fallback from vmID", func(t *testing.T) {
		p := &api.Plan{}
		otherID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

		got1 := ResolveTargetVmName(p, vmID, "...")
		got2 := ResolveTargetVmName(p, vmID, "...")
		gotOther := ResolveTargetVmName(p, otherID, "...")

		if got1 != got2 {
			t.Fatalf("non-deterministic: first call %q, second call %q", got1, got2)
		}
		if got1 == "" || got1 == "..." {
			t.Fatalf("expected a valid fallback name, got %q", got1)
		}
		if got1 == gotOther {
			t.Fatalf("different vmIDs should produce different fallbacks: %q == %q", got1, gotOther)
		}
		if errs := k8svalidation.IsDNS1123Label(got1); len(errs) > 0 {
			t.Fatalf("fallback %q is not a valid DNS1123 label: %v", got1, errs)
		}
	})
}
