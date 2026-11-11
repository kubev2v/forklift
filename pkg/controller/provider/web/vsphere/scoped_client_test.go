package vsphere_test

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	vsphereCollector "github.com/kubev2v/forklift/pkg/controller/provider/container/vsphere"
	vspheremodel "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	providerweb "github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	vsphereweb "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	libcontainer "github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// The simulator harness below is duplicated from the collector package's
// scoped-datacenter tests: Go test files are not importable across packages.

// scopedVCenter starts a two-datacenter vCenter simulator. The collector's
// connect() logs into the vAPI REST endpoint of a vCenter, which the
// simulator does not serve, so just enough of it is served here: session
// login and an empty tag list.
func scopedVCenter(t *testing.T) (*simulator.Model, *simulator.Server) {
	t.Helper()
	sim := simulator.VPX()
	sim.Datacenter = 2
	err := sim.Create()
	if err != nil {
		t.Fatalf("create simulator model: %v", err)
	}
	t.Cleanup(sim.Remove)
	// NewServer creates the ServeMux if it is still nil; assign it here so
	// the REST handler is registered before the server accepts connections.
	if sim.Service.ServeMux == nil {
		sim.Service.ServeMux = http.NewServeMux()
	}
	sim.Service.ServeMux.HandleFunc("/rest/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/com/vmware/cis/session":
			_ = json.NewEncoder(w).Encode(map[string]string{"value": "scoped-client-test-session"})
		case "/rest/com/vmware/cis/tagging/tag":
			_ = json.NewEncoder(w).Encode(map[string][]string{"value": nil})
		default:
			http.NotFound(w, r)
		}
	})
	server := sim.Service.NewServer()
	t.Cleanup(server.Close)
	return sim, server
}

// scopedCollector builds a collector connected to the simulator with the
// given provider settings, and its own object database. The database is not
// closed by a cleanup: the container runs the collector's update loop as a
// goroutine that unwinds asynchronously after the container deletes it, and
// closing the database during that unwind races. The process is short-lived
// and the temp directory is reclaimed with the file open.
func scopedCollector(t *testing.T, server *simulator.Server, settings map[string]string) (*vsphereCollector.Collector, *api.Provider) {
	t.Helper()
	db := libmodel.New(filepath.Join(t.TempDir(), "inventory.db"), vspheremodel.All()...)
	err := db.Open(true)
	if err != nil {
		t.Fatalf("open inventory db: %v", err)
	}
	providerType := api.VSphere
	provider := &api.Provider{
		ObjectMeta: meta.ObjectMeta{
			Name:      "scoped-client-test",
			Namespace: "test",
			UID:       "scoped-client-test-uid",
		},
		Spec: api.ProviderSpec{
			Type:     &providerType,
			URL:      server.URL.String(),
			Settings: settings,
		},
	}
	password, _ := server.URL.User.Password()
	secret := &core.Secret{
		Data: map[string][]byte{
			"user":               []byte(server.URL.User.Username()),
			"password":           []byte(password),
			"insecureSkipVerify": []byte("true"),
		},
	}
	return vsphereCollector.New(db, provider, secret), provider
}

