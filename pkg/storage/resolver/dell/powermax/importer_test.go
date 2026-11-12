package powermax

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/storage/resolver"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
		imp, err := NewPowermaxImporter(tt.input, "user", "pass", testSymID, true, nil, "")
		if err != nil {
			t.Fatalf("NewPowermaxImporter(%q) unexpected error: %v", tt.input, err)
		}
		if imp.baseURL != tt.want {
			t.Errorf("NewPowermaxImporter(%q) baseURL = %q, want %q", tt.input, imp.baseURL, tt.want)
		}
	}

	if _, err := NewPowermaxImporter("", "user", "pass", testSymID, true, nil, ""); err == nil {
		t.Error("expected error for empty host, got nil")
	}
	if _, err := NewPowermaxImporter("https://host", "user", "pass", "", true, nil, ""); err == nil {
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

	backing := &resolver.DiskBacking{IsRDM: true, DeviceName: "naa.60000970000197900046533030339999"}
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
	if _, err := NewPowermaxImporter("http://host:8443", "user", "pass", testSymID, true, nil, ""); err == nil {
		t.Error("expected error for http endpoint, got nil")
	}
}

// fakeArray is a minimal Unisphere sloprovisioning model for Prepare tests.
type fakeArray struct {
	sgs       map[string][]string // device ID -> storage groups
	mv        map[string][]string // storage group -> masking view names
	parents   map[string][]string // storage group -> parent storage groups
	ids       map[string]string   // device ID -> volume identifier
	calls     []string            // "remove:<sg>" / "add:<sg>" in order
	failAddSG string              // add to this storage group returns 500
}

func importClass(groups string) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "powermax-import"},
		Parameters: map[string]string{"importSourceStorageGroups": groups},
	}
}

