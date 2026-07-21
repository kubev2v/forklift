package ocp

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

func newBuilder(plan *api.Plan) *Builder {
	return &Builder{
		Context: &plancontext.Context{
			Plan: plan,
		},
	}
}

func TestHasCustomPVCNameTemplate_NoTemplateSet(t *testing.T) {
	b := newBuilder(&api.Plan{})
	vmRef := ref.Ref{Name: "vm1", Namespace: "ns1"}

	if b.hasCustomPVCNameTemplate(vmRef) {
		t.Error("expected false when no template is set")
	}
}

func TestHasCustomPVCNameTemplate_PlanLevelTemplate(t *testing.T) {
	plan := &api.Plan{
		Spec: api.PlanSpec{
			PVCNameTemplate: "{{.TargetVmName}}-disk-{{.DiskIndex}}",
		},
	}
	b := newBuilder(plan)
	vmRef := ref.Ref{Name: "vm1", Namespace: "ns1"}

	if !b.hasCustomPVCNameTemplate(vmRef) {
		t.Error("expected true when plan-level template is set")
	}
}

func TestHasCustomPVCNameTemplate_VMLevelTemplate(t *testing.T) {
	plan := &api.Plan{
		Spec: api.PlanSpec{
			VMs: []planapi.VM{
				{
					Ref:             ref.Ref{Name: "vm1"},
					PVCNameTemplate: "{{.SourcePVCName}}-migrated",
				},
			},
		},
	}
	b := newBuilder(plan)
	vmRef := ref.Ref{Name: "vm1"}

	if !b.hasCustomPVCNameTemplate(vmRef) {
		t.Error("expected true when VM-level template is set")
	}
}

func TestHasCustomPVCNameTemplate_VMLevelOverrideOnly(t *testing.T) {
	plan := &api.Plan{
		Spec: api.PlanSpec{
			VMs: []planapi.VM{
				{
					Ref:             ref.Ref{Name: "vm1"},
					PVCNameTemplate: "custom-{{.DiskIndex}}",
				},
				{
					Ref: ref.Ref{Name: "vm2"},
				},
			},
		},
	}
	b := newBuilder(plan)

	if !b.hasCustomPVCNameTemplate(ref.Ref{Name: "vm1"}) {
		t.Error("expected true for vm1 with custom template")
	}
	if b.hasCustomPVCNameTemplate(ref.Ref{Name: "vm2"}) {
		t.Error("expected false for vm2 without custom template")
	}
}

func TestSetPVCNameFromTemplate_DefaultTemplate(t *testing.T) {
	plan := &api.Plan{}
	b := newBuilder(plan)

	vmRef := ref.Ref{Name: "vm1", Namespace: "ns1"}

	template := b.getPVCNameTemplate(vmRef)
	if template != "{{.SourcePVCName}}" {
		t.Errorf("expected default template '{{.SourcePVCName}}', got %q", template)
	}
}

func TestSetPVCNameFromTemplate_CustomPlanTemplate(t *testing.T) {
	plan := &api.Plan{
		Spec: api.PlanSpec{
			PVCNameTemplate: "migrated-{{.DiskIndex}}",
		},
	}
	b := newBuilder(plan)

	vmRef := ref.Ref{Name: "vm1", Namespace: "ns1"}

	template := b.getPVCNameTemplate(vmRef)
	if template != "migrated-{{.DiskIndex}}" {
		t.Errorf("expected plan-level template, got %q", template)
	}
}

func TestSetPVCNameFromTemplate_VMLevelOverridesPlan(t *testing.T) {
	plan := &api.Plan{
		Spec: api.PlanSpec{
			PVCNameTemplate: "plan-level-{{.DiskIndex}}",
			VMs: []planapi.VM{
				{
					Ref:             ref.Ref{Name: "vm1"},
					PVCNameTemplate: "vm-level-{{.DiskIndex}}",
				},
			},
		},
	}
	b := newBuilder(plan)

	template := b.getPVCNameTemplate(ref.Ref{Name: "vm1"})
	if template != "vm-level-{{.DiskIndex}}" {
		t.Errorf("expected VM-level template to override plan-level, got %q", template)
	}
}

