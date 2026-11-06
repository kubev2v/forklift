package powermax

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/storage/resolver"
)

const testSymID = "000197900046"

// mockUnisphereAPI creates a test server that handles Unisphere volume list queries.
// volumesByWWN maps a WWN query value to a volume ID.
func mockUnisphereAPI(t *testing.T, volumesByWWN map[string]string) (*httptest.Server, *PowermaxImporter) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "test" || pass != "test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != "/univmax/restapi/100/sloprovisioning/symmetrix/"+testSymID+"/volume" {
			http.NotFound(w, r)
			return
		}
		id, ok := volumesByWWN[r.URL.Query().Get("wwn")]
		if !ok {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"count": 0})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"count": 1,
			"resultList": map[string]interface{}{
				"result": []map[string]string{{"volumeId": id}},
			},
		})
	}))

	imp := &PowermaxImporter{
		baseURL:     srv.URL,
		user:        "test",
		pass:        "test",
		symmetrixID: testSymID,
		client:      srv.Client(),
	}
	return srv, imp
}

func TestNewPowermaxImporter(t *testing.T) {
	validTests := []struct {
		input string
		want  string
	}{
		{"https://host", "https://host"},
		{"https://host:8443", "https://host:8443"},
		{"https://10.46.2.10/", "https://10.46.2.10"},
		{"10.46.2.10", "https://10.46.2.10"},
		{"host:8443", "https://host:8443"},
	}
	for _, tt := range validTests {
		imp, err := NewPowermaxImporter(tt.input, "user", "pass", testSymID, true)
		if err != nil {
			t.Fatalf("NewPowermaxImporter(%q) unexpected error: %v", tt.input, err)
		}
		if imp.baseURL != tt.want {
			t.Errorf("NewPowermaxImporter(%q) baseURL = %q, want %q", tt.input, imp.baseURL, tt.want)
		}
	}

	if _, err := NewPowermaxImporter("", "user", "pass", testSymID, true); err == nil {
		t.Error("expected error for empty host, got nil")
	}
	if _, err := NewPowermaxImporter("https://host", "user", "pass", "", true); err == nil {
		t.Error("expected error for missing symmetrix ID, got nil")
	}
}

func TestResolveRDM(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{
		"60000970000197900046533030334545": "003EE",
	})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60000970000197900046533030334545"}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations[annotationKey] != "003EE" {
		t.Errorf("unexpected annotation value: %v", annotations)
	}
}

func TestResolveRDMFromVML(t *testing.T) {
	// VML format from vSphere — WWN at hex[10:42]
	vml := "vml.020001000060000970000197900046533030334545535946202020"
	srv, imp := mockUnisphereAPI(t, map[string]string{
		"60000970000197900046533030334545": "003EF",
	})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: vml}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations[annotationKey] != "003EF" {
		t.Errorf("unexpected annotation value: %v", annotations)
	}
}

func TestResolveRDMNotFound(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60000970000197900046533030339999"}
	_, err := imp.Resolve(backing)
	if err == nil {
		t.Fatal("expected error for unknown RDM, got nil")
	}
	if !strings.Contains(err.Error(), "no PowerMax volume found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveRDMAPIError(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{})
	defer srv.Close()
	imp.pass = "wrong"

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60000970000197900046533030334545"}
	_, err := imp.Resolve(backing)
	if err == nil {
		t.Fatal("expected error for unauthorized request, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveVVol(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{})
	defer srv.Close()

	backing := &resolver.DiskBacking{VVolID: "vvol:e8307953-a6a3-4adb-bce1-098c210ca53c"}
	_, err := imp.Resolve(backing)
	if err == nil {
		t.Fatal("expected error for VVol, got nil")
	}
	if !strings.Contains(err.Error(), "supports only RDM") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveVMDK(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{})
	defer srv.Close()

	backing := &resolver.DiskBacking{DeviceName: "[ds] vm/vm.vmdk"}
	_, err := imp.Resolve(backing)
	if err == nil {
		t.Fatal("expected error for VMDK, got nil")
	}
	if !strings.Contains(err.Error(), "supports only RDM") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveNilBacking(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{})
	defer srv.Close()

	if _, err := imp.Resolve(nil); err == nil {
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
			deviceName: "naa.60000970000197900046533030334545",
			want:       "60000970000197900046533030334545",
		},
		{
			name:       "naa lowercase",
			deviceName: "naa.6000097000019790004653303033454f",
			want:       "6000097000019790004653303033454F",
		},
		{
			name:       "vml format",
			deviceName: "vml.020001000060000970000197900046533030334545535946202020",
			want:       "60000970000197900046533030334545",
		},
		{
			name:       "vml too short",
			deviceName: "vml.0200010000",
			wantErr:    true,
		},
		{
			name:       "vml without NAA type 6",
			deviceName: "vml.02000100005000097000019790004653303033454553594620202",
			wantErr:    true,
		},
		{
			name:       "bare device name",
			deviceName: "some-device",
			want:       "SOME-DEVICE",
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
