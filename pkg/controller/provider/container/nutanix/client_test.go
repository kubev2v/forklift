package nutanix

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
)

// createTestClient creates a client for testing.
func createTestClient(url string) *Client {
	secret := &core.Secret{
		Data: map[string][]byte{
			"user":     []byte("admin"),
			"password": []byte("password"),
		},
	}

	return &Client{
		url:    url,
		secret: secret,
		settings: map[string]string{
			api.NutanixPrismType: api.NutanixPrismElement,
		},
		log:           logging.WithName("test"),
		clientTimeout: 30 * time.Second,
	}
}

func createTestClientWithSettings(url string, settings map[string]string) *Client {
	client := createTestClient(url)
	client.settings = settings
	return client
}

// TestClientConnect tests the connect method.
func TestClientConnect(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Error("Expected Authorization header")
		}

		// Return success
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"api_version": "3.1"}`))
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	status, err := client.connect()

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Errorf("Expected status 200, got %d", status)
	}
}

// TestClientListClusters tests listing clusters.
func TestClientListClusters(t *testing.T) {
	// Load test data
	data, err := os.ReadFile("testdata/clusters_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/nutanix/v3/clusters/list" {
			t.Errorf("Expected /api/nutanix/v3/clusters/list, got %s", r.URL.Path)
		}

		// Return test data
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.url = server.URL // Override to use mock server

	entities, err := client.listClusters()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entities) == 0 {
		t.Error("Expected at least one cluster")
	}

	// Verify first cluster has expected fields
	if len(entities) > 0 {
		cluster := entities[0]
		metadata, ok := cluster["metadata"].(map[string]interface{})
		if !ok {
			t.Error("Expected metadata field")
		}
		if _, ok := metadata["uuid"]; !ok {
			t.Error("Expected uuid in metadata")
		}
	}
}

// TestClientListHosts tests listing hosts.
func TestClientListHosts(t *testing.T) {
	// Load test data
	data, err := os.ReadFile("testdata/hosts_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Return test data
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.url = server.URL

	entities, err := client.listHosts()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entities) == 0 {
		t.Error("Expected at least one host")
	}
}

// TestClientListClusters_ExcludesPrismCentral verifies that Prism
// Central's own self-registered pseudo-cluster entry (identified by
// "PRISM_CENTRAL" in its service_list) is filtered out of the clusters
// list, leaving only real AHV/Prism Element clusters.
func TestClientListClusters_ExcludesPrismCentral(t *testing.T) {
	data, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{"total_matches": 2},
		"entities": []interface{}{
			clusterEntityWithServiceList("real-cluster", "AOS"),
			clusterEntityWithServiceList("pc-cluster", "PRISM_CENTRAL"),
		},
	})
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.url = server.URL

	entities, err := client.listClusters()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 cluster after excluding Prism Central, got %d", len(entities))
	}
	if uuid := getString(entities[0], "metadata.uuid"); uuid != "real-cluster" {
		t.Errorf("expected real-cluster, got %s", uuid)
	}
}

// TestClientListHosts_ExcludesPrismCentralHosts verifies that hosts
// belonging to Prism Central's own pseudo-cluster (its underlying
// appliance, not a real hypervisor node) are filtered out.
func TestClientListHosts_ExcludesPrismCentralHosts(t *testing.T) {
	clustersData, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{"total_matches": 2},
		"entities": []interface{}{
			clusterEntityWithServiceList("real-cluster", "AOS"),
			clusterEntityWithServiceList("pc-cluster", "PRISM_CENTRAL"),
		},
	})
	if err != nil {
		t.Fatalf("Failed to marshal clusters response: %v", err)
	}

	hostsData, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{"total_matches": 2},
		"entities": []interface{}{
			map[string]interface{}{
				"metadata": map[string]interface{}{"uuid": "real-host"},
				"spec": map[string]interface{}{
					"cluster_reference": map[string]interface{}{"uuid": "real-cluster"},
				},
			},
			map[string]interface{}{
				"metadata": map[string]interface{}{"uuid": "pc-host"},
				"spec": map[string]interface{}{
					"cluster_reference": map[string]interface{}{"uuid": "pc-cluster"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to marshal hosts response: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/api/nutanix/v3/clusters/list":
			_, _ = w.Write(clustersData)
		case "/api/nutanix/v3/hosts/list":
			_, _ = w.Write(hostsData)
		default:
			t.Fatalf("Unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.url = server.URL

	entities, err := client.listHosts()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 host after excluding Prism Central's pseudo-cluster, got %d", len(entities))
	}
	if uuid := getString(entities[0], "metadata.uuid"); uuid != "real-host" {
		t.Errorf("expected real-host, got %s", uuid)
	}
}

// TestClientListVMs tests listing VMs.
func TestClientListVMs(t *testing.T) {
	// Load test data
	data, err := os.ReadFile("testdata/vms_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Verify request body contains pagination
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if _, ok := body["length"]; !ok {
			t.Error("Expected 'length' in request body")
		}
		if _, ok := body["offset"]; !ok {
			t.Error("Expected 'offset' in request body")
		}

		// Return test data
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.url = server.URL

	entities, err := client.listVMs()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entities) == 0 {
		t.Error("Expected at least one VM")
	}

	// Verify VM has expected structure
	if len(entities) > 0 {
		vm := entities[0]
		metadata, ok := vm["metadata"].(map[string]interface{})
		if !ok {
			t.Error("Expected metadata field")
		}
		if _, ok := metadata["uuid"]; !ok {
			t.Error("Expected uuid in metadata")
		}

		status, ok := vm["status"].(map[string]interface{})
		if !ok {
			t.Error("Expected status field")
		}
		if _, ok := status["resources"]; !ok {
			t.Error("Expected resources in status")
		}
	}
}

// TestClientListSubnets tests listing networks/subnets.
func TestClientListSubnets(t *testing.T) {
	// Load test data
	data, err := os.ReadFile("testdata/subnets_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.url = server.URL

	entities, err := client.listSubnets()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entities) == 0 {
		t.Error("Expected at least one subnet")
	}
}

// TestClientListStorageContainers tests listing storage containers on Prism Element.
func TestClientListStorageContainers(t *testing.T) {
	data, err := os.ReadFile("testdata/storage_containers_v2_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/nutanix/v3/clusters/list":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
		case "/api/nutanix/v3/prism_central":
			w.WriteHeader(http.StatusNotFound)
		case "/api/nutanix/v2.0/storage_containers":
			if r.Method != "GET" {
				t.Errorf("Expected GET, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := createTestClientWithSettings(server.URL, map[string]string{
		api.NutanixPrismType: api.NutanixPrismElement,
	})
	client.url = server.URL

	entities, err := client.listStorageContainers()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entities) == 0 {
		t.Error("Expected at least one storage container")
	}
}

// TestClientListStorageContainersCentral tests listing storage containers on Prism Central.
func TestClientListStorageContainersCentral(t *testing.T) {
	data, err := os.ReadFile("testdata/storage_containers_v4_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/nutanix/v3/clusters/list":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
		case "/api/clustermgmt/v4.0/config/storage-containers":
			if r.Method != "GET" {
				t.Errorf("Expected GET, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := createTestClientWithSettings(server.URL, map[string]string{
		api.NutanixPrismType: api.NutanixPrismCentral,
	})
	client.url = server.URL

	entities, err := client.listStorageContainers()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entities) != 2 {
		t.Fatalf("Expected 2 storage containers, got %d", len(entities))
	}
}

// TestClientListImages tests listing images.
func TestClientListImages(t *testing.T) {
	// Load test data
	data, err := os.ReadFile("testdata/images_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.url = server.URL

	entities, err := client.listImages()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entities) == 0 {
		t.Error("Expected at least one image")
	}
}

// TestClientListClusters_ScopedToCluster verifies that setting clusterUuid
// scopes the clusters list down to just that cluster, instead of every
// cluster registered to Prism Central.
func TestClientListClusters_ScopedToCluster(t *testing.T) {
	data, err := os.ReadFile("testdata/clusters_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	const prodClusterUUID = "0005e123-4567-89ab-cdef-000000000001"
	client := createTestClientWithSettings(server.URL, map[string]string{
		api.NutanixPrismType:   api.NutanixPrismCentral,
		api.NutanixClusterUUID: prodClusterUUID,
	})
	client.url = server.URL

	entities, err := client.listClusters()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 cluster scoped to %s, got %d", prodClusterUUID, len(entities))
	}
	if uuid := getString(entities[0], "metadata.uuid"); uuid != prodClusterUUID {
		t.Errorf("expected cluster %s, got %s", prodClusterUUID, uuid)
	}
}

// TestClientListHosts_ScopedToCluster verifies that setting clusterUuid
// scopes the hosts list to hosts belonging to that cluster.
func TestClientListHosts_ScopedToCluster(t *testing.T) {
	data, err := os.ReadFile("testdata/hosts_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	const devClusterUUID = "0005e123-4567-89ab-cdef-000000000002"
	client := createTestClientWithSettings(server.URL, map[string]string{
		api.NutanixPrismType:   api.NutanixPrismCentral,
		api.NutanixClusterUUID: devClusterUUID,
	})
	client.url = server.URL

	entities, err := client.listHosts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 host scoped to %s, got %d", devClusterUUID, len(entities))
	}
	if name := getString(entities[0], "metadata.name"); name != "ahv-dev-node-01" {
		t.Errorf("expected ahv-dev-node-01, got %s", name)
	}
}

// TestClientListVMs_ScopedToCluster verifies that setting clusterUuid scopes
// the VM list to VMs belonging to that cluster, and that leaving it unset
// (the default) still returns VMs across every cluster.
func TestClientListVMs_ScopedToCluster(t *testing.T) {
	data, err := os.ReadFile("testdata/vms_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	const prodClusterUUID = "0005e123-4567-89ab-cdef-000000000001"
	scoped := createTestClientWithSettings(server.URL, map[string]string{
		api.NutanixPrismType:   api.NutanixPrismCentral,
		api.NutanixClusterUUID: prodClusterUUID,
	})
	scoped.url = server.URL

	entities, err := scoped.listVMs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 4 {
		t.Fatalf("expected 4 VMs scoped to %s, got %d", prodClusterUUID, len(entities))
	}
	for _, e := range entities {
		if uuid := getString(e, "spec.cluster_reference.uuid"); uuid != prodClusterUUID {
			t.Errorf("expected VM cluster %s, got %s", prodClusterUUID, uuid)
		}
	}

	unscoped := createTestClientWithSettings(server.URL, map[string]string{
		api.NutanixPrismType: api.NutanixPrismCentral,
	})
	unscoped.url = server.URL

	all, err := unscoped.listVMs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(all) != 6 {
		t.Fatalf("expected all 6 VMs when clusterUuid is unset, got %d", len(all))
	}
}

// TestClientListAllPaginates verifies that listAll follows total_matches
// across multiple pages instead of stopping after the first, which is what
// clusters/hosts/subnets/images did before they were switched to it.
func TestClientListAllPaginates(t *testing.T) {
	const total = 5
	const pageSize = 2

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		offset := int(body["offset"].(float64))

		entities := []map[string]interface{}{}
		for i := offset; i < offset+pageSize && i < total; i++ {
			entities = append(entities, map[string]interface{}{
				"metadata": map[string]interface{}{"uuid": fmt.Sprintf("uuid-%d", i)},
			})
		}

		resp := map[string]interface{}{
			"metadata": map[string]interface{}{"total_matches": total},
			"entities": entities,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.url = server.URL

	// Connect first so the initial testConnection() probe (which also hits
	// the clusters/list endpoint) doesn't count as a pagination request.
	if _, err := client.connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	requests = 0

	entities, err := client.listAll("cluster", nil, pageSize)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != total {
		t.Fatalf("expected %d entities across all pages, got %d", total, len(entities))
	}

	wantPages := 3 // pages of 2, 2, 1
	if requests != wantPages {
		t.Errorf("expected %d page requests, got %d", wantPages, requests)
	}

	seen := map[string]bool{}
	for _, e := range entities {
		uuid := e["metadata"].(map[string]interface{})["uuid"].(string)
		if seen[uuid] {
			t.Fatalf("duplicate entity %s returned across pages", uuid)
		}
		seen[uuid] = true
	}
}

// TestClientListAllStopsOnEmptyPage guards against an infinite loop if a
// page comes back empty before total_matches is reached.
func TestClientListAllStopsOnEmptyPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"metadata": map[string]interface{}{"total_matches": 100},
			"entities": []map[string]interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.url = server.URL

	entities, err := client.listAll("cluster", nil, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 0 {
		t.Fatalf("expected no entities, got %d", len(entities))
	}
}

// TestClientListAllV4Paginates verifies that listAllV4 pages through a v4
// "config" namespace endpoint using $page/$limit query params, following
// metadata.totalAvailableResults, instead of truncating at the first page.
func TestClientListAllV4Paginates(t *testing.T) {
	const total = 5
	const pageSize = 2

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/nutanix/v3/clusters/list" {
			// The connect()-time testConnection() probe; not part of the
			// paginated listing under test.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
			return
		}

		requests++

		page, err := strconv.Atoi(r.URL.Query().Get("$page"))
		if err != nil {
			t.Errorf("failed to parse $page: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		limit, err := strconv.Atoi(r.URL.Query().Get("$limit"))
		if err != nil {
			t.Errorf("failed to parse $limit: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if limit != pageSize {
			t.Errorf("expected $limit=%d, got %d", pageSize, limit)
		}

		offset := page * limit
		data := []map[string]interface{}{}
		for i := offset; i < offset+limit && i < total; i++ {
			data = append(data, map[string]interface{}{"extId": fmt.Sprintf("uuid-%d", i)})
		}

		resp := map[string]interface{}{
			"data":     data,
			"metadata": map[string]interface{}{"totalAvailableResults": total},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.url = server.URL

	if _, err := client.connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	requests = 0

	entities, err := client.listAllV4(storageContainersV4Path, pageSize)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != total {
		t.Fatalf("expected %d entities across all pages, got %d", total, len(entities))
	}

	wantPages := 3 // pages of 2, 2, 1
	if requests != wantPages {
		t.Errorf("expected %d page requests, got %d", wantPages, requests)
	}

	seen := map[string]bool{}
	for _, e := range entities {
		uuid := e["extId"].(string)
		if seen[uuid] {
			t.Fatalf("duplicate entity %s returned across pages", uuid)
		}
		seen[uuid] = true
	}
}

// TestClientListAllV4StopsOnEmptyPage guards against an infinite loop if a
// page comes back empty before totalAvailableResults is reached.
func TestClientListAllV4StopsOnEmptyPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data":     []map[string]interface{}{},
			"metadata": map[string]interface{}{"totalAvailableResults": 100},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.url = server.URL

	entities, err := client.listAllV4(storageContainersV4Path, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 0 {
		t.Fatalf("expected no entities, got %d", len(entities))
	}
}

// TestClientBasicAuth tests the basic auth encoding.
func TestClientBasicAuth(t *testing.T) {
	username := "admin"
	password := "password"

	result := basicAuth(username, password)

	// The base64 encoding of "admin:password" is "YWRtaW46cGFzc3dvcmQ="
	expected := "YWRtaW46cGFzc3dvcmQ="
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

// TestClientErrorHandling tests error handling.
func TestClientErrorHandling(t *testing.T) {
	// Create mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 401 for authentication errors
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.url = server.URL

	// Try to list clusters which should fail with unauthorized
	_, err := client.listClusters()

	if err == nil {
		t.Error("Expected error for unauthorized request")
	}
}

// TestProviderType verifies the provider type constant.
func TestProviderType(t *testing.T) {
	if api.Nutanix != "nutanix" {
		t.Errorf("Expected provider type 'nutanix', got %s", api.Nutanix)
	}
}