func TestExecuteTemplate_OCPTemplateData(t *testing.T) {
	b := newBuilder(&api.Plan{})

	data := &api.OCPPVCNameTemplateData{
		VmName:             "source-vm",
		TargetVmName:       "target-vm",
		PlanName:           "my-plan",
		DiskIndex:          0,
		SourcePVCName:      "my-pvc",
		SourcePVCNamespace: "src-ns",
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "default template",
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := b.executeTemplate(tc.template, data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestApplyDestinationStorageModes_UsesMappingAccessModeWhenSet(t *testing.T) {
	spec := createDataVolumeSpec(resource.MustParse("10Gi"), "target-sc", "https://example", "cm", "secret")
	sourcePVC := &core.PersistentVolumeClaim{
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
		},
	}
	destination := api.DestinationStorage{
		StorageClass: "target-sc",
		AccessMode:   core.ReadWriteMany,
	}

	applyDestinationStorageModes(spec, destination, sourcePVC)

	if len(spec.Storage.AccessModes) != 1 || spec.Storage.AccessModes[0] != core.ReadWriteMany {
		t.Fatalf("expected mapping AccessMode RWX, got %v", spec.Storage.AccessModes)
	}
}

func TestApplyDestinationStorageModes_PreservesSourceAccessModeWhenMappingOmits(t *testing.T) {
	spec := createDataVolumeSpec(resource.MustParse("11Mi"), "ocs-storagecluster-ceph-rbd-virtualization", "https://example", "cm", "secret")
	sourcePVC := &core.PersistentVolumeClaim{
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
		},
	}
	destination := api.DestinationStorage{
		StorageClass: "ocs-storagecluster-ceph-rbd-virtualization",
	}

	applyDestinationStorageModes(spec, destination, sourcePVC)

	if len(spec.Storage.AccessModes) != 1 || spec.Storage.AccessModes[0] != core.ReadWriteOnce {
		t.Fatalf("expected source AccessMode RWO, got %v", spec.Storage.AccessModes)
	}
}

func TestApplyDestinationStorageModes_PreservesSourceVolumeModeWhenMappingOmits(t *testing.T) {
	blockMode := core.PersistentVolumeBlock
	spec := createDataVolumeSpec(resource.MustParse("10Gi"), "target-sc", "https://example", "cm", "secret")
	sourcePVC := &core.PersistentVolumeClaim{
		Spec: core.PersistentVolumeClaimSpec{
			VolumeMode: &blockMode,
		},
	}
	destination := api.DestinationStorage{StorageClass: "target-sc"}

	applyDestinationStorageModes(spec, destination, sourcePVC)

	if spec.Storage.VolumeMode == nil || *spec.Storage.VolumeMode != core.PersistentVolumeBlock {
		t.Fatalf("expected source VolumeMode Block, got %v", spec.Storage.VolumeMode)
	}
}

func TestApplyDestinationStorageModes_UsesMappingVolumeModeWhenSet(t *testing.T) {
	filesystemMode := core.PersistentVolumeFilesystem
	spec := createDataVolumeSpec(resource.MustParse("10Gi"), "target-sc", "https://example", "cm", "secret")
	sourcePVC := &core.PersistentVolumeClaim{
		Spec: core.PersistentVolumeClaimSpec{
			VolumeMode: &filesystemMode,
		},
	}
	destination := api.DestinationStorage{
		StorageClass: "target-sc",
		VolumeMode:   core.PersistentVolumeBlock,
	}

	applyDestinationStorageModes(spec, destination, sourcePVC)

	if spec.Storage.VolumeMode == nil || *spec.Storage.VolumeMode != core.PersistentVolumeBlock {
		t.Fatalf("expected mapping VolumeMode Block, got %v", spec.Storage.VolumeMode)
	}
}

func TestApplyDestinationStorageModes_NilSafe(t *testing.T) {
	applyDestinationStorageModes(nil, api.DestinationStorage{}, &core.PersistentVolumeClaim{})
	applyDestinationStorageModes(&cdi.DataVolumeSpec{}, api.DestinationStorage{}, &core.PersistentVolumeClaim{})
	applyDestinationStorageModes(createDataVolumeSpec(resource.MustParse("1Gi"), "sc", "https://example", "cm", "secret"), api.DestinationStorage{}, nil)
}
