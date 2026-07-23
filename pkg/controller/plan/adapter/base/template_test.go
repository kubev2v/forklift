package base

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetPVCNameOnObject_GenerateName(t *testing.T) {
	objectMeta := &metav1.ObjectMeta{}
	templateData := &api.PVCNameTemplateData{
		VmName:       "my-vm",
		TargetVmName: "my-vm",
		PlanName:     "test-plan",
		DiskIndex:    0,
		VmId:         "vm-123",
	}

	err := SetPVCNameOnObject(objectMeta, DefaultPVCNameTemplate, true, templateData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objectMeta.Name != "" {
		t.Errorf("expected Name to be empty, got %q", objectMeta.Name)
	}
	if objectMeta.GenerateName == "" {
		t.Fatal("expected GenerateName to be set")
	}
	if objectMeta.GenerateName != "test-plan-my-vm-disk-0-" {
		t.Errorf("expected GenerateName = %q, got %q", "test-plan-my-vm-disk-0-", objectMeta.GenerateName)
	}
}

func TestSetPVCNameOnObject_ExactName(t *testing.T) {
	objectMeta := &metav1.ObjectMeta{}
	templateData := &api.PVCNameTemplateData{
		VmName:       "my-vm",
		TargetVmName: "my-vm",
		PlanName:     "test-plan",
		DiskIndex:    1,
		VmId:         "vm-123",
	}

	err := SetPVCNameOnObject(objectMeta, DefaultPVCNameTemplate, false, templateData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objectMeta.GenerateName != "" {
		t.Errorf("expected GenerateName to be empty, got %q", objectMeta.GenerateName)
	}
	if objectMeta.Name != "test-plan-my-vm-disk-1" {
		t.Errorf("expected Name = %q, got %q", "test-plan-my-vm-disk-1", objectMeta.Name)
	}
}

func TestSetPVCNameOnObject_Truncation(t *testing.T) {
	objectMeta := &metav1.ObjectMeta{}
	templateData := &api.PVCNameTemplateData{
		VmName:       "a-very-long-vm-name-that-exceeds-fifteen",
		TargetVmName: "a-very-long-vm-name-that-exceeds-fifteen",
		PlanName:     "a-very-long-plan-name-that-exceeds",
		DiskIndex:    0,
		VmId:         "vm-456",
	}

	err := SetPVCNameOnObject(objectMeta, DefaultPVCNameTemplate, false, templateData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// trunc 15 should limit each to 15 chars
	expected := "a-very-long-pla-a-very-long-vm--disk-0"
	if objectMeta.Name != expected {
		t.Errorf("expected Name = %q, got %q", expected, objectMeta.Name)
	}
	if len(objectMeta.Name) > 63 {
		t.Errorf("name exceeds DNS1123 limit: len=%d", len(objectMeta.Name))
	}
}

func TestSetPVCNameOnObject_CustomTemplate(t *testing.T) {
	objectMeta := &metav1.ObjectMeta{}
	templateData := &api.PVCNameTemplateData{
		VmName:       "web-server",
		TargetVmName: "web-server",
		PlanName:     "prod-plan",
		DiskIndex:    2,
		VmId:         "vm-789",
	}

	customTemplate := "{{.PlanName}}-{{.VmId}}-{{.DiskIndex}}"
	err := SetPVCNameOnObject(objectMeta, customTemplate, false, templateData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objectMeta.Name != "prod-plan-vm-789-2" {
		t.Errorf("expected Name = %q, got %q", "prod-plan-vm-789-2", objectMeta.Name)
	}
}

func TestSetPVCNameOnObject_InvalidTemplate(t *testing.T) {
	objectMeta := &metav1.ObjectMeta{}
	templateData := &api.PVCNameTemplateData{
		VmName:       "my-vm",
		TargetVmName: "my-vm",
		PlanName:     "plan",
		DiskIndex:    0,
		VmId:         "vm-1",
	}

	// Template that would produce invalid k8s label (uppercase)
	err := SetPVCNameOnObject(objectMeta, "{{.VmName}}_INVALID", false, templateData)
	if err == nil {
		t.Fatal("expected error for invalid template output, got nil")
	}
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
