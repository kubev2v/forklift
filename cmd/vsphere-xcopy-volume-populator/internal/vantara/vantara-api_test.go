package vantara

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewBlockStorageAPI tests the constructor
func TestNewBlockStorageAPI(t *testing.T) {
	api := NewBlockStorageAPI("192.0.2.0", "8443", "storage123", "admin", "password")

	if api.GumIPAddr != "192.0.2.0" {
		t.Errorf("Expected GumIPAddr=192.0.2.0, got %s", api.GumIPAddr)
	}
	if api.Port != "8443" {
		t.Errorf("Expected Port=8443, got %s", api.Port)
	}
	if api.StorageID != "storage123" {
		t.Errorf("Expected StorageID=storage123, got %s", api.StorageID)
	}
	if api.username != "admin" {
		t.Errorf("Expected username=admin, got %s", api.username)
	}
	if api.password != "password" {
		t.Errorf("Expected password=password, got %s", api.password) // NOSONAR
	}
	if api.httpClient == nil {
		t.Error("Expected httpClient to be initialized")
	}
	if api.isConnected {
		t.Error("Expected isConnected to be false initially")
	}
}

// TestConnectSuccess tests successful connection
func TestConnectSuccess(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ConfigurationManager/configuration/version":
			// API version endpoint
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"apiVersion": "1.9.0",
			})
		case "/ConfigurationManager/v1/objects/sessions":
			// Session creation endpoint
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":     "test-token-123",
				"sessionId": float64(42),
			})
		default:
			t.Errorf("Unexpected request to %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	api := NewBlockStorageAPI(server.Listener.Addr().String(), "", "storage123", "admin", "password")
	// Override the base URL to use test server
	api.BaseURL = server.URL + "/ConfigurationManager/v1"

	err := api.Connect()
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	if !api.isConnected {
		t.Error("Expected isConnected to be true after Connect()")
	}
	if api.sessionToken != "test-token-123" {
		t.Errorf("Expected sessionToken=test-token-123, got %s", api.sessionToken)
	}
	if api.sessionId != "42" {
		t.Errorf("Expected sessionId=42, got %s", api.sessionId)
	}
}

// TestConnectReuseSession tests that Connect reuses existing session
func TestConnectReuseSession(t *testing.T) {
	callCount := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch r.URL.Path {
		case "/ConfigurationManager/configuration/version":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"apiVersion": "1.9.0"})
		case "/ConfigurationManager/v1/objects/sessions":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":     "test-token-123",
				"sessionId": float64(42),
			})
		}
	}))
	defer server.Close()

	api := NewBlockStorageAPI(server.Listener.Addr().String(), "", "storage123", "admin", "password")
	api.BaseURL = server.URL + "/ConfigurationManager/v1"

	// First connect
	err := api.Connect()
	if err != nil {
		t.Fatalf("First Connect() failed: %v", err)
	}
	firstCallCount := callCount

	// Second connect should reuse session
	err = api.Connect()
	if err != nil {
		t.Fatalf("Second Connect() failed: %v", err)
	}

	if callCount != firstCallCount {
		t.Errorf("Expected no additional API calls on second Connect(), but got %d total calls", callCount)
	}
}

// TestDisconnect tests session cleanup
func TestDisconnect(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ConfigurationManager/configuration/version":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"apiVersion": "1.9.0"})
		case "/ConfigurationManager/v1/objects/sessions":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":     "test-token-123",
				"sessionId": float64(42),
			})
		case "/ConfigurationManager/v1/objects/sessions/42":
			// Session deletion
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	}))
	defer server.Close()

	api := NewBlockStorageAPI(server.Listener.Addr().String(), "", "storage123", "admin", "password")
	api.BaseURL = server.URL + "/ConfigurationManager/v1"

	// Connect first
	api.Connect()

	// Now disconnect
	err := api.Disconnect()
	if err != nil {
		t.Fatalf("Disconnect() failed: %v", err)
	}

	if api.isConnected {
		t.Error("Expected isConnected to be false after Disconnect()")
	}
	if api.sessionToken != "" {
		t.Error("Expected sessionToken to be empty after Disconnect()")
	}
	if api.sessionId != "" {
		t.Error("Expected sessionId to be empty after Disconnect()")
	}
}

