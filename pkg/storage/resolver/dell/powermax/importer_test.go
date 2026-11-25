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
func mockUnisphereAPI(t *testing.T, volumesByWWN map[string]string, effective ...map[string]string) (*httptest.Server, *PowermaxImporter) {
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
		if q := r.URL.Query().Get("effective_wwn"); q != "" && len(effective) > 0 {
			id, ok = effective[0][q]
		}
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
	if annotations[annotationKey] != testSymID+"/003EE" {
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
	if annotations[annotationKey] != testSymID+"/003EF" {
		t.Errorf("unexpected annotation value: %v", annotations)
	}
}

func TestResolveRDMNotFound(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60000970000197900046533030334646"}
	_, err := imp.Resolve(backing)
	if err == nil {
		t.Fatal("expected error for unknown RDM, got nil")
	}
	if !strings.Contains(err.Error(), "found 0") {
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

func TestResolveVVolFallsThrough(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{})
	defer srv.Close()

	annotations, err := imp.Resolve(&resolver.DiskBacking{VVolID: "vvol:e8307953-a6a3-4adb-bce1-098c210ca53c"})
	if err != nil || annotations != nil {
		t.Fatalf("expected (nil, nil) for VVol, got %v, %v", annotations, err)
	}
}

func TestResolveLunUuidFallback(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{
		"60000970000197900046533030334545": "003EE",
	})
	defer srv.Close()

	annotations, err := imp.Resolve(&resolver.DiskBacking{IsRDM: true, LunUuid: "naa.60000970000197900046533030334545"})
	if err != nil || annotations[annotationKey] != testSymID+"/003EE" {
		t.Fatalf("unexpected result %v, %v", annotations, err)
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

func TestResolveRealWWN(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{"60000970000197900046533030324538": "002E8"})
	defer srv.Close()

	annotations, err := imp.Resolve(&resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60000970000197900046533030324538", CompatibilityMode: "physicalMode"})
	if err != nil || annotations[annotationKey] != testSymID+"/002E8" {
		t.Fatalf("unexpected result %v, %v", annotations, err)
	}
}

func TestResolveEffectiveWWN(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{}, map[string]string{"60000970000197900046533030334545": "003EE"})
	defer srv.Close()

	annotations, err := imp.Resolve(&resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60000970000197900046533030334545"})
	if err != nil || annotations[annotationKey] != testSymID+"/003EE" {
		t.Fatalf("unexpected result %v, %v", annotations, err)
	}
}

func TestResolveNonEMCFallsThrough(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{})
	defer srv.Close()

	annotations, err := imp.Resolve(&resolver.DiskBacking{IsRDM: true, DeviceName: "naa.50060160abcdef0123456789abcdef01"})
	if err != nil || annotations != nil {
		t.Fatalf("expected (nil, nil) for non-EMC NAA, got %v, %v", annotations, err)
	}
}

func TestResolveMalformedNAA(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{})
	defer srv.Close()

	for _, name := range []string{"naa.invalid", "naa.60000970000197900046543030334545"} {
		if _, err := imp.Resolve(&resolver.DiskBacking{IsRDM: true, DeviceName: name}); err == nil || !strings.Contains(err.Error(), "malformed NAA") {
			t.Errorf("%s: expected malformed NAA error, got %v", name, err)
		}
	}
}

func TestResolveVirtualModeRDM(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{"60000970000197900046533030334545": "003EE"})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60000970000197900046533030334545", CompatibilityMode: "virtualMode"}
	if annotations, err := imp.Resolve(backing); err != nil || annotations != nil {
		t.Fatalf("expected (nil, nil) for virtual-mode RDM, got %v, %v", annotations, err)
	}
	backing.VMHasSnapshots = true
	if _, err := imp.Resolve(backing); err == nil || !strings.Contains(err.Error(), "snapshots") {
		t.Fatalf("expected snapshot error for virtual-mode RDM, got %v", err)
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
			name:       "naa with device path",
			deviceName: "/vmfs/devices/disks/naa.60000970000197900046533030334545",
			want:       "60000970000197900046533030334545",
		},
		{
			name:       "vml with device path",
			deviceName: "/vmfs/devices/disks/vml.020001000060000970000197900046533030334545535946202020",
			want:       "60000970000197900046533030334545",
		},
		{
			name:       "bare LUN UUID",
			deviceName: "020001000060000970000197900046533030334545535946202020",
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

func TestNewPowermaxImporterRejectsHTTP(t *testing.T) {
	if _, err := NewPowermaxImporter("http://host:8443", "user", "pass", testSymID, true); err == nil {
		t.Error("expected error for http endpoint, got nil")
	}
}

func TestResolveInvalidDevice(t *testing.T) {
	srv, imp := mockUnisphereAPI(t, map[string]string{})
	defer srv.Close()

	for _, name := range []string{"vml.0200", "naa.60000970000197900046533030334547"} {
		if _, err := imp.Resolve(&resolver.DiskBacking{IsRDM: true, DeviceName: name}); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestCallErrors(t *testing.T) {
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
			_, _ = w.Write([]byte("{"))
		}, "", "failed to read"},
		{"bad json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("not json")) }, "", "failed to parse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imp := &PowermaxImporter{baseURL: tt.baseURL, symmetrixID: testSymID, client: http.DefaultClient}
			if tt.handler != nil {
				srv := httptest.NewServer(tt.handler)
				defer srv.Close()
				imp.baseURL = srv.URL
			}
			_, err := imp.Resolve(&resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60000970000197900046533030334545"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}
