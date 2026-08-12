package ocp

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	"github.com/kubev2v/forklift/pkg/templateutil"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	export "kubevirt.io/api/export/v1alpha1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetPVCNameTemplate_UniversalDefault(t *testing.T) {
	plan := &api.Plan{}

	template := planbase.GetPVCNameTemplate(plan, "vm1-id")
	expected := planbase.DefaultPVCNameTemplate
	if template != expected {
		t.Errorf("expected universal default template %q, got %q", expected, template)
	}
}

func TestGetPVCNameTemplate_OCPDefault(t *testing.T) {
	ocpType := api.OpenShift
	plan := &api.Plan{}
	plan.Provider.Source = &api.Provider{
		Spec: api.ProviderSpec{Type: &ocpType},
	}
	template := planbase.GetPVCNameTemplate(plan, "vm1-id")
	if template != planbase.DefaultOCPPVCNameTemplate {
		t.Errorf("expected OCP default template %q, got %q", planbase.DefaultOCPPVCNameTemplate, template)
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

func TestDataVolumes_PVCNameTemplate(t *testing.T) {
	const (
		ns      = "src-ns"
		vmName  = "test-vm"
		pvcName = "source-pvc"
		scName  = "source-sc"
	)

	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	_ = export.AddToScheme(scheme)
	_ = cdi.AddToScheme(scheme)

	ocpType := api.OpenShift
	newBuilder := func(objs ...runtime.Object) *Builder {
		srcClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
		dstClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		plan := &api.Plan{
			ObjectMeta: metav1.ObjectMeta{Name: "test-plan"},
			Spec: api.PlanSpec{
				TargetNamespace: "target-ns",
			},
		}
		plan.Provider.Source = &api.Provider{
			Spec: api.ProviderSpec{Type: &ocpType},
		}
		return &Builder{
			Context: &plancontext.Context{
				Plan: plan,
				Map: struct {
					Network *api.NetworkMap
					Storage *api.StorageMap
				}{
					Storage: &api.StorageMap{
						Spec: api.StorageMapSpec{
							Map: []api.StoragePair{{
								Source:      ref.Ref{Name: scName},
								Destination: api.DestinationStorage{StorageClass: "dest-sc"},
							}},
						},
					},
				},
				Destination: plancontext.Destination{Client: dstClient},
				Log:         logging.WithName("ocp-builder-test"),
			},
			sourceClient: srcClient,
		}
	}

	storageClass := scName
	makePVC := func(name string) *core.PersistentVolumeClaim {
		return &core.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: core.PersistentVolumeClaimSpec{
				StorageClassName: &storageClass,
				Resources: core.VolumeResourceRequirements{
					Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("1Gi")},
				},
			},
		}
	}
	makeExport := func(volumeName string) *export.VirtualMachineExport {
		return &export.VirtualMachineExport{
			ObjectMeta: metav1.ObjectMeta{Name: vmName, Namespace: ns},
			Status: &export.VirtualMachineExportStatus{
				Links: &export.VirtualMachineExportLinks{
					External: &export.VirtualMachineExportLink{
						Cert: "cert",
						Volumes: []export.VirtualMachineExportVolume{{
							Name: volumeName,
							Formats: []export.VirtualMachineExportVolumeFormat{
								{Format: export.KubeVirtGz, Url: "https://export.example/disk.gz"},
							},
						}},
					},
				},
			},
		}
	}
	dvTemplate := &cdi.DataVolume{
		ObjectMeta: metav1.ObjectMeta{Namespace: "target-ns", Annotations: map[string]string{}},
	}
	secret := &core.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret"}}
	configMap := &core.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm"}}
	vmRef := ref.Ref{ID: "vm-1", Name: vmName, Namespace: ns}

	t.Run("default uses exact source PVC name", func(t *testing.T) {
		builder := newBuilder(makeExport(pvcName), makePVC(pvcName))
		dvs, err := builder.DataVolumes(vmRef, secret, configMap, dvTemplate.DeepCopy(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(dvs) != 1 {
			t.Fatalf("expected 1 DataVolume, got %d", len(dvs))
		}
		if dvs[0].Name != pvcName {
			t.Errorf("expected Name %q, got Name=%q GenerateName=%q", pvcName, dvs[0].Name, dvs[0].GenerateName)
		}
		if dvs[0].GenerateName != "" {
			t.Errorf("expected empty GenerateName for OCP default, got %q", dvs[0].GenerateName)
		}
	})

	t.Run("explicit non-OCP template uses generateName", func(t *testing.T) {
		builder := newBuilder(makeExport(pvcName), makePVC(pvcName))
		builder.Plan.Spec.PVCNameTemplate = planbase.DefaultPVCNameTemplate
		builder.Plan.Spec.PVCNameTemplateUseGenerateName = ptr.To(true)
		dvs, err := builder.DataVolumes(vmRef, secret, configMap, dvTemplate.DeepCopy(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(dvs) != 1 {
			t.Fatalf("expected 1 DataVolume, got %d", len(dvs))
		}
		want := "test-plan-test-vm-disk-0-"
		if dvs[0].GenerateName != want {
			t.Errorf("expected GenerateName %q, got Name=%q GenerateName=%q", want, dvs[0].Name, dvs[0].GenerateName)
		}
	})

	t.Run("setPVCNameFromTemplate with trunc template", func(t *testing.T) {
		builder := newBuilder()
		objectMeta := &metav1.ObjectMeta{}
		err := builder.setPVCNameFromTemplate(objectMeta, vmRef, &core.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: ns},
		}, 0, planbase.DefaultPVCNameTemplate, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "test-plan-test-vm-disk-0-"
		if objectMeta.GenerateName != want {
			t.Errorf("expected GenerateName %q, got %q", want, objectMeta.GenerateName)
		}
	})

	t.Run("plan template overrides OCP default", func(t *testing.T) {
		builder := newBuilder(makeExport(pvcName), makePVC(pvcName))
		builder.Plan.Spec.PVCNameTemplate = "custom-{{.DiskIndex}}"
		builder.Plan.Spec.PVCNameTemplateUseGenerateName = ptr.To(false)
		dvs, err := builder.DataVolumes(vmRef, secret, configMap, dvTemplate.DeepCopy(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(dvs) != 1 {
			t.Fatalf("expected 1 DataVolume, got %d", len(dvs))
		}
		if dvs[0].Name != "custom-0" {
			t.Errorf("expected Name %q, got Name=%q GenerateName=%q", "custom-0", dvs[0].Name, dvs[0].GenerateName)
		}
	})

	t.Run("OCP global template overrides OCP default", func(t *testing.T) {
		prev := settings.Settings.OCPPVCNameTemplate
		settings.Settings.OCPPVCNameTemplate = "ocpglob-{{.DiskIndex}}"
		t.Cleanup(func() { settings.Settings.OCPPVCNameTemplate = prev })

		builder := newBuilder(makeExport(pvcName), makePVC(pvcName))
		builder.Plan.Spec.PVCNameTemplateUseGenerateName = ptr.To(false)
		dvs, err := builder.DataVolumes(vmRef, secret, configMap, dvTemplate.DeepCopy(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(dvs) != 1 {
			t.Fatalf("expected 1 DataVolume, got %d", len(dvs))
		}
		if dvs[0].Name != "ocpglob-0" {
			t.Errorf("expected Name %q, got Name=%q GenerateName=%q", "ocpglob-0", dvs[0].Name, dvs[0].GenerateName)
		}
	})
}

func TestSelectExportFormat(t *testing.T) {
	t.Run("prefers kubevirt gzip over archive", func(t *testing.T) {
		url, contentType, ok := selectExportFormat([]export.VirtualMachineExportVolumeFormat{
			{Format: export.ArchiveGz, Url: "https://export.example/disk.tar.gz"},
			{Format: export.KubeVirtGz, Url: "https://export.example/disk.gz"},
		})
		if !ok {
			t.Fatal("expected ok")
		}
		if url != "https://export.example/disk.gz" {
			t.Errorf("expected kubevirt url, got %q", url)
		}
		if contentType != cdi.DataVolumeKubeVirt {
			t.Errorf("expected kubevirt content type, got %q", contentType)
		}
	})

	t.Run("uses archive when only tar.gz is available", func(t *testing.T) {
		url, contentType, ok := selectExportFormat([]export.VirtualMachineExportVolumeFormat{
			{Format: export.ArchiveGz, Url: "https://export.example/fs.tar.gz"},
			{Format: export.Dir, Url: "https://export.example/fs/"},
		})
		if !ok {
			t.Fatal("expected ok")
		}
		if url != "https://export.example/fs.tar.gz" {
			t.Errorf("expected archive url, got %q", url)
		}
		if contentType != cdi.DataVolumeArchive {
			t.Errorf("expected archive content type, got %q", contentType)
		}
	})

	t.Run("returns false when no supported format", func(t *testing.T) {
		_, _, ok := selectExportFormat([]export.VirtualMachineExportVolumeFormat{
			{Format: export.Dir, Url: "https://export.example/fs/"},
		})
		if ok {
			t.Fatal("expected not ok")
		}
	})
}

func TestDataVolumes_ArchiveExportSetsContentType(t *testing.T) {
	const (
		ns      = "src-ns"
		vmName  = "test-vm"
		pvcName = "virtiofs-pvc"
		scName  = "source-sc"
	)

	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	_ = export.AddToScheme(scheme)
	_ = cdi.AddToScheme(scheme)

	ocpType := api.OpenShift
	storageClass := scName
	fsMode := core.PersistentVolumeFilesystem
	srcClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(
		&export.VirtualMachineExport{
			ObjectMeta: metav1.ObjectMeta{Name: vmName, Namespace: ns},
			Status: &export.VirtualMachineExportStatus{
				Links: &export.VirtualMachineExportLinks{
					External: &export.VirtualMachineExportLink{
						Cert: "cert",
						Volumes: []export.VirtualMachineExportVolume{{
							Name: pvcName,
							Formats: []export.VirtualMachineExportVolumeFormat{
								{Format: export.ArchiveGz, Url: "https://export.example/fs.tar.gz"},
							},
						}},
					},
				},
			},
		},
		&core.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: ns},
			Spec: core.PersistentVolumeClaimSpec{
				StorageClassName: &storageClass,
				VolumeMode:       &fsMode,
				AccessModes:      []core.PersistentVolumeAccessMode{core.ReadWriteMany},
				Resources: core.VolumeResourceRequirements{
					Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("1Gi")},
				},
			},
		},
	).Build()
	dstClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	plan := &api.Plan{
		ObjectMeta: metav1.ObjectMeta{Name: "test-plan"},
		Spec:       api.PlanSpec{TargetNamespace: "target-ns"},
	}
	plan.Provider.Source = &api.Provider{Spec: api.ProviderSpec{Type: &ocpType}}
	builder := &Builder{
		Context: &plancontext.Context{
			Plan: plan,
			Map: struct {
				Network *api.NetworkMap
				Storage *api.StorageMap
			}{
				Storage: &api.StorageMap{
					Spec: api.StorageMapSpec{
						Map: []api.StoragePair{{
							Source:      ref.Ref{Name: scName},
							Destination: api.DestinationStorage{StorageClass: "dest-sc"},
						}},
					},
				},
			},
			Destination: plancontext.Destination{Client: dstClient},
			Log:         logging.WithName("ocp-builder-test"),
		},
		sourceClient: srcClient,
	}

	dvs, err := builder.DataVolumes(
		ref.Ref{ID: "vm-1", Name: vmName, Namespace: ns},
		&core.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret"}},
		&core.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm"}},
		&cdi.DataVolume{ObjectMeta: metav1.ObjectMeta{Namespace: "target-ns", Annotations: map[string]string{}}},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dvs) != 1 {
		t.Fatalf("expected 1 DataVolume, got %d", len(dvs))
	}
	dv := dvs[0]
	if dv.Spec.ContentType != cdi.DataVolumeArchive {
		t.Errorf("expected contentType archive, got %q", dv.Spec.ContentType)
	}
	if dv.Spec.Source == nil || dv.Spec.Source.HTTP == nil || dv.Spec.Source.HTTP.URL != "https://export.example/fs.tar.gz" {
		t.Errorf("expected archive export URL, got %#v", dv.Spec.Source)
	}
	if dv.Spec.Storage == nil || dv.Spec.Storage.VolumeMode == nil || *dv.Spec.Storage.VolumeMode != core.PersistentVolumeFilesystem {
		t.Errorf("expected Filesystem volume mode, got %#v", dv.Spec.Storage)
	}
	if len(dv.Spec.Storage.AccessModes) != 1 || dv.Spec.Storage.AccessModes[0] != core.ReadWriteMany {
		t.Errorf("expected source access modes preserved, got %#v", dv.Spec.Storage.AccessModes)
	}
}

func TestDataVolumes_ArchiveExportKeepsFilesystemWithBlockDestination(t *testing.T) {
	const (
		ns      = "src-ns"
		vmName  = "test-vm"
		pvcName = "virtiofs-pvc"
		scName  = "source-sc"
	)

	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	_ = export.AddToScheme(scheme)
	_ = cdi.AddToScheme(scheme)

	ocpType := api.OpenShift
	storageClass := scName
	fsMode := core.PersistentVolumeFilesystem
	srcClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(
		&export.VirtualMachineExport{
			ObjectMeta: metav1.ObjectMeta{Name: vmName, Namespace: ns},
			Status: &export.VirtualMachineExportStatus{
				Links: &export.VirtualMachineExportLinks{
					External: &export.VirtualMachineExportLink{
						Cert: "cert",
						Volumes: []export.VirtualMachineExportVolume{{
							Name: pvcName,
							Formats: []export.VirtualMachineExportVolumeFormat{
								{Format: export.ArchiveGz, Url: "https://export.example/fs.tar.gz"},
							},
						}},
					},
				},
			},
		},
		&core.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: ns},
			Spec: core.PersistentVolumeClaimSpec{
				StorageClassName: &storageClass,
				VolumeMode:       &fsMode,
				Resources: core.VolumeResourceRequirements{
					Requests: core.ResourceList{core.ResourceStorage: resource.MustParse("1Gi")},
				},
			},
		},
	).Build()
	dstClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	plan := &api.Plan{
		ObjectMeta: metav1.ObjectMeta{Name: "test-plan"},
		Spec:       api.PlanSpec{TargetNamespace: "target-ns"},
	}
	plan.Provider.Source = &api.Provider{Spec: api.ProviderSpec{Type: &ocpType}}
	builder := &Builder{
		Context: &plancontext.Context{
			Plan: plan,
			Map: struct {
				Network *api.NetworkMap
				Storage *api.StorageMap
			}{
				Storage: &api.StorageMap{
					Spec: api.StorageMapSpec{
						Map: []api.StoragePair{{
							Source: ref.Ref{Name: scName},
							Destination: api.DestinationStorage{
								StorageClass: "dest-sc",
								VolumeMode:   core.PersistentVolumeBlock,
							},
						}},
					},
				},
			},
			Destination: plancontext.Destination{Client: dstClient},
			Log:         logging.WithName("ocp-builder-test"),
		},
		sourceClient: srcClient,
	}

	dvs, err := builder.DataVolumes(
		ref.Ref{ID: "vm-1", Name: vmName, Namespace: ns},
		&core.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret"}},
		&core.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm"}},
		&cdi.DataVolume{ObjectMeta: metav1.ObjectMeta{Namespace: "target-ns", Annotations: map[string]string{}}},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dvs) != 1 {
		t.Fatalf("expected 1 DataVolume, got %d", len(dvs))
	}
	dv := dvs[0]
	if dv.Spec.ContentType != cdi.DataVolumeArchive {
		t.Errorf("expected contentType archive, got %q", dv.Spec.ContentType)
	}
	if dv.Spec.Storage == nil || dv.Spec.Storage.VolumeMode == nil || *dv.Spec.Storage.VolumeMode != core.PersistentVolumeFilesystem {
		t.Errorf("expected Filesystem volume mode for archive import, got %#v", dv.Spec.Storage)
	}
}
