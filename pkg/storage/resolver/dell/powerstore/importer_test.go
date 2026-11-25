package powerstore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/storage/resolver"
)

// mockPowerstoreAPI creates a test server that handles PowerStore REST volume queries.
// volumesByFilter maps a "<field>=eq.<value>" query string to a volume name.
func mockPowerstoreAPI(t *testing.T, volumesByFilter map[string]string) (*httptest.Server, *PowerstoreImporter) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/rest/volume" {
			http.NotFound(w, r)
			return
		}

		var filter string
		for key, vals := range r.URL.Query() {
			if key == "select" {
				continue
			}
			filter = key + "=" + vals[0]
		}
		name, ok := volumesByFilter[filter]
		if !ok {
			_ = json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]string{{"name": name}})
	}))

	imp := &PowerstoreImporter{
		baseURL: srv.URL,
		user:    "test",
		pass:    "test",
		client:  srv.Client(),
	}
	return srv, imp
}

func TestNewPowerstoreImporter(t *testing.T) {
	validTests := []struct {
		input string
		want  string
	}{
		{"https://host", "https://host"},
		{"https://10.46.2.10", "https://10.46.2.10"},
		{"https://10.46.2.10/", "https://10.46.2.10"},
		{"10.46.2.10", "https://10.46.2.10"},
		{"host:8080", "https://host:8080"},
	}
	for _, tt := range validTests {
		imp, err := NewPowerstoreImporter(tt.input, "user", "pass", true)
		if err != nil {
			t.Fatalf("NewPowerstoreImporter(%q) unexpected error: %v", tt.input, err)
		}
		if imp.baseURL != tt.want {
			t.Errorf("NewPowerstoreImporter(%q) baseURL = %q, want %q", tt.input, imp.baseURL, tt.want)
		}
	}

	invalidTests := []string{
		"",
		"http://host:8080",
	}
	for _, input := range invalidTests {
		_, err := NewPowerstoreImporter(input, "user", "pass", true)
		if err == nil {
			t.Errorf("NewPowerstoreImporter(%q) expected error for missing scheme, got nil", input)
		}
	}
}

func TestResolveVVolRejected(t *testing.T) {
	srv, imp := mockPowerstoreAPI(t, map[string]string{})
	defer srv.Close()

	for _, id := range []string{"vvol:e8307953-a6a3-4adb-bce1-098c210ca53c", "naa.68ccf098000a2415fa5c15f00000000f"} {
		_, err := imp.Resolve(&resolver.DiskBacking{VVolID: id})
		if err == nil || !strings.Contains(err.Error(), "does not support VVol-backed disks") {
			t.Errorf("Resolve(VVolID=%q) error = %v, want VVol rejection", id, err)
		}
	}
}

func TestResolveRDM(t *testing.T) {
	naa := "naa.68ccf098000a2415fa5c15f00000000f"
	srv, imp := mockPowerstoreAPI(t, map[string]string{
		"wwn=eq." + naa: "tshefi-ecosystem-vmware",
	})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: naa, CompatibilityMode: "physicalMode"}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations[annotationKey] != "tshefi-ecosystem-vmware" {
		t.Errorf("unexpected annotation value: %v", annotations)
	}
}

func TestResolveVirtualModeRDMWithSnapshotsRejected(t *testing.T) {
	srv, imp := mockPowerstoreAPI(t, map[string]string{})
	defer srv.Close()

	backing := &resolver.DiskBacking{
		IsRDM:             true,
		DeviceName:        "naa.68ccf098000a2415fa5c15f00000000f",
		CompatibilityMode: "virtualMode",
		VMHasSnapshots:    true,
	}
	_, err := imp.Resolve(backing)
	if err == nil {
		t.Fatal("expected error for virtual-mode RDM with snapshots, got nil")
	}
	if !strings.Contains(err.Error(), "virtual-mode RDMs with snapshots") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveVirtualModeRDMWithoutSnapshots(t *testing.T) {
	naa := "naa.68ccf098000a2415fa5c15f00000000f"
	srv, imp := mockPowerstoreAPI(t, map[string]string{
		"wwn=eq." + naa: "tshefi-virtual-mode-vmware",
	})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: naa, CompatibilityMode: "virtualMode"}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations[annotationKey] != "tshefi-virtual-mode-vmware" {
		t.Errorf("unexpected annotation value: %v", annotations)
	}
}

