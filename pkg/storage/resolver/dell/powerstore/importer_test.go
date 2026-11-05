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
		{"http://host:8080", "http://host:8080"},
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
	}
	for _, input := range invalidTests {
		_, err := NewPowerstoreImporter(input, "user", "pass", true)
		if err == nil {
			t.Errorf("NewPowerstoreImporter(%q) expected error for missing scheme, got nil", input)
		}
	}
}

func TestResolveVVolByID(t *testing.T) {
	uuid := "e8307953-a6a3-4adb-bce1-098c210ca53c"
	srv, imp := mockPowerstoreAPI(t, map[string]string{
		"id=eq." + uuid: "my-powerstore-volume",
	})
	defer srv.Close()

	backing := &resolver.DiskBacking{VVolID: "vvol:" + uuid}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations[annotationKey] != "my-powerstore-volume" {
		t.Errorf("unexpected annotation value: %v", annotations)
	}
}

func TestResolveVVolByWWN(t *testing.T) {
	// PowerStore (like HPE Primera) can report the VVol BackingObjectId as a WWN.
	naa := "naa.68ccf098000a2415fa5c15f00000000f"
	srv, imp := mockPowerstoreAPI(t, map[string]string{
		"wwn=eq." + naa: "my-vvol-volume",
	})
	defer srv.Close()

	backing := &resolver.DiskBacking{VVolID: naa}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations[annotationKey] != "my-vvol-volume" {
		t.Errorf("unexpected annotation value: %v", annotations)
	}
}

func TestResolveVVolNotFound(t *testing.T) {
	srv, imp := mockPowerstoreAPI(t, map[string]string{})
	defer srv.Close()

	backing := &resolver.DiskBacking{VVolID: "vvol:no-such-uuid"}
	_, err := imp.Resolve(backing)
	if err == nil {
		t.Fatal("expected error for unknown VVol, got nil")
	}
}

func TestResolveRDM(t *testing.T) {
	naa := "naa.68ccf098000a2415fa5c15f00000000f"
	srv, imp := mockPowerstoreAPI(t, map[string]string{
		"wwn=eq." + naa: "tshefi-ecosystem-vmware",
	})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: naa}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations[annotationKey] != "tshefi-ecosystem-vmware" {
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
			name:       "bare device name",
			deviceName: "SOME-DEVICE",
			want:       "some-device",
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
