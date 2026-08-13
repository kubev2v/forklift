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

	var response struct {
		Entities []storageContainerV2Raw `json:"entities"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(response.Entities) == 0 {
		t.Fatal("no entities in response")
	}

	m := &model.StorageContainer{}
	e := response.Entities[0].toEntity()
	e.ApplyTo(m)

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

	var response struct {
		Data []storageContainerV4Raw `json:"data"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(response.Data) == 0 {
		t.Fatal("no entities in response")
	}

	m := &model.StorageContainer{}
	e := response.Data[0].toEntity()
	e.ApplyTo(m)

	if m.Name != "default-container-prod" {
		t.Fatalf("unexpected name: %s", m.Name)
	}
	if m.StorageContainerUUID == "" {
		t.Fatal("expected storage container uuid")
	}
}

func TestFilterStorageContainersByCluster_Prism(t *testing.T) {
	entities := []storageContainerEntity{
		storageContainerV4Raw{ExtID: "sc-1", Name: "one", ClusterExtID: "cluster-a"}.toEntity(),
		storageContainerV4Raw{ExtID: "sc-2", Name: "two", ClusterExtID: "cluster-b"}.toEntity(),
	}

	filtered := filterStorageContainersByCluster(entities, "cluster-a")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 container, got %d", len(filtered))
	}
}

// TestImageEntityFromV4 verifies that a v4 image raw entity is normalised into
// the canonical imageEntity and ApplyTo reads every mapped field correctly.
func TestImageEntityFromV4(t *testing.T) {
	raw := imageV4Raw{
		ExtID:     "img-1",
		Name:      "RHEL-8.9-x86_64",
		Type:      "DISK_IMAGE",
		SizeBytes: 2147483648,
	}

	m := &model.Image{}
	e := raw.toEntity()
	e.ApplyTo(m)

	if m.ID != "img-1" {
		t.Errorf("expected ID 'img-1', got %q", m.ID)
	}
	if m.ImageUUID != "img-1" {
		t.Errorf("expected ImageUUID 'img-1', got %q", m.ImageUUID)
	}
	if m.Name != "RHEL-8.9-x86_64" {
		t.Errorf("expected name to be set, got %q", m.Name)
	}
	if m.ImageType != "DISK_IMAGE" {
		t.Errorf("expected image type 'DISK_IMAGE', got %q", m.ImageType)
	}
	if m.SizeBytes != 2147483648 {
		t.Errorf("expected size 2147483648, got %d", m.SizeBytes)
	}
}

// TestImageEntityFromV4MissingFields guards against a panic when optional
// v4 fields (e.g. a missing/unset source) are absent from the raw entity.
func TestImageEntityFromV4MissingFields(t *testing.T) {
	raw := imageV4Raw{ExtID: "img-2", Name: "minimal-image"}
	entity := raw.toEntity()

	m := &model.Image{}
	entity.ApplyTo(m)

	if m.ID != "img-2" {
		t.Errorf("expected ID 'img-2', got %q", m.ID)
	}
	if m.SourceURI != "" {
		t.Errorf("expected empty SourceURI, got %q", m.SourceURI)
	}
}
