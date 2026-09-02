package ocp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

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
						Cert: "",
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

func TestTlsCertForExport(t *testing.T) {
	caPEM, serverCert, _ := newTestTLSCredentials(t)
	listener := startTestTLSServer(t, serverCert)
	exportURL := "https://" + listener.Addr().String()

	t.Run("empty cert", func(t *testing.T) {
		got, err := tlsCertForExport(exportURL, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Fatalf("expected empty cert, got %q", got)
		}
	})

	t.Run("matching CA", func(t *testing.T) {
		got, err := tlsCertForExport(exportURL, string(caPEM))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != string(caPEM) {
			t.Fatalf("expected provided CA, got %q", got)
		}
	})

	t.Run("wrong CA when system also untrusted", func(t *testing.T) {
		wrongCAPEM, _, _ := newTestTLSCredentials(t)
		_, err := tlsCertForExport(exportURL, string(wrongCAPEM))
		if err == nil {
			t.Fatal("expected error when neither VMExport cert nor system CA verifies")
		}
	})
}

func TestCreateDataVolumeSpec_OmitsCertConfigMap(t *testing.T) {
	spec := createDataVolumeSpec(resource.MustParse("1Gi"), "sc", "https://example/disk", "", "secret")
	if spec.Source.HTTP.CertConfigMap != "" {
		t.Fatalf("expected empty CertConfigMap, got %q", spec.Source.HTTP.CertConfigMap)
	}

	spec = createDataVolumeSpec(resource.MustParse("1Gi"), "sc", "https://example/disk", "ca-cm", "secret")
	if spec.Source.HTTP.CertConfigMap != "ca-cm" {
		t.Fatalf("expected CertConfigMap ca-cm, got %q", spec.Source.HTTP.CertConfigMap)
	}
}

func newTestTLSCredentials(t *testing.T) (caPEM []byte, serverCert tls.Certificate, serverKey *ecdsa.PrivateKey) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	serverKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate server key: %v", err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create server cert: %v", err)
	}
	serverCert = tls.Certificate{
		Certificate: [][]byte{serverDER, caDER},
		PrivateKey:  serverKey,
	}
	return caPEM, serverCert, serverKey
}

func startTestTLSServer(t *testing.T, serverCert tls.Certificate) net.Listener {
	t.Helper()

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{serverCert},
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	return listener
}
