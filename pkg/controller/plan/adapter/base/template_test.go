package base

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
)

func TestSetPVCNameOnObject(t *testing.T) {
	tests := []struct {
		name             string
		templateData     *api.PVCNameTemplateData
		template         string
		useGenerateName  bool
		wantName         string
		wantGenerateName string
		wantErr          bool
	}{
		{
			name: "generate-name",
			templateData: &api.PVCNameTemplateData{
				VmName:       "my-vm",
				TargetVmName: "my-vm",
				PlanName:     "test-plan",
				DiskIndex:    0,
				VmId:         "vm-123",
			},
			template:         DefaultPVCNameTemplate,
			useGenerateName:  true,
			wantGenerateName: "test-plan-my-vm-disk-0-",
		},
		{
			name: "exact-name",
			templateData: &api.PVCNameTemplateData{
				VmName:       "my-vm",
				TargetVmName: "my-vm",
				PlanName:     "test-plan",
				DiskIndex:    1,
				VmId:         "vm-123",
			},
			template: DefaultPVCNameTemplate,
			wantName: "test-plan-my-vm-disk-1",
		},
		{
			name: "truncation",
			templateData: &api.PVCNameTemplateData{
				VmName:       "a-very-long-vm-name-that-exceeds-fifteen",
				TargetVmName: "a-very-long-vm-name-that-exceeds-fifteen",
				PlanName:     "a-very-long-plan-name-that-exceeds",
				DiskIndex:    0,
				VmId:         "vm-456",
			},
			template: DefaultPVCNameTemplate,
			wantName: "a-very-long-pla-a-very-long-vm--disk-0",
		},
		{
			name: "custom-template",
			templateData: &api.PVCNameTemplateData{
				VmName:       "web-server",
				TargetVmName: "web-server",
				PlanName:     "prod-plan",
				DiskIndex:    2,
				VmId:         "vm-789",
			},
			template: "{{.PlanName}}-{{.VmId}}-{{.DiskIndex}}",
			wantName: "prod-plan-vm-789-2",
		},
		{
			name: "invalid-template",
			templateData: &api.PVCNameTemplateData{
				VmName:       "my-vm",
				TargetVmName: "my-vm",
				PlanName:     "plan",
				DiskIndex:    0,
				VmId:         "vm-1",
			},
			template: "{{.VmName}}_INVALID",
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			objectMeta := &metav1.ObjectMeta{}
			err := SetPVCNameOnObject(objectMeta, tc.template, tc.useGenerateName, tc.templateData)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error for invalid template output, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.useGenerateName {
				if objectMeta.Name != "" {
					t.Errorf("expected Name to be empty, got %q", objectMeta.Name)
				}
				if objectMeta.GenerateName != tc.wantGenerateName {
					t.Errorf("expected GenerateName = %q, got %q", tc.wantGenerateName, objectMeta.GenerateName)
				}
				return
			}
			if objectMeta.GenerateName != "" {
				t.Errorf("expected GenerateName to be empty, got %q", objectMeta.GenerateName)
			}
			if objectMeta.Name != tc.wantName {
				t.Errorf("expected Name = %q, got %q", tc.wantName, objectMeta.Name)
			}
			if tc.name == "truncation" && len(objectMeta.Name) > 63 {
				t.Errorf("name exceeds DNS1123 limit: len=%d", len(objectMeta.Name))
			}
		})
	}
}

func TestGetPVCNameTemplate_NilPlan(t *testing.T) {
	template := GetPVCNameTemplate(nil, "vm-1")
	if template != DefaultPVCNameTemplate {
		t.Errorf("expected default template for nil plan, got %q", template)
	}
}

// newOCPPlan returns a Plan whose source provider is OpenShift.
func newOCPPlan() *api.Plan {
	ocpType := api.OpenShift
	p := &api.Plan{}
	p.Provider.Source = &api.Provider{
		Spec: api.ProviderSpec{Type: &ocpType},
	}
	return p
}

