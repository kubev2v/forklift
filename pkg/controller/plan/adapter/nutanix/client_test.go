package nutanix

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	webbase "github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/nutanix"
	libclient "github.com/kubev2v/forklift/pkg/lib/client/nutanix"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// errNotImplemented is returned by fakeInventory methods that aren't
// exercised by these tests, so callers get a clear error instead of a
// silent nil/nil.
var errNotImplemented = fmt.Errorf("not implemented by fakeInventory")

// fakeInventory is a minimal web.Client stub that only implements Find,
// returning a fixed VM. The other methods aren't exercised by these tests.
type fakeInventory struct {
	vm *model.VM
}

func (f *fakeInventory) Finder() webbase.Finder { return nil }
func (f *fakeInventory) Get(_ interface{}, _ string) error {
	return nil
}
func (f *fakeInventory) List(_ interface{}, _ ...webbase.Param) error {
	return nil
}
func (f *fakeInventory) Watch(_ interface{}, _ webbase.EventHandler) (*webbase.Watch, error) {
	return nil, errNotImplemented
}
func (f *fakeInventory) Find(resource interface{}, _ webbase.Ref) error {
	if vm, ok := resource.(*model.VM); ok {
		*vm = *f.vm
	}
	return nil
}
func (f *fakeInventory) VM(_ *webbase.Ref) (interface{}, error) { return f.vm, nil }
func (f *fakeInventory) Workload(_ *webbase.Ref) (interface{}, error) {
	return nil, errNotImplemented
}
func (f *fakeInventory) Network(_ *webbase.Ref) (interface{}, error) {
	return nil, errNotImplemented
}
func (f *fakeInventory) Storage(_ *webbase.Ref) (interface{}, error) {
	return nil, errNotImplemented
}
func (f *fakeInventory) Host(_ *webbase.Ref) (interface{}, error) {
	return nil, errNotImplemented
}

type v3ImageListResponse struct {
	Entities []libclient.V3Image `json:"entities"`
	Metadata struct {
		TotalMatches int `json:"total_matches"`
	} `json:"metadata"`
}

type v3ImageCreateRequest struct {
	Spec struct {
		Name      string `json:"name"`
		Resources struct {
			DataSourceReference struct {
				Kind string `json:"kind"`
				UUID string `json:"uuid"`
			} `json:"data_source_reference"`
			ImageType string `json:"image_type"`
		} `json:"resources"`
	} `json:"spec"`
}

const testMigrationUID = "7b33cda8-a846-4130-a6ec-f176a511a4e9"

func testMigrationImageName(vmRef ref.Ref, diskUUID string) string {
	return migrationImageName(testMigrationUID, vmRef, diskUUID)
}

func newTestClient(url string) *Client {
	return newTestClientWithInventory(url, nil)
}

func newTestClientWithInventory(url string, vm *model.VM) *Client {
	return &Client{
		Context: &plancontext.Context{
			Migration: &api.Migration{
				ObjectMeta: meta.ObjectMeta{
					UID: testMigrationUID,
				},
			},
			Source: plancontext.Source{
				Provider: &api.Provider{Spec: api.ProviderSpec{URL: url}},
				Secret: &core.Secret{
					Data: map[string][]byte{
						"user":     []byte("admin"),
						"password": []byte("password"),
					},
				},
				Inventory: &fakeInventory{vm: vm},
			},
			Log: logging.WithName("test"),
		},
	}
}

func testV3Image(uuid, name, state string) libclient.V3Image {
	image := libclient.V3Image{}
	image.Metadata.UUID = uuid
	image.Spec.Name = name
	image.Status.State = state
	return image
}

// TestConnect verifies connect() picks up the provider URL and secret from
// the plan context and authenticates against the Nutanix API.
func TestConnect(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") == "" {
			t.Error("expected Authorization header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"entities":[]}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	if err := client.connect(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests == 0 {
		t.Fatal("expected connect() to issue at least one authenticated request")
	}
	if client.URL != server.URL {
		t.Fatalf("expected client URL to be %s, got %s", server.URL, client.URL)
	}
}

// TestConnect_FailsOnUnauthorized verifies connect() surfaces an error when
// the connectivity probe is rejected.
func TestConnect_FailsOnUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	if err := client.connect(); err == nil {
		t.Fatal("expected an error when the connectivity probe returns 401")
	}
}

// vmEntity builds a minimal v3 VM entity body with the given power state.
func vmEntity(uuid, powerState string) libclient.VM {
	resources := libclient.VMResources{PowerState: powerState}
	return libclient.VM{
		APIVersion: "3.1",
		Metadata:   libclient.Metadata{UUID: uuid},
		Spec: libclient.VMSpec{
			Name:      "test-vm",
			Resources: resources,
		},
		Status: libclient.VMStatus{Resources: resources},
	}
}

