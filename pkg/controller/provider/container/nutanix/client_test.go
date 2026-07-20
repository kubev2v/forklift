package nutanix

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
		url:           url,
		secret:        secret,
		log:           logging.WithName("test"),
		clientTimeout: 30 * time.Second,
	}
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
		w.Write([]byte(`{"api_version": "3.1"}`))
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
		w.Write(data)
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
		w.Write(data)
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
		w.Write(data)
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
		w.Write(data)
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

// TestClientListStorageContainers tests listing storage containers.
func TestClientListStorageContainers(t *testing.T) {
	// Load test data
	data, err := os.ReadFile("testdata/storage_containers_list.json")
	if err != nil {
		t.Fatalf("Failed to read testdata: %v", err)
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	client := createTestClient(server.URL)
	client.url = server.URL

	entities, err := client.listStorageContainers()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(entities) == 0 {
		t.Error("Expected at least one storage container")
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
		w.Write(data)
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
		w.Write([]byte(`{"error": "unauthorized"}`))
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
