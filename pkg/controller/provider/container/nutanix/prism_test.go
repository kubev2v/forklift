package nutanix

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
)

func TestParsePrismMode(t *testing.T) {
	mode, err := parsePrismMode(api.NutanixPrismElement)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != PrismElement {
		t.Fatalf("expected element, got %s", mode)
	}

	_, err = parsePrismMode("invalid")
	if err == nil {
		t.Fatal("expected error for invalid prismType")
	}
}

func TestDetectPrismMode_Element(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/nutanix/v3/prism_central":
			w.WriteHeader(http.StatusNotFound)
		case "/api/nutanix/v2.0/storage_containers":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
		case "/api/nutanix/v3/clusters/list":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	mode, err := client.detectPrismMode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != PrismElement {
		t.Fatalf("expected element, got %s", mode)
	}
}

func TestDetectPrismMode_Central(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/nutanix/v3/prism_central":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"resources":{"version":"pc.2024.1"}}`))
		case "/api/nutanix/v3/clusters/list":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	mode, err := client.detectPrismMode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != PrismCentral {
		t.Fatalf("expected central, got %s", mode)
	}
}

func TestResolvePrismConfig_Explicit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/nutanix/v3/clusters/list" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.settings = map[string]string{
		api.NutanixPrismType: api.NutanixPrismElement,
	}

	config, err := client.resolvePrismConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !config.Explicit {
		t.Fatal("expected explicit config")
	}
	if config.Mode != PrismElement {
		t.Fatalf("expected element, got %s", config.Mode)
	}
}

// TestResolvePrismConfig_AutoDetect verifies the fallback path taken when no
// prismType setting is configured: resolvePrismConfig defers to
// detectPrismMode instead of trusting an explicit setting.
func TestResolvePrismConfig_AutoDetect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/nutanix/v3/prism_central":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"resources":{"version":"pc.2024.1"}}`))
		case "/api/nutanix/v3/clusters/list":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := createTestClientWithSettings(server.URL, map[string]string{})

	config, err := client.resolvePrismConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Explicit {
		t.Fatal("expected a non-explicit config when no prismType setting is present")
	}
	if config.Mode != PrismCentral {
		t.Fatalf("expected auto-detected central, got %s", config.Mode)
	}
}

// TestEnsurePrismConfig_CachesResult verifies that a second call to
// ensurePrismConfig doesn't re-resolve (and re-probe the API) once the mode
// has already been determined.
func TestEnsurePrismConfig_CachesResult(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch r.URL.Path {
		case "/api/nutanix/v3/prism_central":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"resources":{"version":"pc.2024.1"}}`))
		case "/api/nutanix/v3/clusters/list":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := createTestClientWithSettings(server.URL, map[string]string{})

	if err := client.ensurePrismConfig(); err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if client.prism.Mode != PrismCentral {
		t.Fatalf("expected central, got %s", client.prism.Mode)
	}

	countAfterFirst := requestCount
	if countAfterFirst == 0 {
		t.Fatal("expected the first call to make at least one request")
	}

	if err := client.ensurePrismConfig(); err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if requestCount != countAfterFirst {
		t.Fatalf("expected ensurePrismConfig to be cached after the first resolution; "+
			"request count grew from %d to %d", countAfterFirst, requestCount)
	}
}

// TestDetectPrismMode_NeitherResponds verifies that detectPrismMode returns
// an error, rather than silently defaulting, when neither the Prism Central
// nor Prism Element probe succeeds.
func TestDetectPrismMode_NeitherResponds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/nutanix/v3/clusters/list":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	_, err := client.detectPrismMode()
	if err == nil {
		t.Fatal("expected an error when neither Prism Central nor Element probes succeed")
	}
}

func TestStorageContainerEntityFromV2(t *testing.T) {
	data, err := os.ReadFile("testdata/storage_containers_v2_list.json")
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	entities, err := extractMapList(response, "entities")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entity := storageContainerEntityFromV2(entities[0])
	m := &model.StorageContainer{}
	applyStorageContainer(entity, m)

	if m.Name != "default-container-prod" {
		t.Fatalf("unexpected name: %s", m.Name)
	}
	if m.StorageContainerUUID == "" {
		t.Fatal("expected storage container uuid")
	}
	if m.Cluster == "" {
		t.Fatal("expected cluster uuid")
	}
	if m.MaxCapacityBytes == 0 {
		t.Fatal("expected max capacity")
	}
	if m.UsageBytes == 0 {
		t.Fatal("expected usage bytes")
	}
}

func TestStorageContainerEntityFromV4(t *testing.T) {
	data, err := os.ReadFile("testdata/storage_containers_v4_list.json")
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	entities, err := extractMapList(response, "data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entity := storageContainerEntityFromV4(entities[0])
	m := &model.StorageContainer{}
	applyStorageContainer(entity, m)

	if m.Name != "default-container-prod" {
		t.Fatalf("unexpected name: %s", m.Name)
	}
	if m.StorageContainerUUID == "" {
		t.Fatal("expected storage container uuid")
	}
}

func TestFilterStorageContainersByCluster(t *testing.T) {
	entities := []map[string]interface{}{
		storageContainerEntityFromV4(map[string]interface{}{
			"extId":        "sc-1",
			"name":         "one",
			"clusterExtId": "cluster-a",
		}),
		storageContainerEntityFromV4(map[string]interface{}{
			"extId":        "sc-2",
			"name":         "two",
			"clusterExtId": "cluster-b",
		}),
	}

	filtered := filterStorageContainersByCluster(entities, "cluster-a")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 container, got %d", len(filtered))
	}
}
