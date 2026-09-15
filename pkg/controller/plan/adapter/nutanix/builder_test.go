package nutanix

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/nutanix"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/api/core/v1"
)

func TestConfigMapSetsCDICertKeys(t *testing.T) {
	cacert := []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----")
	secret := &core.Secret{
		Data: map[string][]byte{
			"ca.crt": cacert,
		},
	}
	configMap := &core.ConfigMap{}
	builder := &Builder{}

	err := builder.ConfigMap(ref.Ref{}, secret, configMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(configMap.BinaryData["ca.pem"], cacert) {
		t.Fatalf("expected ca.pem to match provider CA")
	}
	if !bytes.Equal(configMap.BinaryData["tls.crt"], cacert) {
		t.Fatalf("expected tls.crt to match provider CA for CDI nbdkit cainfo")
	}
}

func TestConfigMapInsecureDoesNotFetchEarly(t *testing.T) {
	secret := &core.Secret{
		Data: map[string][]byte{
			"insecureSkipVerify": []byte("true"),
		},
	}
	configMap := &core.ConfigMap{}
	builder := &Builder{}

	err := builder.ConfigMap(ref.Ref{}, secret, configMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if configMapHasCert(configMap) {
		t.Fatal("expected insecure ConfigMap() to defer cert fetch to DataVolumes()")
	}
}

func TestFetchCertFromURL(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(server.Close)

	builder := &Builder{}
	secret := &core.Secret{
		Data: map[string][]byte{"insecureSkipVerify": []byte("true")},
	}
	cacert, err := builder.fetchCertFromURL(server.URL, secret)
	if err != nil {
		t.Fatalf("fetchCertFromURL: %v", err)
	}
	if len(cacert) == 0 || !bytes.Contains(cacert, []byte("BEGIN CERTIFICATE")) {
		t.Fatalf("expected PEM certificate, got %q", cacert)
	}
}

func TestImportCertURLs_PrismElement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/nutanix/v3/clusters/list":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[{"status":{"resources":{"network":{"external_ip":"10.0.0.1"}}}}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	client := newConnectedTestClient(t, server.URL)
	builder := &Builder{
		Context: &plancontext.Context{
			Source: plancontext.Source{
				Provider: &api.Provider{Spec: api.ProviderSpec{URL: server.URL}},
			},
		},
	}
	vm := &model.VM{VM1: model.VM1{Disks: []model.Disk{{UUID: "disk-1"}}}}

	got, err := builder.importCertURLs(client, ref.Ref{ID: "vm-1"}, vm)
	if err != nil {
		t.Fatalf("importCertURLs: %v", err)
	}
	if len(got) != 1 || got[0] != server.URL {
		t.Fatalf("expected single URL %q, got %v", server.URL, got)
	}
}

func TestMergePEMCertificatesDedupes(t *testing.T) {
	pemBlock := []byte("-----BEGIN CERTIFICATE-----\nYQ==\n-----END CERTIFICATE-----\n")
	merged := mergePEMCertificates(pemBlock, pemBlock, append(pemBlock, pemBlock...))
	if bytes.Count(merged, []byte("BEGIN CERTIFICATE")) != 1 {
		t.Fatalf("expected one certificate in merged bundle, got %q", merged)
	}
}

func TestBuildImportCertBundleSecureUsesProviderCAOnly(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(server.Close)

	providerCA := []byte("-----BEGIN CERTIFICATE-----\nprovider\n-----END CERTIFICATE-----\n")
	secret := &core.Secret{
		Data: map[string][]byte{
			"ca.crt": providerCA,
		},
	}
	builder := &Builder{}
	bundle := builder.buildImportCertBundle(secret, []string{server.URL})
	if !bytes.Equal(bundle, providerCA) {
		t.Fatalf("expected provider ca.crt only, got %q", bundle)
	}
}

func TestBuildImportCertBundleMergesProviderAndFetch(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(server.Close)

	providerCA := []byte("-----BEGIN CERTIFICATE-----\nprovider\n-----END CERTIFICATE-----\n")
	secret := &core.Secret{
		Data: map[string][]byte{
			"ca.crt":             providerCA,
			"insecureSkipVerify": []byte("true"),
		},
	}
	builder := &Builder{}
	bundle := builder.buildImportCertBundle(secret, []string{server.URL})
	if !bytes.Contains(bundle, []byte("provider")) {
		t.Fatal("expected provider ca.crt in bundle")
	}
	if !bytes.Contains(bundle, []byte("BEGIN CERTIFICATE")) {
		t.Fatalf("expected fetched leaf in bundle, got %q", bundle)
	}
	if bytes.Count(bundle, []byte("BEGIN CERTIFICATE")) < 2 {
		t.Fatalf("expected at least two certificates, got %q", bundle)
	}
}

func TestAppendUniqueImportCertURLByHost(t *testing.T) {
	seen := map[string]struct{}{}
	var urls []string
	var err error
	urls, err = appendUniqueImportCertURL(urls, seen, "https://10.0.0.1:9440")
	if err != nil || len(urls) != 1 {
		t.Fatalf("first URL: urls=%v err=%v", urls, err)
	}
	urls, err = appendUniqueImportCertURL(urls, seen, "https://10.0.0.1:9440/entity_download/foo")
	if err != nil || len(urls) != 1 {
		t.Fatalf("same host different path should dedupe: urls=%v err=%v", urls, err)
	}
	urls, err = appendUniqueImportCertURL(urls, seen, "https://10.0.0.2:9440")
	if err != nil || len(urls) != 2 {
		t.Fatalf("second host: urls=%v err=%v", urls, err)
	}
}

func TestBootDiskUUID(t *testing.T) {
	tests := []struct {
		name            string
		bootDeviceOrder string
		disks           []model.Disk
		want            string
	}{
		{
			name:            "first block disk in disk_list when indices collide",
			bootDeviceOrder: "DISK,CDROM,NETWORK",
			disks: []model.Disk{
				{UUID: "sata-0", DeviceType: "DISK", AdapterType: adapterSATA, DeviceIndex: 0},
				{UUID: "scsi-0", DeviceType: "DISK", AdapterType: adapterSCSI, DeviceIndex: 0},
			},
			want: "sata-0",
		},
		{
			name:            "skips cdrom before first disk in inventory order",
			bootDeviceOrder: "CDROM,DISK,NETWORK",
			disks: []model.Disk{
				{UUID: "cdrom-1", DeviceType: "CDROM", IsCdrom: true},
				{UUID: "sata-0", DeviceType: "DISK", AdapterType: adapterSATA, DeviceIndex: 0},
				{UUID: "scsi-0", DeviceType: "DISK", AdapterType: adapterSCSI, DeviceIndex: 0},
			},
			want: "sata-0",
		},
		{
			name: "defaults to disk cdrom network when boot order empty",
			disks: []model.Disk{
				{UUID: "cdrom-1", DeviceType: "CDROM", IsCdrom: true},
				{UUID: "disk-1", DeviceType: "DISK"},
			},
			want: "disk-1",
		},
		{
			name:            "falls back to first non-cdrom when boot order has no disk",
			bootDeviceOrder: "CDROM,NETWORK",
			disks: []model.Disk{
				{UUID: "cdrom-1", DeviceType: "CDROM", IsCdrom: true},
				{UUID: "disk-1", DeviceType: "DISK"},
			},
			want: "disk-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &model.VM{VM1: model.VM1{
				BootDeviceOrder: tt.bootDeviceOrder,
				Disks:           tt.disks,
			}}
			if got := bootDiskUUID(vm); got != tt.want {
				t.Fatalf("bootDiskUUID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapDisks_BootOrder(t *testing.T) {
	tests := []struct {
		name            string
		bootDeviceOrder string
		disks           []model.Disk
		wantBootDisk    string
	}{
		{
			name:            "exactly one boot disk when adapter indices collide",
			bootDeviceOrder: "DISK,CDROM,NETWORK",
			disks: []model.Disk{
				{UUID: "sata-0", DeviceType: "DISK", AdapterType: adapterSATA, DeviceIndex: 0},
				{UUID: "scsi-0", DeviceType: "DISK", AdapterType: adapterSCSI, DeviceIndex: 0},
			},
			wantBootDisk: "sata-0",
		},
		{
			name:            "boot order follows inventory not adapter priority",
			bootDeviceOrder: "DISK,CDROM,NETWORK",
			disks: []model.Disk{
				{UUID: "scsi-0", DeviceType: "DISK", AdapterType: adapterSCSI, DeviceIndex: 0},
				{UUID: "sata-0", DeviceType: "DISK", AdapterType: adapterSATA, DeviceIndex: 0},
			},
			wantBootDisk: "scsi-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &model.VM{VM1: model.VM1{
				BootDeviceOrder: tt.bootDeviceOrder,
				Disks:           tt.disks,
			}}
			pvcs := make([]*core.PersistentVolumeClaim, 0, len(tt.disks))
			for _, disk := range tt.disks {
				if disk.IsCdrom {
					continue
				}
				pvcs = append(pvcs, &core.PersistentVolumeClaim{
					ObjectMeta: meta.ObjectMeta{
						Annotations: map[string]string{planbase.AnnDiskSource: disk.UUID},
					},
				})
			}

			object := &cnv.VirtualMachineSpec{Template: &cnv.VirtualMachineInstanceTemplateSpec{}}
			(&Builder{}).mapDisks(vm, pvcs, object)

			var bootCount int
			for _, disk := range object.Template.Spec.Domain.Devices.Disks {
				if disk.BootOrder == nil {
					continue
				}
				bootCount++
				if disk.Name != tt.wantBootDisk {
					t.Fatalf("expected boot order on %q, got %q", tt.wantBootDisk, disk.Name)
				}
			}
			if bootCount != 1 {
				t.Fatalf("expected exactly one boot disk, got %d", bootCount)
			}
		})
	}
}