// TestGetLdev tests GetLdev method
func TestGetLdev(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ConfigurationManager/configuration/version":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"apiVersion": "1.9.0"})
		case "/ConfigurationManager/v1/objects/sessions":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":     "test-token-123",
				"sessionId": float64(42),
			})
		case "/ConfigurationManager/v1/objects/ldevs/100":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ldevId": 100,
				"naaId":  "60060E8012345678",
				"ports": []interface{}{
					map[string]interface{}{
						"portId":          "CL1-A",
						"hostGroupName":   "HG01",
						"hostGroupNumber": float64(1),
						"lun":             float64(0),
					},
				},
			})
		}
	}))
	defer server.Close()

	api := NewBlockStorageAPI(server.Listener.Addr().String(), "", "storage123", "admin", "password")
	api.BaseURL = server.URL + "/ConfigurationManager/v1"

	ldev, err := api.GetLdev("100")
	if err != nil {
		t.Fatalf("GetLdev() failed: %v", err)
	}

	if ldev.LdevId != 100 {
		t.Errorf("Expected LdevId=100, got %f", ldev.LdevId)
	}
	if ldev.NaaId != "60060E8012345678" {
		t.Errorf("Expected NaaId=60060E8012345678, got %s", ldev.NaaId)
	}
	if len(ldev.Ports) != 1 {
		t.Fatalf("Expected 1 port, got %d", len(ldev.Ports))
	}
	if ldev.Ports[0].PortId != "CL1-A" {
		t.Errorf("Expected PortId=CL1-A, got %s", ldev.Ports[0].PortId)
	}
}

// TestAddPath tests AddPath method
func TestAddPath(t *testing.T) {
	jobCompleted := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ConfigurationManager/configuration/version":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"apiVersion": "1.9.0"})
		case "/ConfigurationManager/v1/objects/sessions":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":     "test-token-123",
				"sessionId": float64(42),
			})
		case "/ConfigurationManager/v1/objects/luns":
			// AddPath endpoint
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jobId": float64(100),
				"self":  "/jobs/100",
			})
		case "/ConfigurationManager/v1/objects/jobs/100":
			// Job status endpoint
			w.WriteHeader(http.StatusOK)
			if jobCompleted {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "Completed",
				})
			} else {
				jobCompleted = true
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "Initializing",
				})
			}
		}
	}))
	defer server.Close()

	api := NewBlockStorageAPI(server.Listener.Addr().String(), "", "storage123", "admin", "password")
	api.BaseURL = server.URL + "/ConfigurationManager/v1"

	err := api.AddPath("100", "CL1-A", "1")
	if err != nil {
		t.Fatalf("AddPath() failed: %v", err)
	}
}

// TestDeletePath tests DeletePath method
func TestDeletePath(t *testing.T) {
	jobCompleted := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ConfigurationManager/configuration/version":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"apiVersion": "1.9.0"})
		case "/ConfigurationManager/v1/objects/sessions":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":     "test-token-123",
				"sessionId": float64(42),
			})
		case "/ConfigurationManager/v1/objects/luns/CL1-A,1,0":
			// DeletePath endpoint
			if r.Method != "DELETE" {
				t.Errorf("Expected DELETE method, got %s", r.Method)
			}
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jobId": float64(101),
				"self":  "/jobs/101",
			})
		case "/ConfigurationManager/v1/objects/jobs/101":
			w.WriteHeader(http.StatusOK)
			if jobCompleted {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "Completed",
				})
			} else {
				jobCompleted = true
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "Initializing",
				})
			}
		}
	}))
	defer server.Close()

	api := NewBlockStorageAPI(server.Listener.Addr().String(), "", "storage123", "admin", "password")
	api.BaseURL = server.URL + "/ConfigurationManager/v1"

	err := api.DeletePath("100", "CL1-A", "1", "0")
	if err != nil {
		t.Fatalf("DeletePath() failed: %v", err)
	}
}

