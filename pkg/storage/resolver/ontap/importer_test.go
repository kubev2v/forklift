package ontap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/storage/resolver"
)

// mockOntapAPI creates a test server that handles ONTAP REST API LUN queries.
// lunsBySerial maps an ASCII serial number to a LUN path (e.g. "/vol/flexvol/lun0").
func mockOntapAPI(t *testing.T, lunsBySerial map[string]string) (*httptest.Server, *OntapImporter) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || !strings.HasPrefix(r.URL.Path, "/api/storage/luns") {
			http.NotFound(w, r)
			return
		}

		serial := r.URL.Query().Get("serial_number")
		name, ok := lunsBySerial[serial]
		if !ok {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"num_records": 0,
				"records":     []interface{}{},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"num_records": 1,
			"records":     []map[string]string{{"name": name}},
		})
	}))

	imp := &OntapImporter{
		baseURL:     srv.URL,
		user:        "test",
		pass:        "test",
		svm:         "test-svm",
		backendUUID: "test-backend-uuid",
		driverType:  DriverOntapSan,
		httpClient:  srv.Client(),
	}
	return srv, imp
}

func TestNewOntapImporter(t *testing.T) {
	validTests := []struct {
		host string
		want string
	}{
		{"https://host", "https://host"},
		{"https://10.46.246.90", "https://10.46.246.90"},
		{"https://10.46.246.90/", "https://10.46.246.90"},
		{"http://host:8080", "http://host:8080"},
	}
	for _, tt := range validTests {
		imp, err := NewOntapImporter(tt.host, "user", "pass", "svm", "uuid", "", true, nil, "")
		if err != nil {
			t.Fatalf("NewOntapImporter(%q) unexpected error: %v", tt.host, err)
		}
		if imp.baseURL != tt.want {
			t.Errorf("NewOntapImporter(%q) baseURL = %q, want %q", tt.host, imp.baseURL, tt.want)
		}
	}

	invalidHosts := []string{
		"10.46.246.90",
		"host:8080",
		"",
	}
	for _, host := range invalidHosts {
		_, err := NewOntapImporter(host, "user", "pass", "svm", "uuid", "", true, nil, "")
		if err == nil {
			t.Errorf("NewOntapImporter(%q) expected error for missing scheme, got nil", host)
		}
	}

	// Missing SVM
	_, err := NewOntapImporter("https://host", "user", "pass", "", "uuid", "", true, nil, "")
	if err == nil {
		t.Fatal("expected error for empty SVM, got nil")
	}

	// Missing backend UUID
	_, err = NewOntapImporter("https://host", "user", "pass", "svm", "", "", true, nil, "")
	if err == nil {
		t.Fatal("expected error for empty backendUUID, got nil")
	}
}

func TestResolveRDM(t *testing.T) {
	serial := "819TI$X101LA"
	hexSerial := "383139544924583130314c41"
	naa := "naa." + ontapProviderID + hexSerial
	lunPath := "/vol/eco_lun01_vmfs01/eco_lun01_vmfs01"

	srv, imp := mockOntapAPI(t, map[string]string{serial: lunPath})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: naa}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if annotations[annImportOriginalName] != "eco_lun01_vmfs01" {
		t.Errorf("expected importOriginalName = 'eco_lun01_vmfs01', got %q", annotations[annImportOriginalName])
	}
	if annotations[annImportBackendUUID] != "test-backend-uuid" {
		t.Errorf("expected importBackendUUID = 'test-backend-uuid', got %q", annotations[annImportBackendUUID])
	}
	if annotations[annNotManaged] != "true" {
		t.Errorf("expected notManaged = 'true', got %q", annotations[annNotManaged])
	}
}

func TestResolveRDMFromVML(t *testing.T) {
	serial := "819TI$X102P-"
	vml := "vml.0200180000600a098038313954492458313032502d4c554e20432d"
	lunPath := "/vol/rdm_10g/lun0"

	srv, imp := mockOntapAPI(t, map[string]string{serial: lunPath})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: vml}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations[annImportOriginalName] != "rdm_10g" {
		t.Errorf("expected importOriginalName = 'rdm_10g', got %q", annotations[annImportOriginalName])
	}
}

func TestResolveRDMEconomy(t *testing.T) {
	serial := "819TI$X101LA"
	hexSerial := "383139544924583130314c41"
	naa := "naa." + ontapProviderID + hexSerial
	lunPath := "/vol/trident_lun_pool_abc/lun1"

	srv, imp := mockOntapAPI(t, map[string]string{serial: lunPath})
	defer srv.Close()
	imp.driverType = DriverOntapSanEconomy

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: naa}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations[annImportOriginalName] != "trident_lun_pool_abc/lun1" {
		t.Errorf("expected importOriginalName = 'trident_lun_pool_abc/lun1', got %q", annotations[annImportOriginalName])
	}
}

func TestResolveRDMBadNAA(t *testing.T) {
	srv, imp := mockOntapAPI(t, map[string]string{})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60002ac0badbadserial"}
	_, err := imp.Resolve(backing)
	if err == nil {
		t.Fatal("expected error for non-ONTAP NAA, got nil")
	}
	if !strings.Contains(err.Error(), "ONTAP OUI") {
		t.Errorf("expected OUI error, got: %v", err)
	}
}