func newFakeArray(t *testing.T, a *fakeArray) (*httptest.Server, *PowermaxImporter) {
	t.Helper()
	base := "/univmax/restapi/100/sloprovisioning/symmetrix/" + testSymID
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "test" || p != "test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, base)
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(path, "/volume/"):
			dev := strings.TrimPrefix(path, "/volume/")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"storageGroupId": a.sgs[dev], "volume_identifier": a.ids[dev]})
		case r.Method == http.MethodGet && strings.HasPrefix(path, "/storagegroup/"):
			sg := strings.TrimPrefix(path, "/storagegroup/")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"maskingview": a.mv[sg], "parent_storage_group": a.parents[sg]})
		case r.Method == http.MethodPut && strings.HasPrefix(path, "/storagegroup/"):
			sg := strings.TrimPrefix(path, "/storagegroup/")
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Edit struct {
					Remove *struct {
						IDs []string `json:"volumeId"`
					} `json:"removeVolumeParam"`
					Expand *struct {
						Add *struct {
							IDs []string `json:"volumeId"`
						} `json:"addVolumeParam"`
					} `json:"expandStorageGroupParam"`
				} `json:"editStorageGroupActionParam"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if req.Edit.Remove != nil {
				a.calls = append(a.calls, "remove:"+sg)
				for _, dev := range req.Edit.Remove.IDs {
					a.sgs[dev] = removeString(a.sgs[dev], sg)
				}
			}
			if req.Edit.Expand != nil && req.Edit.Expand.Add != nil {
				a.calls = append(a.calls, "add:"+sg)
				if sg == a.failAddSG {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				for _, dev := range req.Edit.Expand.Add.IDs {
					a.sgs[dev] = append(a.sgs[dev], sg)
				}
			}
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	var k8s client.Client = fake.NewClientBuilder().WithObjects(importClass("stage-SG")).Build()
	imp := &PowermaxImporter{baseURL: srv.URL, user: "test", pass: "test", symmetrixID: testSymID, storageClass: "powermax-import", k8sClient: k8s, client: srv.Client()}
	return srv, imp
}

func removeString(list []string, v string) []string {
	var out []string
	for _, x := range list {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

var rdm = &resolver.DiskBacking{IsRDM: true, DeviceName: "naa.x"}

func rdmAnnotation() map[string]string {
	return map[string]string{annotationKey: testSymID + "/003EE"}
}

func TestPrepareMovesMappedDeviceToStaging(t *testing.T) {
	a := &fakeArray{sgs: map[string][]string{"003EE": {"esxi-SG"}}, mv: map[string][]string{"esxi-SG": {"esxi-MV"}}}
	_, imp := newFakeArray(t, a)
	if err := imp.Prepare(rdm, rdmAnnotation()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Join(a.calls, ",") != "remove:esxi-SG,add:stage-SG" {
		t.Errorf("unexpected array calls: %v", a.calls)
	}
	if len(a.sgs["003EE"]) != 1 || a.sgs["003EE"][0] != "stage-SG" {
		t.Errorf("device not only in staging: %v", a.sgs["003EE"])
	}
}

func TestPrepareCascadedMaskingViewOnParent(t *testing.T) {
	a := &fakeArray{
		sgs:     map[string][]string{"003EE": {"esxi-child-SG"}},
		mv:      map[string][]string{"esxi-parent-SG": {"esxi-MV"}},
		parents: map[string][]string{"esxi-child-SG": {"esxi-parent-SG"}},
	}
	_, imp := newFakeArray(t, a)
	if err := imp.Prepare(rdm, rdmAnnotation()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Join(a.calls, ",") != "remove:esxi-child-SG,add:stage-SG" {
		t.Errorf("unexpected array calls: %v", a.calls)
	}
}

func TestPrepareSkipsAdoptedDevice(t *testing.T) {
	a := &fakeArray{
		sgs: map[string][]string{"003EE": {"stage-SG", "csi-node-SG"}},
		mv:  map[string][]string{"csi-node-SG": {"csi-node-MV"}},
		ids: map[string]string{"003EE": "csi-ABC-imp_-pvc-ns"},
	}
	_, imp := newFakeArray(t, a)
	if err := imp.Prepare(rdm, rdmAnnotation()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(a.calls) != 0 {
		t.Errorf("an adopted device must not be touched, got %v", a.calls)
	}
}

func TestPrepareRefusesSharedDevice(t *testing.T) {
	a := &fakeArray{
		sgs: map[string][]string{"003EE": {"host1-SG", "host2-SG"}},
		mv:  map[string][]string{"host1-SG": {"host1-MV"}, "host2-SG": {"host2-MV"}},
	}
	_, imp := newFakeArray(t, a)
	err := imp.Prepare(rdm, rdmAnnotation())
	if err == nil || !strings.Contains(err.Error(), "shared") {
		t.Fatalf("expected shared-device error, got %v", err)
	}
	if len(a.calls) != 0 {
		t.Errorf("array changed for a shared device: %v", a.calls)
	}
}

func TestPrepareRefusesMaskedStagingGroup(t *testing.T) {
	a := &fakeArray{sgs: map[string][]string{"003EE": {"stage-SG"}}, mv: map[string][]string{"stage-SG": {"some-MV"}}}
	_, imp := newFakeArray(t, a)
	if err := imp.Prepare(rdm, rdmAnnotation()); err == nil {
		t.Fatal("expected error for a masked staging group")
	}
	if len(a.calls) != 0 {
		t.Errorf("array changed: %v", a.calls)
	}
}

func TestPrepareIsIdempotentWhenAlreadyStaged(t *testing.T) {
	a := &fakeArray{sgs: map[string][]string{"003EE": {"stage-SG"}}}
	_, imp := newFakeArray(t, a)
	if err := imp.Prepare(rdm, rdmAnnotation()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(a.calls) != 0 {
		t.Errorf("expected no array changes, got %v", a.calls)
	}
}

func TestPrepareAddsUnmappedDeviceToStaging(t *testing.T) {
	a := &fakeArray{sgs: map[string][]string{"003EE": {}}}
	_, imp := newFakeArray(t, a)
	if err := imp.Prepare(rdm, rdmAnnotation()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Join(a.calls, ",") != "add:stage-SG" {
		t.Errorf("unexpected array calls: %v", a.calls)
	}
}

func TestPrepareRollsBackWhenStagingAddFails(t *testing.T) {
	a := &fakeArray{sgs: map[string][]string{"003EE": {"esxi-SG"}}, mv: map[string][]string{"esxi-SG": {"esxi-MV"}}, failAddSG: "stage-SG"}
	_, imp := newFakeArray(t, a)
	err := imp.Prepare(rdm, rdmAnnotation())
	if err == nil || !strings.Contains(err.Error(), "manual recovery") {
		t.Fatalf("expected staging failure with recovery message, got %v", err)
	}
	if len(a.sgs["003EE"]) != 1 || a.sgs["003EE"][0] != "esxi-SG" {
		t.Errorf("device not restored to esxi-SG after failed add: %v", a.sgs["003EE"])
	}
}

func TestPrepareStagingGroupFromStorageClass(t *testing.T) {
	for name, tc := range map[string]struct {
		class   *storagev1.StorageClass
		sc      string
		want    string
		wantErr bool
	}{
		"one group":      {class: importClass("stage-SG"), sc: "powermax-import", want: "stage-SG"},
		"spaces trimmed": {class: importClass(" stage-SG "), sc: "powermax-import", want: "stage-SG"},
		"empty":          {class: importClass(""), sc: "powermax-import", wantErr: true},
		"several groups": {class: importClass("a-SG,b-SG"), sc: "powermax-import", wantErr: true},
		"missing class":  {sc: "powermax-import", wantErr: true},
		"no class name":  {class: importClass("stage-SG"), wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			b := fake.NewClientBuilder()
			if tc.class != nil {
				b = b.WithObjects(tc.class)
			}
			imp := &PowermaxImporter{k8sClient: b.Build(), storageClass: tc.sc}
			got, err := imp.stagingStorageGroup()
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got %q, %v; want %q, wantErr %v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestPrepareRejectsWrongAnnotation(t *testing.T) {
	a := &fakeArray{sgs: map[string][]string{}}
	_, imp := newFakeArray(t, a)
	if err := imp.Prepare(rdm, map[string]string{annotationKey: "000000000000/003EE"}); err == nil {
		t.Fatal("expected error for annotation on another array")
	}
}

func TestPrepareSkipsVVol(t *testing.T) {
	a := &fakeArray{sgs: map[string][]string{}}
	_, imp := newFakeArray(t, a)
	if err := imp.Prepare(&resolver.DiskBacking{VVolID: "vvol:x"}, nil); err != nil {
		t.Fatalf("unexpected error for VVol: %v", err)
	}
	if len(a.calls) != 0 {
		t.Errorf("expected no array changes for VVol, got %v", a.calls)
	}
}
