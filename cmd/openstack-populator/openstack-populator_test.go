package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func setupMockServer() (*httptest.Server, string, int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, "", 0, err
	}

	mux := http.NewServeMux()

	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://localhost:%d", port)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf(`{
            "versions": {
                "values": [
                    {
                        "id": "v3.0",
                        "links": [
                            {"rel": "self", "href": "%s/v3/"}
                        ],
                        "status": "stable"
                    }
                ]
            }
        }`, baseURL)
		fmt.Fprint(w, response)
	})

	mux.HandleFunc("/v2/images/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `mock_data`)
	})

	mux.HandleFunc("/v3/auth/tokens", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Subject-Token", "MIIFvgY")
		w.WriteHeader(http.StatusCreated)
		identityServer := fmt.Sprintf("%s/v3/", baseURL)
		imageServiceURL := fmt.Sprintf("%s/v2/images", baseURL)
		fmt.Println("identityServer ", identityServer)
		response := fmt.Sprintf(`{
			"token": {
				"methods": ["password"],
				"project": {
					"domain": {
						"id": "default",
						"name": "Default"
					},
					"id": "8538a3f13f9541b28c2620eb19065e45",
					"name": "admin"
				},
				"catalog": [
					{
						"type": "identity",
						"name": "keystone",
						"endpoints": [
							{
								"url": "%s",
								"region": "RegionOne",
								"interface": "public",
								"id": "identity-public-endpoint-id"
							},
							{
								"url": "%s",
								"region": "RegionOne",
								"interface": "admin",
								"id": "identity-admin-endpoint-id"
							},
							{
								"url": "%s",
								"region": "RegionOne",
								"interface": "internal",
								"id": "identity-internal-endpoint-id"
							}
						]
					},
					{
						"type": "image",
						"name": "glance",
						"endpoints": [
							{
								"url": "%s",
								"region": "RegionOne",
								"interface": "public",
								"id": "image-public-endpoint-id"
							}
						]
					}
				],
				"user": {
					"domain": {
						"id": "default",
						"name": "Default"
					},
					"id": "3ec3164f750146be97f21559ee4d9c51",
					"name": "admin"
				},
				"issued_at": "201406-10T20:55:16.806027Z"
			}
		}`,
			identityServer,
			identityServer,
			identityServer,
			imageServiceURL)

		fmt.Fprint(w, response)
	})

	server := httptest.NewUnstartedServer(mux)
	server.Listener = listener

	server.Start()

	return server, baseURL, port, nil
}

func TestPopulate(t *testing.T) {
	os.Setenv("username", "testuser")
	os.Setenv("password", "testpassword")
	os.Setenv("projectName", "Default")
	os.Setenv("domainName", "Default")
	os.Setenv("insecureSkipVerify", "true")
	os.Setenv("availability", "public")
	os.Setenv("regionName", "RegionOne")
	os.Setenv("authType", "password")

	server, identityServerURL, port, err := setupMockServer()
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer server.Close()

	fmt.Printf("Mock server running on port: %d\n", port)

	fileName := "disk.img"
	secretName := "test-secret"
	imageID := "test-image-id"
	ownerUID := "test-uid"

	config := &AppConfig{
		identityEndpoint: identityServerURL,
		secretName:       secretName,
		imageID:          imageID,
		ownerUID:         ownerUID,
		pvcSize:          100,
		volumePath:       fileName,
	}

	fmt.Println("server ", identityServerURL)
	populate(config)

	file, err := os.Open(fileName)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close() // Ensure the file is closed after reading

	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != "mock_data\n" {
		t.Errorf("Expected %s, got %s", "mock_data", string(content))
	}

	os.Remove(fileName)
}
