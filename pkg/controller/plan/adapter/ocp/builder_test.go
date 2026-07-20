package ocp

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	"github.com/kubev2v/forklift/pkg/templateutil"
)

func TestGetPVCNameTemplate_UniversalDefault(t *testing.T) {
	plan := &api.Plan{}

	template := planbase.GetPVCNameTemplate(plan, "vm1-id")
	expected := planbase.DefaultPVCNameTemplate
	if template != expected {
		t.Errorf("expected universal default template %q, got %q", expected, template)
	}
}

func TestGetPVCNameTemplate_PlanLevelTemplate(t *testing.T) {
	plan := &api.Plan{
		Spec: api.PlanSpec{
			PVCNameTemplate: "migrated-{{.DiskIndex}}",
		},
	}

	template := planbase.GetPVCNameTemplate(plan, "vm1-id")
	if template != "migrated-{{.DiskIndex}}" {
		t.Errorf("expected plan-level template, got %q", template)
	}
}

func TestGetPVCNameTemplate_VMLevelOverridesPlan(t *testing.T) {
	plan := &api.Plan{
		Spec: api.PlanSpec{
			PVCNameTemplate: "plan-level-{{.DiskIndex}}",
			VMs: []planapi.VM{
				{
					Ref:             ref.Ref{ID: "vm1-id", Name: "vm1"},
					PVCNameTemplate: "vm-level-{{.DiskIndex}}",
				},
			},
		},
	}

	template := planbase.GetPVCNameTemplate(plan, "vm1-id")
	if template != "vm-level-{{.DiskIndex}}" {
		t.Errorf("expected VM-level template to override plan-level, got %q", template)
	}
}

func TestGetPVCNameTemplate_VMLevelOnlyForMatchingVM(t *testing.T) {
	plan := &api.Plan{
		Spec: api.PlanSpec{
			PVCNameTemplate: "plan-{{.DiskIndex}}",
			VMs: []planapi.VM{
				{
					Ref:             ref.Ref{ID: "vm1-id", Name: "vm1"},
					PVCNameTemplate: "custom-{{.DiskIndex}}",
				},
				{
					Ref: ref.Ref{ID: "vm2-id", Name: "vm2"},
				},
			},
		},
	}

	if tmpl := planbase.GetPVCNameTemplate(plan, "vm1-id"); tmpl != "custom-{{.DiskIndex}}" {
		t.Errorf("expected VM-level template for vm1, got %q", tmpl)
	}
	if tmpl := planbase.GetPVCNameTemplate(plan, "vm2-id"); tmpl != "plan-{{.DiskIndex}}" {
		t.Errorf("expected plan-level template for vm2, got %q", tmpl)
	}
}

func TestExecuteTemplate_OCPTemplateData(t *testing.T) {
	data := &api.PVCNameTemplateData{
		VmName:             "source-vm",
		TargetVmName:       "target-vm",
		PlanName:           "my-plan",
		DiskIndex:          0,
		VmId:               "vm-12345",
		SourcePVCName:      "my-pvc",
		SourcePVCNamespace: "src-ns",
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "source pvc name template",
			template: "{{.SourcePVCName}}",
			expected: "my-pvc",
		},
		{
			name:     "target vm with disk index",
			template: "{{.TargetVmName}}-disk-{{.DiskIndex}}",
			expected: "target-vm-disk-0",
		},
		{
			name:     "plan name prefix",
			template: "{{.PlanName}}-{{.SourcePVCName}}",
			expected: "my-plan-my-pvc",
		},
		{
			name:     "VmId variable",
			template: "{{.PlanName}}-{{.VmId}}",
			expected: "my-plan-vm-12345",
		},
		{
			name:     "universal default template",
			template: "{{trunc 15 .PlanName}}-{{trunc 15 .TargetVmName}}-disk-{{.DiskIndex}}",
			expected: "my-plan-target-vm-disk-0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := templateutil.ExecuteTemplate(tc.template, data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}
