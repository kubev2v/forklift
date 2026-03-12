package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var log = logging.WithName("hyperv|smb-mount")

const defaultCatalogPath = "/hyperv"

type validateDisksRequest struct {
	Paths []string `json:"paths"`
}

type validateDisksResponse struct {
	Missing []string `json:"missing"`
}

func validateDisksHandler(catalogPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req validateDisksRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var missing []string
		for _, p := range req.Paths {
			if !strings.HasPrefix(p, catalogPath) {
				log.Info("Rejected path outside catalog root", "path", p, "catalogPath", catalogPath)
				missing = append(missing, p)
				continue
			}
			if _, err := os.Stat(p); os.IsNotExist(err) {
				missing = append(missing, p)
			}
		}

		resp := validateDisksResponse{Missing: missing}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func main() {
	catalogPath := os.Getenv("CATALOG_PATH")
	if catalogPath == "" {
		catalogPath = defaultCatalogPath
	}

	log.Info("HyperV provider-server started", "catalogPath", catalogPath)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	mux.HandleFunc("/validate-disks", validateDisksHandler(catalogPath))

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		log.Info("Listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(err, "HTTP server error")
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh

	log.Info("Received signal, shutting down", "signal", sig)
	_ = server.Close()
}