func TestResolveRDMNotFound(t *testing.T) {
	srv, imp := mockOntapAPI(t, map[string]string{})
	defer srv.Close()

	hexSerial := "4e4f53554348"
	naa := "naa." + ontapProviderID + hexSerial
	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: naa}

	_, err := imp.Resolve(backing)
	if err == nil {
		t.Fatal("expected error for unknown serial, got nil")
	}
	if !strings.Contains(err.Error(), "no ONTAP LUN found") {
		t.Errorf("expected 'no ONTAP LUN found' error, got: %v", err)
	}
}

func TestResolveVVol(t *testing.T) {
	srv, imp := mockOntapAPI(t, map[string]string{})
	defer srv.Close()

	backing := &resolver.DiskBacking{VVolID: "vvol:some-uuid"}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("expected nil error for VVol (not yet supported by ONTAP importer), got: %v", err)
	}
	if annotations != nil {
		t.Errorf("expected nil annotations for VVol (not yet supported by ONTAP importer), got: %v", annotations)
	}
}

func TestResolveVMDK(t *testing.T) {
	srv, imp := mockOntapAPI(t, map[string]string{})
	defer srv.Close()

	backing := &resolver.DiskBacking{DeviceName: "[ds] vm/vm.vmdk"}
	_, err := imp.Resolve(backing)
	if err == nil {
		t.Fatal("expected error for VMDK, got nil")
	}
	if !strings.Contains(err.Error(), "does not support VMDK") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveNilBacking(t *testing.T) {
	srv, imp := mockOntapAPI(t, map[string]string{})
	defer srv.Close()

	_, err := imp.Resolve(nil)
	if err == nil {
		t.Fatal("expected error for nil backing, got nil")
	}
}

func TestExtractSerialFromNAA(t *testing.T) {
	tests := []struct {
		name    string
		naa     string
		want    string
		wantErr bool
	}{
		{
			name: "valid lowercase",
			naa:  "naa.600a0980383139544924583130314c41",
			want: "819TI$X101LA",
		},
		{
			name: "valid uppercase",
			naa:  "naa.600A0980383139544924583130314C41",
			want: "819TI$X101LA",
		},
		{
			name: "without naa prefix",
			naa:  "600a0980383139544924583130314c41",
			want: "819TI$X101LA",
		},
		{
			name: "VML format from real ONTAP RDM",
			naa:  "vml.0200180000600a098038313954492458313032502d4c554e20432d",
			want: "819TI$X102P-",
		},
		{
			name:    "VML too short",
			naa:     "vml.0200180000",
			wantErr: true,
		},
		{
			name:    "wrong OUI",
			naa:     "naa.60002ac0badbadserial",
			wantErr: true,
		},
		{
			name:    "empty string",
			naa:     "",
			wantErr: true,
		},
		{
			name:    "only OUI no serial",
			naa:     "naa.600a0980",
			wantErr: true,
		},
		{
			name:    "invalid hex after OUI",
			naa:     "naa.600a0980zzzz",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractSerialFromNAA(tt.naa)
			if (err != nil) != tt.wantErr {
				t.Fatalf("extractSerialFromNAA(%q) error = %v, wantErr %v", tt.naa, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("extractSerialFromNAA(%q) = %q, want %q", tt.naa, got, tt.want)
			}
		})
	}
}

func TestFormatImportName(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		driverType string
		want       string
		wantErr    bool
	}{
		{
			name:       "ontap-san standard path",
			path:       "/vol/eco_lun01_vmfs01/eco_lun01_vmfs01",
			driverType: DriverOntapSan,
			want:       "eco_lun01_vmfs01",
		},
		{
			name:       "ontap-san lun0 path",
			path:       "/vol/my_flexvol/lun0",
			driverType: DriverOntapSan,
			want:       "my_flexvol",
		},
		{
			name:       "ontap-san-economy returns FlexVol/LUN",
			path:       "/vol/trident_lun_pool_abc/lun1",
			driverType: DriverOntapSanEconomy,
			want:       "trident_lun_pool_abc/lun1",
		},
		{
			name:       "ontap-san-economy standard path",
			path:       "/vol/rdm_10g/lun0",
			driverType: DriverOntapSanEconomy,
			want:       "rdm_10g/lun0",
		},
		{
			name:       "empty driver defaults to ontap-san",
			path:       "/vol/my_flexvol/lun0",
			driverType: "",
			want:       "my_flexvol",
		},
		{
			name:       "too short",
			path:       "/vol/",
			driverType: DriverOntapSan,
			wantErr:    true,
		},
		{
			name:       "empty",
			path:       "",
			driverType: DriverOntapSan,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formatImportName(tt.path, tt.driverType)
			if (err != nil) != tt.wantErr {
				t.Fatalf("formatImportName(%q, %q) error = %v, wantErr %v", tt.path, tt.driverType, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("formatImportName(%q, %q) = %q, want %q", tt.path, tt.driverType, got, tt.want)
			}
		})
	}
}
