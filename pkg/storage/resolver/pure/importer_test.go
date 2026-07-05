package pure

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/storage/resolver"
)

// mockPureAPI creates a test server that handles Pure FlashArray REST API endpoints.
// volumesByTag maps a VVol UUID to a volume name (for tags API).
// volumesBySerial maps an uppercase serial to a volume name (for volumes API).
func mockPureAPI(t *testing.T, volumesByTag map[string]string, volumesBySerial map[string]string) (*httptest.Server, *PureImporter) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/api_version":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"version": []string{"1.19", "2.0", "2.28"},
			})

		case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/1.") && strings.HasSuffix(r.URL.Path, "/auth/apitoken"):
			_ = json.NewEncoder(w).Encode(map[string]string{"api_token": "test-api-token"})

		case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/2.") && strings.HasSuffix(r.URL.Path, "/login"):
			w.Header().Set("x-auth-token", "test-auth-token")
			w.WriteHeader(http.StatusOK)

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/volumes/tags"):
			filter := r.URL.Query().Get("filter")
			var name string
			var found bool
			for uuid, n := range volumesByTag {
				if strings.Contains(filter, uuid) {
					name = n
					found = true
					break
				}
			}
			if !found {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{
					{"resource": map[string]string{"name": name}},
				},
			})

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/volumes"):
			filter := r.URL.Query().Get("filter")
			var name string
			var found bool
			for serial, n := range volumesBySerial {
				if strings.Contains(filter, serial) {
					name = n
					found = true
					break
				}
			}
			if !found {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]interface{}{
					{"name": name},
				},
			})

		default:
			http.NotFound(w, r)
		}
	}))

	host := strings.TrimPrefix(srv.URL, "http://")
	rc := &RestClient{
		hostname:   host,
		scheme:     "http",
		httpClient: srv.Client(),
		apiV1:      "1.19",
		apiV2:      "2.28",
		authToken:  "test-auth-token",
	}
	imp := &PureImporter{client: rc}
	return srv, imp
}

func TestResolveVVol(t *testing.T) {
	uuid := "e8307953-a6a3-4adb-bce1-098c210ca53c"
	srv, imp := mockPureAPI(t, map[string]string{uuid: "my-pure-volume"}, nil)
	defer srv.Close()

	// Override hostname to use http for test server
	backing := &resolver.DiskBacking{VVolID: "vvol:" + uuid}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations[annotationKey] != "my-pure-volume" {
		t.Errorf("expected annotation value 'my-pure-volume', got: %v", annotations)
	}
}

func TestResolveVVolAnnotationKey(t *testing.T) {
	uuid := "e8307953-a6a3-4adb-bce1-098c210ca53c"
	srv, imp := mockPureAPI(t, map[string]string{uuid: "my-volume"}, nil)
	defer srv.Close()

	backing := &resolver.DiskBacking{VVolID: "vvol:" + uuid}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := annotations["portworx.io/pure-volume-name"]; !ok {
		t.Errorf("expected annotation key portworx.io/pure-volume-name, got: %v", annotations)
	}
}

func TestResolveVVolNotFound(t *testing.T) {
	srv, imp := mockPureAPI(t, map[string]string{}, nil)
	defer srv.Close()

	backing := &resolver.DiskBacking{VVolID: "vvol:no-such-uuid"}
	_, err := imp.Resolve(backing)
	if err == nil {
		t.Fatal("expected error for unknown VVol, got nil")
	}
}

func TestResolveRDM(t *testing.T) {
	serial := "B4DC0FFEE0DDF00D"
	naa := "naa." + flashProviderID + strings.ToLower(serial)
	srv, imp := mockPureAPI(t, nil, map[string]string{serial: "rdm-pure-volume"})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: naa}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations[annotationKey] != "rdm-pure-volume" {
		t.Errorf("expected annotation value 'rdm-pure-volume', got: %v", annotations)
	}
}

func TestResolveVMDK(t *testing.T) {
	srv, imp := mockPureAPI(t, nil, nil)
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

func TestExtractSerialFromNAA(t *testing.T) {
	tests := []struct {
		name    string
		naa     string
		want    string
		wantErr bool
	}{
		{
			name: "valid lowercase",
			naa:  "naa.624a9370b4dc0ffee0ddf00d",
			want: "B4DC0FFEE0DDF00D",
		},
		{
			name: "valid uppercase",
			naa:  "naa.624A9370B4DC0FFEE0DDF00D",
			want: "B4DC0FFEE0DDF00D",
		},
		{
			name:    "without prefix (unsupported)",
			naa:     "624a9370b4dc0ffee0ddf00d",
			wantErr: true,
		},
		{
			name:    "wrong OUI",
			naa:     "naa.60002ac0b4dc0ffee0ddf00d",
			wantErr: true,
		},
		{
			name:    "empty string",
			naa:     "",
			wantErr: true,
		},
		{
			name:    "only OUI no serial",
			naa:     "naa.624a9370",
			wantErr: true,
		},
		// VML format — vSphere encodes the NAA identifier inside a VML string
		{
			name: "vml format",
			naa:  "vml.020000600000624a9370a7b9f7ecc01e40f70002c43e466c61736841",
			want: "A7B9F7ECC01E40F70002C43E",
		},
		{
			name:    "vml format wrong OUI",
			naa:     "vml.020000600000600a098000000000000000000000000000",
			wantErr: true,
		},
		{
			name:    "vml format serial too short",
			naa:     "vml.020000600000624a9370aabbcc",
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

func TestResolveRDMFromVML(t *testing.T) {
	serial := "A7B9F7ECC01E40F70002C43E"
	vml := "vml.020000600000624a9370a7b9f7ecc01e40f70002c43e466c61736841"
	srv, imp := mockPureAPI(t, nil, map[string]string{serial: "amit-rdm-test-2"})
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: vml}
	annotations, err := imp.Resolve(backing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations[annotationKey] != "amit-rdm-test-2" {
		t.Errorf("expected 'amit-rdm-test-2', got: %v", annotations)
	}
}

func TestResolveRDMBadNAA(t *testing.T) {
	srv, imp := mockPureAPI(t, nil, nil)
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60002ac0badbadserial"}
	_, err := imp.Resolve(backing)
	if err == nil {
		t.Fatal("expected error for non-Pure NAA, got nil")
	}
	if !strings.Contains(err.Error(), "Pure OUI") {
		t.Errorf("expected OUI error, got: %v", err)
	}
}
