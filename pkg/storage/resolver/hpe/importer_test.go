package hpe

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/storage/resolver"
)

// mockWSAPI creates a test server that handles HPE WSAPI auth and volume queries.
// volumesByFilter maps a query filter value (e.g. `"wwn EQ ABCD"`) to volume names.
func mockWSAPI(t *testing.T, volumesByFilter map[string]string) (*httptest.Server, *HpeImporter) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/credentials":
			_ = json.NewEncoder(w).Encode(map[string]string{"key": "test-session-key"})

		case r.Method == "GET" && r.URL.Path == "/api/v1/volumes":
			filter := r.URL.Query().Get("query")
			name, ok := volumesByFilter[filter]
			if !ok {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"total": 0, "members": []interface{}{}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"total":   1,
				"members": []map[string]string{{"name": name}},
			})

		default:
			http.NotFound(w, r)
		}
	}))

	imp := &HpeImporter{
		baseURL: srv.URL,
		user:    "test",
		pass:    "test",
		client:  srv.Client(),
	}
	return srv, imp
}

func TestNewHpeImporter(t *testing.T) {
	validTests := []struct {
		input string
		want  string
	}{
		{"https://host:8080", "https://host:8080"},
		{"http://host:8080", "http://host:8080"},
		{"https://10.46.2.10:8080", "https://10.46.2.10:8080"},
		{"https://10.46.2.10:8080/", "https://10.46.2.10:8080"},
	}
	for _, tt := range validTests {
		imp, err := NewHpeImporter(tt.input, "user", "pass", true)
		if err != nil {
			t.Fatalf("NewHpeImporter(%q) unexpected error: %v", tt.input, err)
		}
		if imp.baseURL != tt.want {
			t.Errorf("NewHpeImporter(%q) baseURL = %q, want %q", tt.input, imp.baseURL, tt.want)
		}
	}

	invalidTests := []string{
		"10.46.2.10",
		"host:8080",
		"",
	}
	for _, input := range invalidTests {
		_, err := NewHpeImporter(input, "user", "pass", true)
		if err == nil {
			t.Errorf("NewHpeImporter(%q) expected error for missing scheme, got nil", input)
		}
	}
}

func TestResolveAnnotationKey(t *testing.T) {
	uuid := "e8307953-a6a3-4adb-bce1-098c210ca53c"
	srv, imp := mockWSAPI(t, map[string]string{
		`"uuid EQ ` + uuid + `"`: "my-hpe-volume",
	})
	defer srv.Close()

	backing := &resolver.DiskBacking{VVolID: "vvol:" + uuid}
	annotations, found, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if _, ok := annotations[AnnotationKey]; !ok {
		t.Errorf("expected annotation key csi.hpe.com/importVolAsClone, got: %v", annotations)
	}
}

func TestResolveVVol(t *testing.T) {
	uuid := "e8307953-a6a3-4adb-bce1-098c210ca53c"
	srv, imp := mockWSAPI(t, map[string]string{
		`"uuid EQ ` + uuid + `"`: "my-hpe-volume",
	})
	defer srv.Close()

	backing := &resolver.DiskBacking{VVolID: "vvol:" + uuid}
	annotations, found, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if annotations[AnnotationKey] != "my-hpe-volume" {
		t.Errorf("unexpected annotation value: %v", annotations)
	}
}

func TestResolveVVolNotFoundCrossArray(t *testing.T) {
	srv, imp := mockWSAPI(t, map[string]string{})
	defer srv.Close()

	backing := &resolver.DiskBacking{VVolID: "vvol:no-such-uuid"}
	annotations, found, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("expected no error for cross-array (not found), got: %v", err)
	}
	if found {
		t.Fatal("expected found=false for volume not on this array")
	}
	if annotations != nil {
		t.Errorf("expected nil annotations, got: %v", annotations)
	}
}

func TestResolveRDMNotFoundCrossArray(t *testing.T) {
	srv, imp := mockWSAPI(t, map[string]string{})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60002ac0000000000000182d00021f6b"}
	annotations, found, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("expected no error for cross-array (not found), got: %v", err)
	}
	if found {
		t.Fatal("expected found=false for volume not on this array")
	}
	if annotations != nil {
		t.Errorf("expected nil annotations, got: %v", annotations)
	}
}

func TestResolveRDM(t *testing.T) {
	naa := "naa.60002ac0000000000000182d00021f6b"
	wwn := "60002AC0000000000000182D00021F6B"
	srv, imp := mockWSAPI(t, map[string]string{
		`"wwn EQ ` + wwn + `"`: "tshefi-ecosystem-vmware",
	})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: naa}
	annotations, found, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if annotations[AnnotationKey] != "tshefi-ecosystem-vmware" {
		t.Errorf("unexpected annotation value: %v", annotations)
	}
}

func TestResolveRDMFromVML(t *testing.T) {
	vml := "vml.020001000060002ac0000000000000186b00021f6b565620202020"
	wwn := "60002AC0000000000000186B00021F6B"
	srv, imp := mockWSAPI(t, map[string]string{
		`"wwn EQ ` + wwn + `"`: "tshefi-vml-volume",
	})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: vml}
	annotations, found, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if annotations[AnnotationKey] != "tshefi-vml-volume" {
		t.Errorf("unexpected annotation value: %v", annotations)
	}
}

func TestResolveVMDK(t *testing.T) {
	srv, imp := mockWSAPI(t, map[string]string{})
	defer srv.Close()

	backing := &resolver.DiskBacking{DeviceName: "[ds] vm/vm.vmdk"}
	_, _, err := imp.Resolve(backing)
	if err == nil {
		t.Fatal("expected error for VMDK, got nil")
	}
	if !strings.Contains(err.Error(), "does not support VMDK") {
		t.Errorf("unexpected error message: %v", err)
	}
}
