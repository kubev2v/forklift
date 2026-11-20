package nutanix

import (
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/nutanix"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
)

func testCluster() *model.Cluster {
	cluster := &model.Cluster{ExternalIP: "10.0.0.50"}
	cluster.ID = "0005e123-4567-89ab-cdef-000000000001"
	return cluster
}

func TestCatalogImageListURL_PrismElementUsesClusterVIP(t *testing.T) {
	got := catalogImageListURL("https://pe.example.com:9440", testCluster(), true)
	want := "https://10.0.0.50:9440/PrismGateway/services/rest/v2.0/images/?count=4096"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCatalogImageListURL_PrismCentralProxiesToCluster(t *testing.T) {
	got := catalogImageListURL("https://pc.example.com:9440/", testCluster(), false)

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("unparseable URL %q: %v", got, err)
	}
	if parsed.Host != "pc.example.com:9440" {
		t.Errorf("expected request to Prism Central, got host %q", parsed.Host)
	}
	if parsed.Path != "/PrismGateway/services/rest/v2.0/images/" {
		t.Errorf("unexpected path %q", parsed.Path)
	}
	if v := parsed.Query().Get("proxyClusterUuid"); v != testCluster().ID {
		t.Errorf("proxyClusterUuid = %q, want %q", v, testCluster().ID)
	}
	if v := parsed.Query().Get("count"); v != "4096" {
		t.Errorf("count = %q, want 4096", v)
	}
}

// newCatalogImageServer is a TLS Prism stand-in: it accepts the client's
// connection test and serves a v2 image list, recording the image request so
// tests can assert what resolveCatalogImage sent.
func newCatalogImageServer(t *testing.T, status int, body string, got **http.Request) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/nutanix/v3/clusters/list":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/PrismGateway/services/rest/v2.0/images/":
			if got != nil {
				*got = r.Clone(r.Context())
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// catalogImageTestSetup returns a builder and a connected client whose
// secret trusts the test server only through the legacy "cacert" key, with
// TLS verification enabled -- the configuration that broke the original
// hand-rolled HTTP client.
func catalogImageTestSetup(t *testing.T, server *httptest.Server) (*Builder, *Client) {
	t.Helper()
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	ctx := &plancontext.Context{
		Source: plancontext.Source{
			Provider: &api.Provider{Spec: api.ProviderSpec{URL: server.URL}},
			Secret: &core.Secret{Data: map[string][]byte{
				"user":               []byte("pc-user"),
				"password":           []byte("pc-pass"),
				"cacert":             caPEM,
				"insecureSkipVerify": []byte("false"),
			}},
		},
		Log: logging.WithName("test"),
	}
	client := &Client{Context: ctx}
	if err := client.connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	return &Builder{Context: ctx}, client
}

func TestResolveCatalogImage_PrismCentralProxy(t *testing.T) {
	const body = `{"entities":[
		{"name":"other-image","vm_disk_id":"vmdisk-other","storage_container_uuid":"sc-other"},
		{"name":"forklift-migration-x","vm_disk_id":"vmdisk-1","storage_container_uuid":"sc-1"}
	]}`
	var req *http.Request
	server := newCatalogImageServer(t, http.StatusOK, body, &req)
	builder, client := catalogImageTestSetup(t, server)

	endpoint := catalogImageListURL(server.URL, testCluster(), false)
	vmDiskID, containerUUID, err := builder.resolveCatalogImage(client, endpoint, "forklift-migration-x")
	if err != nil {
		t.Fatalf("resolveCatalogImage: %v", err)
	}
	if vmDiskID != "vmdisk-1" || containerUUID != "sc-1" {
		t.Fatalf("got (%q, %q), want (vmdisk-1, sc-1)", vmDiskID, containerUUID)
	}
	if req == nil {
		t.Fatal("image list was not requested")
	}
	if v := req.URL.Query().Get("proxyClusterUuid"); v != testCluster().ID {
		t.Errorf("request proxyClusterUuid = %q, want %q", v, testCluster().ID)
	}
	if user, pass, ok := req.BasicAuth(); !ok || user != "pc-user" || pass != "pc-pass" {
		t.Errorf("expected provider credentials on request, got (%q, %q, %v)", user, pass, ok)
	}
}

func TestResolveCatalogImage_NotYetBackedIsNotReady(t *testing.T) {
	const body = `{"entities":[{"name":"forklift-migration-x","vm_disk_id":"","storage_container_uuid":"sc-1"}]}`
	server := newCatalogImageServer(t, http.StatusOK, body, nil)
	builder, client := catalogImageTestSetup(t, server)

	_, _, err := builder.resolveCatalogImage(client, catalogImageListURL(server.URL, testCluster(), false), "forklift-migration-x")
	if !errors.As(err, &web.ProviderNotReadyError{}) {
		t.Fatalf("expected ProviderNotReadyError, got %v", err)
	}
}

func TestResolveCatalogImage_MissingIsNotReady(t *testing.T) {
	server := newCatalogImageServer(t, http.StatusOK, `{"entities":[]}`, nil)
	builder, client := catalogImageTestSetup(t, server)

	_, _, err := builder.resolveCatalogImage(client, catalogImageListURL(server.URL, testCluster(), false), "forklift-migration-x")
	if !errors.As(err, &web.ProviderNotReadyError{}) {
		t.Fatalf("expected ProviderNotReadyError, got %v", err)
	}
}

func TestResolveCatalogImage_HTTPErrorFails(t *testing.T) {
	server := newCatalogImageServer(t, http.StatusUnauthorized, ``, nil)
	builder, client := catalogImageTestSetup(t, server)

	_, _, err := builder.resolveCatalogImage(client, catalogImageListURL(server.URL, testCluster(), false), "forklift-migration-x")
	if err == nil || errors.As(err, &web.ProviderNotReadyError{}) {
		t.Fatalf("expected a hard error for HTTP 401, got %v", err)
	}
}
