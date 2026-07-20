package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// Mock Nutanix Prism API server for testing
// This server simulates the Nutanix Prism v3 API using the testdata JSON files

var (
	port     = flag.String("port", "9440", "Port to listen on (default: 9440, same as Prism)")
	username = flag.String("user", "admin", "Username for basic auth")
	password = flag.String("password", "password", "Password for basic auth")
	certFile = flag.String("cert", "server-cert.pem", "TLS certificate file")
	keyFile  = flag.String("key", "server-key.pem", "TLS key file")
)

// checkAuth validates basic authentication
func checkAuth(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false
	}

	// Extract credentials
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return false
	}

	credentials := strings.SplitN(string(decoded), ":", 2)
	if len(credentials) != 2 {
		return false
	}

	return credentials[0] == *username && credentials[1] == *password
}

// sendJSON sends a JSON response
func sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// sendFile sends a JSON file as response
func sendFile(w http.ResponseWriter, filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
	return nil
}

// handleClusters handles /api/nutanix/v3/clusters/list
func handleClusters(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		sendJSON(w, map[string]string{"error": "unauthorized"})
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := sendFile(w, "clusters_list.json"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		sendJSON(w, map[string]string{"error": err.Error()})
	}
}

// handleHosts handles /api/nutanix/v3/hosts/list
func handleHosts(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		sendJSON(w, map[string]string{"error": "unauthorized"})
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := sendFile(w, "hosts_list.json"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		sendJSON(w, map[string]string{"error": err.Error()})
	}
}

// handleVMs handles /api/nutanix/v3/vms/list
func handleVMs(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		sendJSON(w, map[string]string{"error": "unauthorized"})
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := sendFile(w, "vms_list.json"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		sendJSON(w, map[string]string{"error": err.Error()})
	}
}

// handleSubnets handles /api/nutanix/v3/subnets/list
func handleSubnets(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		sendJSON(w, map[string]string{"error": "unauthorized"})
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := sendFile(w, "subnets_list.json"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		sendJSON(w, map[string]string{"error": err.Error()})
	}
}

// handlePrismCentral handles /api/nutanix/v3/prism_central
func handlePrismCentral(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		sendJSON(w, map[string]string{"error": "unauthorized"})
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	sendJSON(w, map[string]interface{}{
		"resources": map[string]string{
			"version": "mock-pc",
		},
	})
}

// handleStorageContainersV2 handles /api/nutanix/v2.0/storage_containers
func handleStorageContainersV2(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		sendJSON(w, map[string]string{"error": "unauthorized"})
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := sendFile(w, "storage_containers_v2_list.json"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		sendJSON(w, map[string]string{"error": err.Error()})
	}
}

// handleStorageContainersV4 handles /api/clustermgmt/v4.1/config/storage-containers
func handleStorageContainersV4(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		sendJSON(w, map[string]string{"error": "unauthorized"})
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := sendFile(w, "storage_containers_v4_list.json"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		sendJSON(w, map[string]string{"error": err.Error()})
	}
}

// handleStorageContainers handles legacy v3 storage list with 404.
func handleStorageContainers(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		sendJSON(w, map[string]string{"error": "unauthorized"})
		return
	}

	w.WriteHeader(http.StatusNotFound)
	sendJSON(w, map[string]string{"error": "v3 storage_containers/list is not available"})
}

// handleImages handles /api/nutanix/v3/images/list
func handleImages(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		sendJSON(w, map[string]string{"error": "unauthorized"})
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := sendFile(w, "images_list.json"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		sendJSON(w, map[string]string{"error": err.Error()})
	}
}

// handleRoot handles /api/nutanix/v3 (basic version endpoint)
func handleRoot(w http.ResponseWriter, r *http.Request) {
	if !checkAuth(r) {
		w.WriteHeader(http.StatusUnauthorized)
		sendJSON(w, map[string]string{"error": "unauthorized"})
		return
	}

	sendJSON(w, map[string]string{
		"api_version": "3.1",
		"server_name": "Mock Nutanix Prism Central",
	})
}

func main() {
	flag.Parse()

	// Print banner
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║   Mock Nutanix Prism API Server for Testing           ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Listening on: http://localhost:%s\n", *port)
	fmt.Printf("Username: %s\n", *username)
	fmt.Printf("Password: %s\n", *password)
	fmt.Println()
	fmt.Println("API Endpoints:")
	fmt.Println("  POST /api/nutanix/v3/clusters/list")
	fmt.Println("  POST /api/nutanix/v3/hosts/list")
	fmt.Println("  POST /api/nutanix/v3/vms/list")
	fmt.Println("  POST /api/nutanix/v3/subnets/list")
	fmt.Println("  GET  /api/nutanix/v3/prism_central")
	fmt.Println("  GET  /api/nutanix/v2.0/storage_containers")
	fmt.Println("  GET  /api/clustermgmt/v4.1/config/storage-containers")
	fmt.Println("  POST /api/nutanix/v3/images/list")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Register handlers
	http.HandleFunc("/api/nutanix/v3", handleRoot)
	http.HandleFunc("/api/nutanix/v3/", handleRoot)
	http.HandleFunc("/api/nutanix/v3/clusters/list", handleClusters)
	http.HandleFunc("/api/nutanix/v3/hosts/list", handleHosts)
	http.HandleFunc("/api/nutanix/v3/vms/list", handleVMs)
	http.HandleFunc("/api/nutanix/v3/subnets/list", handleSubnets)
	http.HandleFunc("/api/nutanix/v3/prism_central", handlePrismCentral)
	http.HandleFunc("/api/nutanix/v2.0/storage_containers", handleStorageContainersV2)
	http.HandleFunc("/api/clustermgmt/v4.1/config/storage-containers", handleStorageContainersV4)
	http.HandleFunc("/api/nutanix/v3/storage_containers/list", handleStorageContainers)
	http.HandleFunc("/api/nutanix/v3/images/list", handleImages)

	// Start server
	addr := ":" + *port
	log.Printf("Starting HTTPS server on %s...\n", addr)
	log.Printf("Using certificate: %s, key: %s\n", *certFile, *keyFile)
	if err := http.ListenAndServeTLS(addr, *certFile, *keyFile, nil); err != nil {
		log.Fatal(err)
	}
}