// TestScopedInventoryVMRefs asserts the datacenter scope at the seam plan
// validation resolves references through: the provider's inventory REST
// client. A VM reference inside the scoped datacenter resolves by ID and by
// name; a reference to a VM in the other datacenter is not found either way.
func TestScopedInventoryVMRefs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sim, server := scopedVCenter(t)
	collector, provider := scopedCollector(t, server, map[string]string{api.Datacenter: "DC0"})
	container := libcontainer.New()
	err := container.Add(collector)
	if err != nil {
		t.Fatalf("add the collector to the container: %v", err)
	}
	t.Cleanup(func() {
		container.Delete(provider)
		// The collector's update loop unwinds asynchronously after the
		// delete cancels it; wait for it here so its logout does not race
		// the simulator cleanup's server shutdown.
		deadline := time.Now().Add(30 * time.Second)
		for collector.HasParity() {
			if time.Now().After(deadline) {
				t.Log("collector still reports parity at cleanup; continuing anyway")
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	})
	waitParity(t, collector)

	router := gin.New()
	for _, h := range vsphereweb.Handlers(container) {
		h.AddRoutes(router)
	}
	restServer := httptest.NewServer(router)
	t.Cleanup(restServer.Close)
	pointClientAt(t, restServer)
	writeKubeconfig(t)

	client, err := providerweb.NewClient(provider)
	if err != nil {
		t.Fatalf("build the inventory client: %v", err)
	}
	dc0VM := simulatorVM(t, sim, "DC0")
	dc1VM := simulatorVM(t, sim, "DC1")

	t.Run("a VM inside the scoped datacenter resolves", func(t *testing.T) {
		_, err = client.VM(&ref.Ref{ID: dc0VM.Self.Value})
		if err != nil {
			t.Fatalf("resolve the scoped VM by ID: %v", err)
		}
		_, err = client.VM(&ref.Ref{Name: dc0VM.Name})
		if err != nil {
			t.Fatalf("resolve the scoped VM by name: %v", err)
		}
	})

	t.Run("a VM reference outside the scoped datacenter is not found", func(t *testing.T) {
		_, err = client.VM(&ref.Ref{ID: dc1VM.Self.Value})
		expectNotFound(t, err)
		_, err = client.VM(&ref.Ref{Name: dc1VM.Name})
		expectNotFound(t, err)
	})
}

// waitParity blocks until the collector reports inventory parity, failing
// the test on timeout.
func waitParity(t *testing.T, collector *vsphereCollector.Collector) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !collector.HasParity() {
		if time.Now().After(deadline) {
			t.Fatal("collector never reached inventory parity")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// simulatorVM returns a virtual machine whose parent chain reaches the named
// datacenter in the simulator.
func simulatorVM(t *testing.T, sim *simulator.Model, dcName string) *simulator.VirtualMachine {
	t.Helper()
	var dcRef types.ManagedObjectReference
	for _, e := range sim.Map().All("Datacenter") {
		if dc, cast := e.(*simulator.Datacenter); cast && dc.Name == dcName {
			dcRef = dc.Self
			break
		}
	}
	if dcRef.Value == "" {
		t.Fatalf("datacenter %q not found in the simulator", dcName)
	}
	for _, e := range sim.Map().All("VirtualMachine") {
		vm, cast := e.(*simulator.VirtualMachine)
		if !cast {
			continue
		}
		for parent := vm.Entity().Parent; parent != nil; parent = simulatorParent(sim, parent) {
			if *parent == dcRef {
				return vm
			}
		}
	}
	t.Fatalf("no virtual machine under datacenter %q", dcName)
	return nil
}

// simulatorParent returns the parent of a simulator entity, or nil at the
// root folder.
func simulatorParent(sim *simulator.Model, ref *types.ManagedObjectReference) *types.ManagedObjectReference {
	entity, cast := sim.Map().Get(*ref).(mo.Entity)
	if !cast {
		return nil
	}
	return entity.Entity().Parent
}

// pointClientAt redirects the inventory REST client at the test server and
// disables request authorization, restoring both when the test ends.
func pointClientAt(t *testing.T, srv *httptest.Server) {
	t.Helper()
	host, port, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("split the test server address: %v", err)
	}
	nPort, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("parse the test server port: %v", err)
	}
	s := base.Settings
	origScheme, origHost, origPort, origAuth :=
		s.Scheme, s.Host, s.Inventory.Port, s.AuthRequired
	s.Scheme = "http"
	s.Host = host
	s.Inventory.Port = nPort
	s.AuthRequired = false
	t.Cleanup(func() {
		s.Scheme = origScheme
		s.Host = origHost
		s.Inventory.Port = origPort
		s.AuthRequired = origAuth
	})
}

// writeKubeconfig writes a minimal kubeconfig and points KUBECONFIG at it:
// the REST client reads its authorization token through controller-runtime's
// kubeconfig, which must load before any request is sent.
func writeKubeconfig(t *testing.T) {
	t.Helper()
	const config = `apiVersion: v1
kind: Config
clusters:
- name: scoped-client-test
  cluster:
    server: https://127.0.0.1:1
contexts:
- name: scoped-client-test
  context:
    cluster: scoped-client-test
    user: scoped-client-test
current-context: scoped-client-test
users:
- name: scoped-client-test
  user: {}
`
	path := filepath.Join(t.TempDir(), "kubeconfig")
	err := os.WriteFile(path, []byte(config), 0o600)
	if err != nil {
		t.Fatalf("write the kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG", path)
}

// expectNotFound fails the test unless the error is the inventory client's
// not-found error.
func expectNotFound(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected the reference to not resolve")
	}
	var notFound providerweb.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected a NotFoundError, got %v", err)
	}
}
