/*
Copyright 2026 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/kubev2v/vm-migration-detective/pkg/vmdetect"
)

const listenAddr = ":8080"

// resultServer is a short-lived HTTP server that exposes the deep-inspection
// result once detection is complete.  The server shuts itself down after
// /results has been served once or on error.
type resultServer struct {
	mu       sync.Mutex
	result   *vmdetect.DetectResult
	detectErr error
	once     sync.Once
	shutdown chan struct{}
}

func newResultServer() *resultServer {
	return &resultServer{
		shutdown: make(chan struct{}),
	}
}

// setResult is called from the detection goroutine once vmdetect.Detect returns.
func (s *resultServer) setResult(result *vmdetect.DetectResult, err error) {
	s.mu.Lock()
	s.result = result
	s.detectErr = err
	s.mu.Unlock()
}

func (s *resultServer) isReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result != nil || s.detectErr != nil
}

func (s *resultServer) triggerShutdown() {
	s.once.Do(func() { close(s.shutdown) })
}

// handleHealthz is the liveness probe handler — always returns 200.
func (s *resultServer) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintln(w, "ok")
}

// handleReady returns 200 when detection is complete, 503 while it is still
// running.
func (s *resultServer) handleReady(w http.ResponseWriter, _ *http.Request) {
	if !s.isReady() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintln(w, "not ready")
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintln(w, "ok")
}

// handleResults serves the DetectResult as JSON.  Returns 503 while detection
// is still running, 500 when detection failed, or 200 with the full result.
// The server shuts down after the first successful 200 response.
func (s *resultServer) handleResults(w http.ResponseWriter, _ *http.Request) {
	if !s.isReady() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintln(w, "not ready")
		return
	}

	s.mu.Lock()
	result := s.result
	detectErr := s.detectErr
	s.mu.Unlock()

	if detectErr != nil {
		http.Error(w, detectErr.Error(), http.StatusInternalServerError)
		s.triggerShutdown()
		return
	}

	// try json encoding first, if that fails, don't return 200 ok but 500 internal server error
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		s.triggerShutdown()
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = buf.WriteTo(w)
	s.triggerShutdown()
}

// run starts the HTTP server on listenAddr and blocks until results have been
// served (or a fatal error occurs).  Returns the exit code the process should
// use (0 = success, 1 = detection failure).
func (s *resultServer) run() (exitCode int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/ready", s.handleReady)
	mux.HandleFunc("/results", s.handleResults)

	srv := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("deep-inspection: HTTP server error: %v\n", err)
			s.mu.Lock()
			s.detectErr = err
			s.mu.Unlock()
			s.triggerShutdown()
		}
	}()

	<-s.shutdown
	_ = srv.Shutdown(context.Background())

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.detectErr != nil {
		return 1
	}
	return 0
}