func vmEntityPtr(uuid, powerState string) *libclient.VM {
	entity := vmEntity(uuid, powerState)
	return &entity
}

// newPowerTestServer serves the connectivity probe plus GET/PUT for a
// single VM entity, tracking PUT bodies and set_power_state transitions
// for assertions. PUT ON/OFF updates the entity in place; ACPI_SHUTDOWN
// leaves the VM running until an OFF transition is posted.
func newPowerTestServer(
	t *testing.T,
	entity *libclient.VM,
) (server *httptest.Server, puts *[]libclient.VMUpdateRequest, transitions *[]string) {
	t.Helper()
	var putBodies []libclient.VMUpdateRequest
	var transitionBodies []string
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/clusters/list"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/set_power_state"):
			var body struct {
				Transition string `json:"transition"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			transitionBodies = append(transitionBodies, body.Transition)
			if body.Transition == powerStateTransitionOff {
				entity.Spec.Resources.PowerState = powerStateOff
				entity.Status.Resources.PowerState = powerStateOff
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(entity)
		case r.Method == http.MethodPut:
			var body libclient.VMUpdateRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			putBodies = append(putBodies, body)
			entity.Spec.Resources.PowerState = body.Spec.Resources.PowerState
			entity.Status.Resources.PowerState = body.Spec.Resources.PowerState
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	return server, &putBodies, &transitionBodies
}

// newConnectedTestClient builds a plan adapter Client and connects it
// against the given server, as the Adapter would before issuing requests.
func newConnectedTestClient(t *testing.T, url string) *Client {
	t.Helper()
	client := newTestClient(url)
	if err := client.connect(); err != nil {
		t.Fatalf("unexpected error connecting: %v", err)
	}
	return client
}

func TestPowerState(t *testing.T) {
	cases := []struct {
		raw  string
		want planapi.VMPowerState
	}{
		{powerStateOn, planapi.VMPowerStateOn},
		{powerStateOff, planapi.VMPowerStateOff},
		{"", planapi.VMPowerStateUnknown},
	}
	for _, c := range cases {
		server, _, _ := newPowerTestServer(t, vmEntityPtr("uuid-1", c.raw))
		defer server.Close()

		client := newConnectedTestClient(t, server.URL)
		state, err := client.PowerState(ref.Ref{ID: "uuid-1"})
		if err != nil {
			t.Fatalf("unexpected error for raw state %q: %v", c.raw, err)
		}
		if state != c.want {
			t.Fatalf("raw state %q: expected %s, got %s", c.raw, c.want, state)
		}
	}
}

func TestPoweredOff(t *testing.T) {
	server, _, _ := newPowerTestServer(t, vmEntityPtr("uuid-1", powerStateOff))
	defer server.Close()

	client := newConnectedTestClient(t, server.URL)
	off, err := client.PoweredOff(ref.Ref{ID: "uuid-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !off {
		t.Fatal("expected PoweredOff to be true when power_state is OFF")
	}
}

// TestPowerOff_SkipsAcpiWhenAlreadyOff verifies PowerOff is a no-op when the
// VM is already off.
func TestPowerOff_SkipsAcpiWhenAlreadyOff(t *testing.T) {
	server, puts, transitions := newPowerTestServer(t, vmEntityPtr("uuid-1", powerStateOff))
	defer server.Close()

	client := newConnectedTestClient(t, server.URL)
	if err := client.PowerOff(ref.Ref{ID: "uuid-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*puts) != 0 {
		t.Fatalf("expected no PUT requests, got %d", len(*puts))
	}
	if len(*transitions) != 0 {
		t.Fatalf("expected no set_power_state requests, got %d", len(*transitions))
	}
}

// TestPowerOff_SubmitsAcpiShutdown verifies PowerOff requests ACPI_SHUTDOWN
// and leaves the VM running until PoweredOff forces OFF after the grace
// period.
func TestPowerOff_SubmitsAcpiShutdown(t *testing.T) {
	server, puts, transitions := newPowerTestServer(t, vmEntityPtr("uuid-1", powerStateOn))
	defer server.Close()

	client := newConnectedTestClient(t, server.URL)
	if err := client.PowerOff(ref.Ref{ID: "uuid-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*puts) != 0 {
		t.Fatalf("expected no PUT requests, got %d", len(*puts))
	}
	if len(*transitions) != 1 {
		t.Fatalf("expected exactly one set_power_state request, got %d", len(*transitions))
	}
	if (*transitions)[0] != powerStateTransitionAcpiShutdown {
		t.Fatalf(
			"expected transition %q, got %q",
			powerStateTransitionAcpiShutdown,
			(*transitions)[0],
		)
	}

	off, err := client.PoweredOff(ref.Ref{ID: "uuid-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if off {
		t.Fatal("expected PoweredOff to be false while ACPI shutdown is pending")
	}
}

func TestPoweredOff_ForcesHardOffAfterGracePeriod(t *testing.T) {
	oldGrace := powerOffGracePeriod
	powerOffGracePeriod = 0
	t.Cleanup(func() { powerOffGracePeriod = oldGrace })

	server, puts, transitions := newPowerTestServer(t, vmEntityPtr("uuid-1", powerStateOn))
	defer server.Close()

	client := newConnectedTestClient(t, server.URL)
	if err := client.PowerOff(ref.Ref{ID: "uuid-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	off, err := client.PoweredOff(ref.Ref{ID: "uuid-1"})
	if err != nil {
		t.Fatalf("unexpected error on first PoweredOff: %v", err)
	}
	if off {
		t.Fatal("expected PoweredOff to be false before hard off completes")
	}
	if len(*transitions) != 2 {
		t.Fatalf("expected ACPI then OFF transitions, got %v", *transitions)
	}
	if (*transitions)[1] != powerStateTransitionOff {
		t.Fatalf("expected hard-off transition %q, got %q", powerStateTransitionOff, (*transitions)[1])
	}
	if len(*puts) != 0 {
		t.Fatalf("expected no PUT requests, got %d", len(*puts))
	}

	off, err = client.PoweredOff(ref.Ref{ID: "uuid-1"})
	if err != nil {
		t.Fatalf("unexpected error on second PoweredOff: %v", err)
	}
	if !off {
		t.Fatal("expected PoweredOff to be true after hard off")
	}
}

// TestPowerOn_SkipsPutWhenAlreadyOn mirrors TestPowerOff_SkipsPutWhenAlreadyOff.
func TestPowerOn_SkipsPutWhenAlreadyOn(t *testing.T) {
	server, puts, _ := newPowerTestServer(t, vmEntityPtr("uuid-1", powerStateOn))
	defer server.Close()

	client := newConnectedTestClient(t, server.URL)
	if err := client.PowerOn(ref.Ref{ID: "uuid-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*puts) != 0 {
		t.Fatalf("expected no PUT requests, got %d", len(*puts))
	}
}

// newImageTestServer serves the connectivity probe plus a minimal v3 image
// list/create/delete implementation backed by an in-memory store keyed by
// UUID, for testing the catalog image lifecycle used by PreTransferActions
// and Finalize.
func newImageTestServer(t *testing.T, images map[string]libclient.V3Image) *httptest.Server {
	t.Helper()
	nextID := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/clusters/list"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/images/list"):
			entities := make([]libclient.V3Image, 0, len(images))
			for _, image := range images {
				entities = append(entities, image)
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(v3ImageListResponse{
				Entities: entities,
				Metadata: struct {
					TotalMatches int `json:"total_matches"`
				}{TotalMatches: len(entities)},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/images"):
			var body v3ImageCreateRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			nextID++
			uuid := fmt.Sprintf("image-%d", nextID)
			images[uuid] = testV3Image(uuid, body.Spec.Name, imageStatePending)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodDelete:
			delete(images, path.Base(r.URL.Path))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
}

func TestFindImageByName_NotFound(t *testing.T) {
	server := newImageTestServer(t, map[string]libclient.V3Image{})
	defer server.Close()

	client := newConnectedTestClient(t, server.URL)
	_, found, err := client.findImageByName("missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected image not to be found")
	}
}

func TestFindImageByName_Found(t *testing.T) {
	images := map[string]libclient.V3Image{
		"image-1": testV3Image("image-1", testMigrationImageName(ref.Ref{ID: "vm-1"}, "disk-1"), imageStateComplete),
	}
	server := newImageTestServer(t, images)
	defer server.Close()

	client := newConnectedTestClient(t, server.URL)
	entity, found, err := client.findImageByName(testMigrationImageName(ref.Ref{ID: "vm-1"}, "disk-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected image to be found")
	}
	if entity.Metadata.UUID != "image-1" {
		t.Fatalf("expected uuid image-1, got %s", entity.Metadata.UUID)
	}
	if entity.Status.State != imageStateComplete {
		t.Fatalf("expected state %s, got %s", imageStateComplete, entity.Status.State)
	}
}

func TestCreateImage_PostsExpectedBody(t *testing.T) {
	images := map[string]libclient.V3Image{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/clusters/list"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/images"):
			var body v3ImageCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			if body.Spec.Name != "forklift-migration-vm-1-disk-1" {
				t.Fatalf("unexpected image name: %q", body.Spec.Name)
			}
			if body.Spec.Resources.ImageType != "DISK_IMAGE" {
				t.Fatalf("expected image_type DISK_IMAGE, got %q", body.Spec.Resources.ImageType)
			}
			if body.Spec.Resources.DataSourceReference.Kind != "vm_disk" ||
				body.Spec.Resources.DataSourceReference.UUID != "disk-uuid-1" {
				t.Fatalf("unexpected data_source_reference: %+v", body.Spec.Resources.DataSourceReference)
			}
			images["image-1"] = testV3Image("image-1", body.Spec.Name, imageStatePending)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client := newConnectedTestClient(t, server.URL)
	if err := client.createImage("forklift-migration-vm-1-disk-1", "disk-uuid-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected exactly one image to be created, got %d", len(images))
	}
}

// TestPreTransferActions_CreatesImagesAndWaitsForComplete verifies
// PreTransferActions creates one image per non-CDROM disk, reports not
// ready while any image is still pending, and reports ready once every
// image has transitioned to COMPLETE -- without needing to change the VM
// disk list between polls (as the migration controller reconciles).
func TestPreTransferActions_CreatesImagesAndWaitsForComplete(t *testing.T) {
	vm := &model.VM{VM1: model.VM1{
		Disks: []model.Disk{
			{UUID: "disk-1", DiskSizeBytes: 1024},
			{UUID: "cdrom-1", IsCdrom: true},
		},
	}}
	images := map[string]libclient.V3Image{}
	server := newImageTestServer(t, images)
	defer server.Close()

	client := newTestClientWithInventory(server.URL, vm)
	if err := client.connect(); err != nil {
		t.Fatalf("unexpected error connecting: %v", err)
	}

	vmRef := ref.Ref{ID: "vm-1"}
	ready, err := client.PreTransferActions(vmRef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ready {
		t.Fatal("expected ready=false on first call, before the image exists")
	}
	if len(images) != 1 {
		t.Fatalf("expected exactly one image to be created (CDROM should be skipped), got %d", len(images))
	}

	ready, err = client.PreTransferActions(vmRef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ready {
		t.Fatal("expected ready=false while the image is still PENDING")
	}

	for uuid, image := range images {
		image.Status.State = imageStateComplete
		images[uuid] = image
	}

	ready, err = client.PreTransferActions(vmRef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ready {
		t.Fatal("expected ready=true once the image is COMPLETE")
	}
}

// TestPreTransferActions_ErrorsOnImageError verifies a failed image
// creation surfaces as an error rather than looping forever.
func TestPreTransferActions_ErrorsOnImageError(t *testing.T) {
	vm := &model.VM{VM1: model.VM1{Disks: []model.Disk{{UUID: "disk-1"}}}}
	images := map[string]libclient.V3Image{
		"image-1": testV3Image("image-1", testMigrationImageName(ref.Ref{ID: "vm-1"}, "disk-1"), imageStateError),
	}
	server := newImageTestServer(t, images)
	defer server.Close()

	client := newTestClientWithInventory(server.URL, vm)
	if err := client.connect(); err != nil {
		t.Fatalf("unexpected error connecting: %v", err)
	}

	if _, err := client.PreTransferActions(ref.Ref{ID: "vm-1"}); err == nil {
		t.Fatal("expected an error when the catalog image failed to create")
	}
}

// TestFinalize_DeletesImages verifies Finalize deletes each non-CDROM
// disk's catalog image and leaves unrelated images untouched.
func TestFinalize_DeletesImages(t *testing.T) {
	vm := &model.VM{VM1: model.VM1{
		Disks: []model.Disk{
			{UUID: "disk-1"},
			{UUID: "cdrom-1", IsCdrom: true},
		},
	}}
	images := map[string]libclient.V3Image{
		"image-1": testV3Image(
			"image-1",
			migrationImageName(testMigrationUID, ref.Ref{ID: "vm-1"}, "disk-1"),
			imageStateComplete,
		),
		"image-unrelated": testV3Image("image-unrelated", "unrelated-image", imageStateComplete),
	}
	server := newImageTestServer(t, images)
	defer server.Close()

	client := newTestClientWithInventory(server.URL, vm)
	if err := client.connect(); err != nil {
		t.Fatalf("unexpected error connecting: %v", err)
	}

	client.Finalize([]*planapi.VMStatus{{VM: planapi.VM{Ref: ref.Ref{ID: "vm-1"}}}}, "test-plan")

	if _, found := images["image-1"]; found {
		t.Fatal("expected the VM's catalog image to be deleted")
	}
	if _, found := images["image-unrelated"]; !found {
		t.Fatal("expected an unrelated image to be left alone")
	}
}

func TestMigrationImageNameWithinLimit(t *testing.T) {
	name := migrationImageName(
		testMigrationUID,
		ref.Ref{ID: "52472f87-a3db-4895-5dd8-eef2efa0e157"},
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
	)
	if len(name) >= migrationImageNameMaxLen {
		t.Fatalf("expected name length < %d, got %d", migrationImageNameMaxLen, len(name))
	}
}
