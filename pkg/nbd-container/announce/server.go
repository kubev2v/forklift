// Package announce serves a mutual-TLS HTTPS endpoint that lists the exported disks.
package announce

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kubev2v/forklift/pkg/nbd-container/runner"
)

// Server announces the current set of exports over HTTPS with mutual TLS.
type Server struct {
	http *http.Server

	mu      sync.RWMutex
	exports []runner.Export
}

// New builds an HTTPS server that presents ServerCert/ServerKey from certsDir
// and requires a client certificate signed by CACert (mutual TLS), matching
// the nbdkit containers.
func New(addr, certsDir string, exports []runner.Export) (*Server, error) {
	cert, err := tls.LoadX509KeyPair(
		filepath.Join(certsDir, ServerCert),
		filepath.Join(certsDir, ServerKey),
	)
	if err != nil {
		return nil, fmt.Errorf("loading server keypair: %w", err)
	}

	caPEM, err := os.ReadFile(filepath.Join(certsDir, CACert))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", CACert, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("%s contained no valid certificates", CACert)
	}

	s := &Server{exports: exports}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /disks", s.handleDisks)
	mux.HandleFunc("GET /", s.handleDisks)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	s.http = &http.Server{
		Addr:    addr,
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ClientCAs:    pool,
		},
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s, nil
}

// SetExports replaces the announced set (useful if discovery is ever re-run).
func (s *Server) SetExports(exports []runner.Export) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exports = exports
}

func (s *Server) handleDisks(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	exports := s.exports
	s.mu.RUnlock()
	if exports == nil {
		exports = []runner.Export{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(exports); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ListenAndServe starts serving. The key/cert are already loaded into TLSConfig, so
// the empty string arguments to ServeTLS are intentional.
func (s *Server) ListenAndServe() error {
	ln, err := tls.Listen("tcp", s.http.Addr, s.http.TLSConfig)
	if err != nil {
		return err
	}
	return s.http.Serve(ln)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
