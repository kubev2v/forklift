package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
)

var (
	server   *http.Server
	warnings []Warning
)

// Warning represents a non-fatal issue that occurred during migration
type Warning struct {
	Reason  string `json:"reason"`  // Short reason code (e.g., "CustomizationFailed")
	Message string `json:"message"` // Detailed message
}

type Server struct {
	AppConfig *config.AppConfig
}

// AddWarning adds a warning that will be exposed via the /warnings endpoint
func AddWarning(warning Warning) {
	warnings = append(warnings, warning)
}

// Start creates a webserver which is exposing information about the guest.
// The controller is periodically trying to request the server to get the information.
// This information is later used in the vm creation step such as the firmware for the OVA or
// Operating System for the VM creation.
func (s Server) Start() error {
	http.HandleFunc("/vm", s.vmHandler)
	http.HandleFunc("/inspection", s.inspectorHandler)
	http.HandleFunc("/warnings", s.warningsHandler)
	http.HandleFunc("/shutdown", s.shutdownHandler)
	server = &http.Server{Addr: ":8080"}

	fmt.Println("Starting server on :8080")
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("Error starting server: %v\n", err)
		return err
	}
	return nil
}

func (s Server) vmHandler(w http.ResponseWriter, r *http.Request) {
	yamlFilePath, err := s.getVmYamlFile(s.AppConfig.Workdir)
	if yamlFilePath == "" {
		// For in-place conversions (especially disk mode used by EC2), virt-v2v-in-place
		// doesn't generate a YAML output file. This is expected behavior.
		// Return 204 No Content to indicate there's no VM config to return.
		if s.AppConfig.IsInPlace {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		fmt.Println("Error: YAML file path is empty.")
		http.Error(w, "YAML file path is empty", http.StatusInternalServerError)
		return
	}
	if err != nil {
		fmt.Println("Error getting XML file:", err)
	}
	yamlData, err := os.ReadFile(yamlFilePath)
	if err != nil {
		fmt.Printf("Error reading YAML file: %v\n", err)
		http.Error(w, "Error reading YAML file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/yaml")
	_, err = w.Write(yamlData)
	if err == nil {
		w.WriteHeader(http.StatusOK)
	} else {
		fmt.Printf("Error writing response: %v\n", err)
		http.Error(w, "Error writing response", http.StatusInternalServerError)
	}
}

func (s Server) inspectorHandler(w http.ResponseWriter, r *http.Request) {
	xmlData, err := os.ReadFile(s.AppConfig.InspectionOutputFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	_, err = w.Write(xmlData)
	if err == nil {
		w.WriteHeader(http.StatusOK)
	} else {
		fmt.Printf("Error writing response: %v\n", err)
		http.Error(w, "Error writing response", http.StatusInternalServerError)
	}
}

func (s Server) warningsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if len(warnings) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	warningsJSON, err := json.Marshal(warnings)
	if err != nil {
		fmt.Printf("Error marshaling warnings: %v\n", err)
		http.Error(w, "Error marshaling warnings", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(warningsJSON); err != nil {
		fmt.Printf("Error writing warnings response: %v\n", err)
	}
}

func (s Server) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Shutdown request received. Shutting down server.")
	w.WriteHeader(http.StatusNoContent)
	if err := server.Shutdown(context.Background()); err != nil {
		fmt.Printf("error shutting down server: %v\n", err)
	}
}

func (s Server) getVmYamlFile(dir string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return "", err
	}
	if len(files) > 0 {
		return files[0], nil
	}
	return "", fmt.Errorf("XML file was not found")
}
