package base

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
