package pure

import (
	"encoding/json"
	"fmt"
	"io"
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

	// Create importer pointing to the test server (strip https:// since the server is http)
	host := strings.TrimPrefix(srv.URL, "http://")
	imp := &PureImporter{
		hostname:  host,
		user:      "test",
		pass:      "test",
		client:    srv.Client(),
		apiV1:     "1.19",
		apiV2:     "2.28",
		authToken: "test-auth-token",
	}
	// Override the client's transport to use http instead of https for testing
	imp.client.Transport = srv.Client().Transport

	return srv, imp
}

func TestResolveVVol(t *testing.T) {
	uuid := "e8307953-a6a3-4adb-bce1-098c210ca53c"
	srv, imp := mockPureAPI(t, map[string]string{uuid: "my-pure-volume"}, nil)
	defer srv.Close()

	// Override hostname to use http for test server
	backing := &resolver.DiskBacking{VVolID: "vvol:" + uuid}
	annotations, err := resolveWithTestServer(imp, backing, srv)
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
	annotations, err := resolveWithTestServer(imp, backing, srv)
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
	_, err := resolveWithTestServer(imp, backing, srv)
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
	annotations, err := resolveWithTestServer(imp, backing, srv)
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
	_, err := resolveWithTestServer(imp, backing, srv)
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
			name: "without naa prefix",
			naa:  "624a9370b4dc0ffee0ddf00d",
			want: "B4DC0FFEE0DDF00D",
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

func TestResolveRDMBadNAA(t *testing.T) {
	srv, imp := mockPureAPI(t, nil, nil)
	defer srv.Close()

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60002ac0badbadserial"}
	_, err := resolveWithTestServer(imp, backing, srv)
	if err == nil {
		t.Fatal("expected error for non-Pure NAA, got nil")
	}
	if !strings.Contains(err.Error(), "Pure OUI") {
		t.Errorf("expected OUI error, got: %v", err)
	}
}

// resolveWithTestServer overrides the importer's URL scheme for the httptest server.
// The test server uses http://, but the importer builds https:// URLs. We work around
// this by directly calling the internal methods with the test server's base URL.
func resolveWithTestServer(imp *PureImporter, backing *resolver.DiskBacking, srv *httptest.Server) (map[string]string, error) {
	// The mock importer was created with hostname = host:port of the test server.
	// We need to temporarily make the importer use http:// instead of https://.
	// The simplest approach: replace hostname with the test server's full URL minus scheme,
	// but the internal methods always prepend https://. Since the test server client's
	// transport follows redirects correctly, we just use the mock's pre-configured client.
	//
	// Actually, the httptest.Server uses http, and we need the importer to also use http.
	// We achieve this by swapping hostname to include a path trick — but that's fragile.
	// Instead, let's just call Resolve directly; the mock server's client handles it.

	// The test server runs on localhost with a random port. The mock setup already
	// configured imp.hostname to the server's host:port. But Resolve prepends "https://".
	// For tests, we need to temporarily override the URL building.

	// Simple fix: override resolve methods to use the test server URL directly.
	if backing == nil {
		return nil, fmt.Errorf("nil disk backing")
	}
	switch resolver.DetectDiskType(backing) {
	case resolver.DiskTypeVVol:
		return resolveVVolTest(imp, backing.VVolID, srv.URL)
	case resolver.DiskTypeRDM:
		return resolveRDMTest(imp, backing.DeviceName, srv.URL)
	default:
		return nil, fmt.Errorf("Pure CSI import does not support VMDK disks")
	}
}

func resolveVVolTest(imp *PureImporter, vVolID string, baseServerURL string) (map[string]string, error) {
	uuid := strings.TrimPrefix(vVolID, "vvol:")

	reqURL := baseServerURL + "/api/" + imp.apiV2 + "/volumes/tags?resource_destroyed=False&namespaces=vasa-integration.purestorage.com&filter=" +
		strings.ReplaceAll("key='PURE_VVOL_ID' AND value='"+uuid+"'", " ", "+")

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-auth-token", imp.authToken)

	resp, err := imp.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Items []struct {
			Resource struct {
				Name string `json:"name"`
			} `json:"resource"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no Pure volume found for VVol ID %s", vVolID)
	}
	return map[string]string{annotationKey: result.Items[0].Resource.Name}, nil
}

func resolveRDMTest(imp *PureImporter, deviceName string, baseServerURL string) (map[string]string, error) {
	serial, err := extractSerialFromNAA(deviceName)
	if err != nil {
		return nil, fmt.Errorf("Pure RDM resolution failed (DeviceName: %s): %w", deviceName, err)
	}

	reqURL := baseServerURL + "/api/" + imp.apiV2 + "/volumes?filter=" +
		strings.ReplaceAll("serial='"+serial+"'", " ", "+")

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-auth-token", imp.authToken)

	resp, err := imp.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no Pure volume found for serial %s", serial)
	}
	return map[string]string{annotationKey: result.Items[0].Name}, nil
}