func TestResolveRDMFromVML(t *testing.T) {
	// VML format from vSphere — WWN at hex[10:42]
	vml := "vml.020001000068ccf098000a2415fa5c15f00000000f565620202020"
	naa := "naa.68ccf098000a2415fa5c15f00000000f"
	srv, imp := mockPowerstoreAPI(t, map[string]string{
		"wwn=eq." + naa: "tshefi-vml-volume",
	})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: vml}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations[annotationKey] != "tshefi-vml-volume" {
		t.Errorf("unexpected annotation value: %v", annotations)
	}
}

func TestResolveVMDK(t *testing.T) {
	srv, imp := mockPowerstoreAPI(t, map[string]string{})
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
	srv, imp := mockPowerstoreAPI(t, map[string]string{})
	defer srv.Close()

	_, err := imp.Resolve(nil)
	if err == nil {
		t.Fatal("expected error for nil backing, got nil")
	}
}

func TestExtractWWN(t *testing.T) {
	tests := []struct {
		name       string
		deviceName string
		want       string
		wantErr    bool
	}{
		{
			name:       "naa prefix",
			deviceName: "naa.68CCF098000A2415FA5C15F0000000F",
			want:       "naa.68ccf098000a2415fa5c15f0000000f",
		},
		{
			name:       "vml format",
			deviceName: "vml.020001000068ccf098000a2415fa5c15f00000000f565620202020",
			want:       "naa.68ccf098000a2415fa5c15f00000000f",
		},
		{
			name:       "vml too short",
			deviceName: "vml.0200010000",
			wantErr:    true,
		},
		{
			name:       "vml without NAA type 6",
			deviceName: "vml.02000100005cccf098000a2415fa5c15f00000000f565620202020",
			wantErr:    true,
		},
		{
			name:       "vml device path",
			deviceName: "/vmfs/devices/disks/vml.020001000068ccf098000a2415fa5c15f00000000f565620202020",
			want:       "naa.68ccf098000a2415fa5c15f00000000f",
		},
		{
			name:       "naa device path",
			deviceName: "/vmfs/devices/disks/naa.68ccf098000a2415fa5c15f00000000f",
			want:       "naa.68ccf098000a2415fa5c15f00000000f",
		},
		{
			name:       "bare naa",
			deviceName: "68CCF098000A2415FA5C15F00000000F",
			want:       "naa.68ccf098000a2415fa5c15f00000000f",
		},
		{
			name:       "unrecognized device name",
			deviceName: "SOME-DEVICE",
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractWWN(tt.deviceName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("extractWWN(%q) error = %v, wantErr %v", tt.deviceName, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("extractWWN(%q) = %q, want %q", tt.deviceName, got, tt.want)
			}
		})
	}
}

func TestResolveInvalidDevice(t *testing.T) {
	srv, imp := mockPowerstoreAPI(t, map[string]string{})
	defer srv.Close()

	for _, name := range []string{"vml.0200", "naa.68ccf09800aabbccddeeff0011223344"} {
		if _, err := imp.Resolve(&resolver.DiskBacking{IsRDM: true, DeviceName: name}); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestVolumeNameByFieldErrors(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		baseURL string
		want    string
	}{
		{"bad url", nil, "http://%zz", "failed to build"},
		{"unreachable", nil, "http://127.0.0.1:1", "request failed"},
		{"truncated body", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "10")
			_, _ = w.Write([]byte("["))
		}, "", "failed to read"},
		{"non-200", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }, "", "returned 401"},
		{"bad json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("not json")) }, "", "failed to parse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imp := &PowerstoreImporter{baseURL: tt.baseURL, client: http.DefaultClient}
			if tt.handler != nil {
				srv := httptest.NewServer(tt.handler)
				defer srv.Close()
				imp.baseURL = srv.URL
			}
			_, err := imp.volumeNameByField("wwn", "naa.68ccf09800aabbccddeeff0011223344")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}
