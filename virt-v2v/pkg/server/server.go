package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/konveyor/forklift-controller/virt-v2v/pkg/global"
	"github.com/konveyor/forklift-controller/virt-v2v/pkg/utils"
)

var (
	server *http.Server
)

// Start creates a webserver which is exposing information about the guest.
// The controller is periodically trying to request the server to get the information.
// This information is later used in the vm creation step such as the firmware for the OVA or
// Operating System for the VM creation.
func Start() error {
	http.HandleFunc("/ovf", ovfHandler)
	http.HandleFunc("/inspection", inspectorHandler)
	http.HandleFunc("/shutdown", shutdownHandler)
	server = &http.Server{Addr: ":8080"}

	fmt.Println("Starting server on :8080")
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("Error starting server: %v\n", err)
		return err
	}
	return nil
}

func ovfHandler(w http.ResponseWriter, r *http.Request) {
	xmlFilePath, err := GetDomainFile(global.DIR, "xml")
	if err != nil {
		fmt.Println("Error getting XML file:", err)
	}
	xmlData, err := utils.ReadXMLFile(xmlFilePath)
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

func inspectorHandler(w http.ResponseWriter, r *http.Request) {
	xmlData, err := utils.ReadXMLFile(global.INSPECTION)
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

func shutdownHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Shutdown request received. Shutting down server.")
	w.WriteHeader(http.StatusNoContent)
	if err := server.Shutdown(context.Background()); err != nil {
		fmt.Printf("error shutting down server: %v\n", err)
	}
}

func GetDomainFile(dir, fileExtension string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("%s.%s", os.Getenv("V2V_vmName"), fileExtension)))
	if err != nil {
		return "", err
	}
	if len(files) > 0 {
		return files[0], nil
	}
	return "", fmt.Errorf("XML file was not found")
}