func TestGetPVCNameTemplate_SettingsAndOCP(t *testing.T) {
	prevPVC := settings.Settings.PVCNameTemplate
	prevOCP := settings.Settings.OCPPVCNameTemplate
	t.Cleanup(func() {
		settings.Settings.PVCNameTemplate = prevPVC
		settings.Settings.OCPPVCNameTemplate = prevOCP
	})

	t.Run("non-OCP uses global setting", func(t *testing.T) {
		settings.Settings.PVCNameTemplate = "global-{{.DiskIndex}}"
		settings.Settings.OCPPVCNameTemplate = "ocp-{{.DiskIndex}}"
		got := GetPVCNameTemplate(&api.Plan{}, "vm-1")
		if got != "global-{{.DiskIndex}}" {
			t.Errorf("got %q, want global non-OCP template", got)
		}
	})

	t.Run("OCP uses OCP global setting", func(t *testing.T) {
		settings.Settings.PVCNameTemplate = "global-{{.DiskIndex}}"
		settings.Settings.OCPPVCNameTemplate = "ocp-{{.DiskIndex}}"
		got := GetPVCNameTemplate(newOCPPlan(), "vm-1")
		if got != "ocp-{{.DiskIndex}}" {
			t.Errorf("got %q, want OCP global template", got)
		}
	})

	t.Run("OCP default when no OCP global", func(t *testing.T) {
		settings.Settings.PVCNameTemplate = "global-{{.DiskIndex}}"
		settings.Settings.OCPPVCNameTemplate = ""
		got := GetPVCNameTemplate(newOCPPlan(), "vm-1")
		if got != DefaultOCPPVCNameTemplate {
			t.Errorf("got %q, want OCP default %q", got, DefaultOCPPVCNameTemplate)
		}
	})

	t.Run("plan beats globals", func(t *testing.T) {
		settings.Settings.PVCNameTemplate = "global-{{.DiskIndex}}"
		settings.Settings.OCPPVCNameTemplate = "ocp-{{.DiskIndex}}"
		plan := newOCPPlan()
		plan.Spec.PVCNameTemplate = "plan-{{.DiskIndex}}"
		got := GetPVCNameTemplate(plan, "vm-1")
		if got != "plan-{{.DiskIndex}}" {
			t.Errorf("got %q, want plan template", got)
		}
	})

	t.Run("OCP source auto-detected", func(t *testing.T) {
		settings.Settings.PVCNameTemplate = "global-{{.DiskIndex}}"
		settings.Settings.OCPPVCNameTemplate = ""
		got := GetPVCNameTemplate(newOCPPlan(), "vm-1")
		if got != DefaultOCPPVCNameTemplate {
			t.Errorf("got %q, want OCP default %q", got, DefaultOCPPVCNameTemplate)
		}
	})

	t.Run("UseGenerateName defaults by provider", func(t *testing.T) {
		if GetPVCNameTemplateUseGenerateName(newOCPPlan()) {
			t.Error("OCP unset default must be false")
		}
		if !GetPVCNameTemplateUseGenerateName(&api.Plan{}) {
			t.Error("non-OCP unset default must be true")
		}
		planTrue := newOCPPlan()
		planTrue.Spec.PVCNameTemplateUseGenerateName = ptr.To(true)
		if !GetPVCNameTemplateUseGenerateName(planTrue) {
			t.Error("explicit true must win for OCP")
		}
		planFalse := &api.Plan{Spec: api.PlanSpec{PVCNameTemplateUseGenerateName: ptr.To(false)}}
		if GetPVCNameTemplateUseGenerateName(planFalse) {
			t.Error("explicit false must win for non-OCP")
		}
	})
}

func TestValidatePVCNameTemplate_GenericData(t *testing.T) {
	testData := &api.PVCNameTemplateData{
		VmName:       "test-vm",
		TargetVmName: "test-vm",
		PlanName:     "my-plan",
		DiskIndex:    0,
		VmId:         "vm-001",
	}

	result, err := ValidatePVCNameTemplate(DefaultPVCNameTemplate, testData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "my-plan-test-vm-disk-0" {
		t.Errorf("expected %q, got %q", "my-plan-test-vm-disk-0", result)
	}
}

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