// TestGetPortDetails tests GetPortDetails method
func TestGetPortDetails(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ConfigurationManager/configuration/version":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"apiVersion": "1.9.0"})
		case "/ConfigurationManager/v1/objects/sessions":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":     "test-token-123",
				"sessionId": float64(42),
			})
		case "/ConfigurationManager/v1/objects/ports":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"portId": "CL1-A",
						"wwn":    "50060E801234ABCD",
						"logins": []interface{}{
							map[string]interface{}{
								"hostGroupId": "CL1-A,1",
								"isLogin":     "true",
								"loginWwn":    "21000024FF123456",
							},
						},
					},
				},
			})
		}
	}))
	defer server.Close()

	api := NewBlockStorageAPI(server.Listener.Addr().String(), "", "storage123", "admin", "password")
	api.BaseURL = server.URL + "/ConfigurationManager/v1"

	portDetails, err := api.GetPortDetails()
	if err != nil {
		t.Fatalf("GetPortDetails() failed: %v", err)
	}

	if len(portDetails.Data) != 1 {
		t.Fatalf("Expected 1 port, got %d", len(portDetails.Data))
	}
	if portDetails.Data[0].PortID != "CL1-A" {
		t.Errorf("Expected PortID=CL1-A, got %s", portDetails.Data[0].PortID)
	}
}

// TestSessionExpiration tests that sessions are refreshed when approaching expiration
func TestSessionExpiration(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ConfigurationManager/configuration/version":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"apiVersion": "1.9.0"})
		case "/ConfigurationManager/v1/objects/sessions":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":     "test-token-123",
				"sessionId": float64(42),
			})
		case "/ConfigurationManager/v1/objects/sessions/42":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	}))
	defer server.Close()

	api := NewBlockStorageAPI(server.Listener.Addr().String(), "", "storage123", "admin", "password")
	api.BaseURL = server.URL + "/ConfigurationManager/v1"

	// Connect
	api.Connect()

	// Simulate session aging by setting start time to 26 minutes ago
	api.sessionStartTime = time.Now().Add(-26 * time.Minute)

	// ensureConnected should trigger reconnection
	err := api.ensureConnected()
	if err != nil {
		t.Fatalf("ensureConnected() failed: %v", err)
	}

	// Check that session was refreshed
	if time.Since(api.sessionStartTime) > 1*time.Minute {
		t.Error("Expected session to be refreshed with recent start time")
	}
}

// TestExtractIPAddress tests IP extraction
func TestExtractIPAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		hasError bool
	}{
		{"192.0.2.0", "192.0.2.0", false},
		{"https://192.0.2.0:8443", "192.0.2.0", false},
		{"http://192.0.2.0/path", "192.0.2.0", false},
		{"hostname.example.com", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := extractIPAddress(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for input %s, but got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %s: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("For input %s, expected %s, got %s", tt.input, tt.expected, result)
				}
			}
		})
	}
}

// TestCheckAPIVersion tests API version checking
func TestCheckAPIVersion(t *testing.T) {
	tests := []struct {
		version  string
		hasError bool
	}{
		{"1.9.0", false},
		{"1.10.0", false},
		{"2.0.0", false},
		{"1.8.0", true},   // Too old
		{"0.9.0", true},   // Too old
		{"invalid", true}, // Invalid format
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			err := CheckAPIVersion(tt.version, 1, 9)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for version %s, but got nil", tt.version)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for version %s: %v", tt.version, err)
				}
			}
		})
	}
}

// TestHTTPErrorHandling tests error handling in HTTP requests
func TestHTTPErrorHandling(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ConfigurationManager/configuration/version":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"apiVersion": "1.9.0"})
		case "/ConfigurationManager/v1/objects/sessions":
			// Simulate authentication error
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "Authentication failed")
		}
	}))
	defer server.Close()

	api := NewBlockStorageAPI(server.Listener.Addr().String(), "", "storage123", "admin", "wrongpassword")
	api.BaseURL = server.URL + "/ConfigurationManager/v1"

	err := api.Connect()
	if err == nil {
		t.Error("Expected error on authentication failure, got nil")
	}
}
