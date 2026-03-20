package pure

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFcUIDToWWPN(t *testing.T) {
	testCases := []struct {
		name          string
		fcUid         string
		expectedWwpn  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid fcUid",
			fcUid:        "fc.2020202020202020:2121212121212121",
			expectedWwpn: "21:21:21:21:21:21:21:21",
			expectError:  false,
		},
		{
			name:          "missing WWPN",
			fcUid:         "fc.2020202020202020",
			expectedWwpn:  "",
			expectError:   true,
			errorContains: "not in expected fc.WWNN:WWPN format",
		},
		{
			name:          "invalid prefix",
			fcUid:         "f.2020202020202020:2121212121212121",
			expectedWwpn:  "",
			expectError:   true,
			errorContains: "doesn't start with 'fc.'",
		},
		{
			name:          "invalid format",
			fcUid:         "fc.2020202020202020:",
			expectedWwpn:  "",
			expectError:   true,
			errorContains: "empty WWNN or WWPN",
		},
		{
			name:          "odd length wwpn",
			fcUid:         "fc.2020202020202020:12345",
			expectedWwpn:  "",
			expectError:   true,
			errorContains: "odd length",
		},
		{
			name:         "lowercase input",
			fcUid:        "fc.2020202020202020:2a2b2c2d2e2f2021",
			expectedWwpn: "2A:2B:2C:2D:2E:2F:20:21", // NOSONAR
			expectError:  false,
		},
		{
			name:          "empty string",
			fcUid:         "",
			expectedWwpn:  "",
			expectError:   true,
			errorContains: "doesn't start with 'fc.'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wwpn, err := fcUIDToWWPN(tc.fcUid)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				} else if !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("expected error to contain %q, but got %q", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if wwpn != tc.expectedWwpn {
					t.Errorf("expected wwpn %q, but got %q", tc.expectedWwpn, wwpn)
				}
			}
		})
	}
}

func TestExtractSerialFromNAA(t *testing.T) {
	testCases := []struct {
		name           string
		naa            string
		expectedSerial string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "valid NAA with naa. prefix",
			naa:            "naa.624a9370abcd1234efgh5678",
			expectedSerial: "ABCD1234EFGH5678",
			expectError:    false,
		},
		{
			name:           "valid NAA without prefix",
			naa:            "624a9370abcd1234efgh5678",
			expectedSerial: "ABCD1234EFGH5678",
			expectError:    false,
		},
		{
			name:           "uppercase NAA",
			naa:            "NAA.624A9370ABCD1234EFGH5678",
			expectedSerial: "ABCD1234EFGH5678",
			expectError:    false,
		},
		{
			name:          "wrong provider ID",
			naa:           "naa.600a0980abcd1234efgh5678",
			expectError:   true,
			errorContains: "does not appear to be a Pure FlashArray device",
		},
		{
			name:          "empty serial",
			naa:           "naa.624a9370",
			expectError:   true,
			errorContains: "could not extract serial",
		},
		{
			name:          "empty string",
			naa:           "",
			expectError:   true,
			errorContains: "does not appear to be a Pure FlashArray device",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serial, err := extractSerialFromNAA(tc.naa)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				} else if !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("expected error to contain %q, but got %q", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if serial != tc.expectedSerial {
					t.Errorf("expected serial %q, but got %q", tc.expectedSerial, serial)
				}
			}
		})
	}
}

// TestAuthenticationMethods tests the different authentication methods for Pure FlashArray
func TestAuthenticationMethods(t *testing.T) {
	testCases := []struct {
		name               string
		username           string
		password           string
		token              string
		setupMockServer    func() *httptest.Server
		expectError        bool
		errorContains      string
		expectedAuthMethod string // "token" or "username_password"
	}{
		{
			name:     "token-based authentication should skip username/password",
			username: "",
			password: "",
			token:    "test-api-token-12345",
			setupMockServer: func() *httptest.Server {
				return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Mock API version endpoint
					if strings.HasSuffix(r.URL.Path, "/api/api_version") {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"version": ["1.19", "2.4"]}`))
						return
					}
					// Mock login endpoint for getting auth token
					if strings.Contains(r.URL.Path, "/login") {
						// Verify that api-token header is present
						apiToken := r.Header.Get("api-token")
						if apiToken != "test-api-token-12345" {
							w.WriteHeader(http.StatusUnauthorized)
							return
						}
						w.Header().Set("x-auth-token", "test-auth-token")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{}`))
						return
					}
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			expectError:        false,
			expectedAuthMethod: "token",
		},
		{
			name:     "username/password authentication should work when token is empty",
			username: "testuser",
			password: "testpass",
			token:    "",
			setupMockServer: func() *httptest.Server {
				return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Mock API version endpoint
					if strings.HasSuffix(r.URL.Path, "/api/api_version") {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"version": ["1.19", "2.4"]}`))
						return
					}
					// Mock API token endpoint
					if strings.Contains(r.URL.Path, "/auth/apitoken") {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"api_token": "obtained-api-token"}`))
						return
					}
					// Mock login endpoint for getting auth token
					if strings.Contains(r.URL.Path, "/login") {
						w.Header().Set("x-auth-token", "test-auth-token")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{}`))
						return
					}
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			expectError:        false,
			expectedAuthMethod: "username_password",
		},
		{
			name:     "token takes precedence when both token and username/password are provided",
			username: "testuser",
			password: "testpass",
			token:    "test-api-token-precedence",
			setupMockServer: func() *httptest.Server {
				apiTokenCalled := false
				return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Mock API version endpoint
					if strings.HasSuffix(r.URL.Path, "/api/api_version") {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"version": ["1.19", "2.4"]}`))
						return
					}
					// Mock API token endpoint - should NOT be called when token is provided
					if strings.Contains(r.URL.Path, "/auth/apitoken") {
						apiTokenCalled = true
						w.WriteHeader(http.StatusInternalServerError)
						w.Write([]byte(`{"error": "should not call this endpoint when token is provided"}`))
						return
					}
					// Mock login endpoint for getting auth token
					if strings.Contains(r.URL.Path, "/login") {
						// Verify we're using the provided token, not obtaining a new one
						if apiTokenCalled {
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						w.Header().Set("x-auth-token", "test-auth-token")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{}`))
						return
					}
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			expectError:        false,
			expectedAuthMethod: "token",
		},
		{
			name:     "invalid token should fail authentication",
			username: "",
			password: "",
			token:    "invalid-token",
			setupMockServer: func() *httptest.Server {
				return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Mock API version endpoint
					if strings.HasSuffix(r.URL.Path, "/api/api_version") {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`{"version": ["1.19", "2.4"]}`))
						return
					}
					// Mock login endpoint - reject invalid token
					if strings.Contains(r.URL.Path, "/login") {
						w.WriteHeader(http.StatusUnauthorized)
						w.Write([]byte(`{"error": "invalid token"}`))
						return
					}
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			expectError:   true,
			errorContains: "failed to get auth token",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.setupMockServer()
			defer server.Close()

			// Extract hostname from test server URL (remove https://)
			hostname := strings.TrimPrefix(server.URL, "https://")

			// Create REST client with test parameters
			client, err := NewRestClient(hostname, tc.username, tc.password, tc.token, true)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				} else if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("expected error to contain %q, but got %q", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if client == nil {
					t.Errorf("expected client to be created, but got nil")
				}
			}
		})
	}
}
