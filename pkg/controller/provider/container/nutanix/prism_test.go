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
			w.Write([]byte(`{"entities":[]}`))
		case "/api/nutanix/v3/clusters/list":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"entities":[]}`))
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
			w.Write([]byte(`{"resources":{"version":"pc.2024.1"}}`))
		case "/api/nutanix/v3/clusters/list":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"entities":[]}`))
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
			w.Write([]byte(`{"entities":[]}`))
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
