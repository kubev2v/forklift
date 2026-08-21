package nutanix

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
)

func newTestClient(url string) *Client {
	return &Client{
		URL: url,
		Secret: &core.Secret{
			Data: map[string][]byte{
				"user":     []byte("admin"),
				"password": []byte("password"),
			},
		},
		Timeout: 5 * time.Second,
		Log:     logging.WithName("test"),
	}
}

// TestBasicAuth verifies the Basic Auth encoding.
func TestBasicAuth(t *testing.T) {
	result := basicAuth("admin", "password")
	// base64("admin:password")
	expected := "YWRtaW46cGFzc3dvcmQ="
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

// TestConnect verifies that Connect authenticates and is idempotent.
func TestConnect(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		if r.Header.Get("Authorization") == "" {
			t.Error("expected Authorization header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"entities":[]}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	if status, err := client.Connect(); err != nil || status != http.StatusOK {
		t.Fatalf("unexpected result: status=%d err=%v", status, err)
	}
	if requestCount.Load() != 1 {
		t.Fatalf("expected 1 connectivity probe request, got %d", requestCount.Load())
	}

	// Second call should be a no-op (already connected).
	if status, err := client.Connect(); err != nil || status != http.StatusOK {
		t.Fatalf("unexpected result on second connect: status=%d err=%v", status, err)
	}
	if requestCount.Load() != 1 {
		t.Fatalf("expected no additional requests on repeated connect, got %d total", requestCount.Load())
	}
}

// TestConnectFailure verifies a failed connectivity probe surfaces an error
// and leaves the client able to retry on a subsequent call.
func TestConnectFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	if _, err := client.Connect(); err == nil {
		t.Fatal("expected an error from a failed connectivity probe")
	}
	if _, err := client.Connect(); err == nil {
		t.Fatal("expected a repeated Connect() to fail while the probe keeps failing")
	}
}

// TestGetPost verifies Get and Post issue authenticated requests and decode
// JSON responses.
func TestGetPost(t *testing.T) {
	var sawRequestID bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/clusters/list") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
			return
		}
		if r.Method == http.MethodPost {
			if id := r.Header.Get("NTNX-Request-Id"); id == "" {
				t.Errorf("POST missing NTNX-Request-Id")
			} else {
				sawRequestID = true
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"method":"get"}`))
		case http.MethodPost:
			_, _ = w.Write([]byte(`{"method":"post"}`))
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	var getResult map[string]interface{}
	if status, err := client.Get(server.URL+"/thing", &getResult); err != nil || status != http.StatusOK {
		t.Fatalf("Get failed: status=%d err=%v", status, err)
	}
	if getResult["method"] != "get" {
		t.Fatalf("unexpected Get result: %v", getResult)
	}

	var postResult map[string]interface{}
	if status, err := client.Post(server.URL+"/thing", map[string]interface{}{"a": 1}, &postResult); err != nil || status != http.StatusOK {
		t.Fatalf("Post failed: status=%d err=%v", status, err)
	}
	if postResult["method"] != "post" {
		t.Fatalf("unexpected Post result: %v", postResult)
	}
	if !sawRequestID {
		t.Fatal("expected Post to send NTNX-Request-Id")
	}
}

// TestGetNoRedirect verifies GetNoRedirect returns a redirect response's
// status and headers as-is, rather than transparently following it like
// Get does -- needed for APIs (e.g. Nutanix's v4 image download) that put
// caller-specific instructions in a redirect's headers that a normal
// redirect-following client would never see.
func TestGetNoRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/nutanix/v3/clusters/list" {
			// Connectivity probe issued by the first call to Connect().
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
			return
		}
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "http://example.invalid/final")
			w.Header().Set("X-Redirect-Token", "some-token")
			w.WriteHeader(http.StatusFound)
			return
		}
		t.Errorf("expected the redirect not to be followed, got a request for %s", r.URL.Path)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	status, header, err := client.GetNoRedirect(server.URL + "/redirect")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, status)
	}
	if header.Get("Location") != "http://example.invalid/final" {
		t.Fatalf("unexpected Location header: %v", header.Get("Location"))
	}
	if header.Get("X-Redirect-Token") != "some-token" {
		t.Fatalf("unexpected X-Redirect-Token header: %v", header.Get("X-Redirect-Token"))
	}
}

// TestPut verifies Put sends the full body and decodes the response.
func TestPut(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/nutanix/v3/clusters/list" {
			// Connectivity probe issued by the first call to Connect().
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
			return
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":{"state":"PENDING"}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	body := map[string]interface{}{
		"spec": map[string]interface{}{
			"resources": map[string]interface{}{"power_state": "OFF"},
		},
	}
	var result map[string]interface{}
	status, err := client.Put(server.URL+"/vms/uuid", body, &result)
	if err != nil || status != http.StatusOK {
		t.Fatalf("Put failed: status=%d err=%v", status, err)
	}
	if receivedBody["spec"] == nil {
		t.Fatalf("server did not receive expected body: %v", receivedBody)
	}
	statusField, _ := result["status"].(map[string]interface{})
	if statusField["state"] != "PENDING" {
		t.Fatalf("unexpected Put result: %v", result)
	}
}

// TestDelete verifies Delete issues a DELETE with no body and decodes any
// response (e.g. a task reference).
func TestDelete(t *testing.T) {
	var sawBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/nutanix/v3/clusters/list" {
			// Connectivity probe issued by the first call to Connect().
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
			return
		}
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("read body failed: %v", readErr)
		}
		sawBody = body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":{"execution_context":{"task_uuid":"abc"}}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	var result map[string]interface{}
	status, err := client.Delete(server.URL+"/images/uuid", &result)
	if err != nil || status != http.StatusOK {
		t.Fatalf("Delete failed: status=%d err=%v", status, err)
	}
	if len(sawBody) != 0 {
		t.Fatalf("expected no request body, got %v", sawBody)
	}
	statusField, _ := result["status"].(map[string]interface{})
	execCtx, _ := statusField["execution_context"].(map[string]interface{})
	if execCtx["task_uuid"] != "abc" {
		t.Fatalf("unexpected Delete result: %v", result)
	}
}

// TestListAllPaginates verifies ListAll pages through total_matches.
func TestListAllPaginates(t *testing.T) {
	const total = 5
	const pageSize = 2
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body failed: %v", err)
			return
		}
		offsetValue, ok := body["offset"].(float64)
		if !ok {
			t.Errorf("expected numeric offset in request body, got %T", body["offset"])
			return
		}
		offset := int(offsetValue)

		remaining := total - offset
		if remaining > pageSize {
			remaining = pageSize
		}
		entities := make([]struct {
			Metadata struct {
				UUID string `json:"uuid"`
			} `json:"metadata"`
		}, remaining)
		for i := range entities {
			entities[i].Metadata.UUID = fmt.Sprintf("%d", offset+i)
		}

		resp := map[string]interface{}{
			"entities": entities,
			"metadata": map[string]interface{}{"total_matches": total},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	entities, err := ListAllV3[struct {
		Metadata struct {
			UUID string `json:"uuid"`
		} `json:"metadata"`
	}](client, "vm", pageSize, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != total {
		t.Fatalf("expected %d entities, got %d", total, len(entities))
	}
	// 1 connectivity probe (from Connect) + 3 list pages (2+2+1).
	if requests.Load() != 4 {
		t.Fatalf("expected 4 requests, got %d", requests.Load())
	}
}

// TestListAllV4Paginates verifies ListAllV4 pages via $page/$limit and
// metadata.totalAvailableResults.
func TestListAllV4Paginates(t *testing.T) {
	const total = 3
	const pageSize = 2

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/nutanix/v3/clusters/list" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
			return
		}
		page := r.URL.Query().Get("$page")
		var offset int
		if page == "1" {
			offset = pageSize
		}
		remaining := total - offset
		if remaining > pageSize {
			remaining = pageSize
		}
		if remaining < 0 {
			remaining = 0
		}
		data := make([]struct {
			ExtID string `json:"extId"`
		}, remaining)
		for i := range data {
			data[i].ExtID = fmt.Sprintf("%d", offset+i)
		}
		resp := map[string]interface{}{
			"data":     data,
			"metadata": map[string]interface{}{"totalAvailableResults": total},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	entities, err := ListAllV4[struct {
		ExtID string `json:"extId"`
	}](client, "/api/clustermgmt/v4.0/config/storage-containers", pageSize)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != total {
		t.Fatalf("expected %d entities, got %d", total, len(entities))
	}
}

// TestListAllV4StopsOnEmptyPage verifies pagination stops on an empty page
// even if totalAvailableResults is never satisfied.
func TestListAllV4StopsOnEmptyPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/nutanix/v3/clusters/list" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
			return
		}
		resp := map[string]interface{}{
			"data":     []interface{}{},
			"metadata": map[string]interface{}{"totalAvailableResults": 100},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	entities, err := ListAllV4[struct{}](client, "/api/clustermgmt/v4.0/config/storage-containers", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 0 {
		t.Fatalf("expected no entities, got %d", len(entities))
	}
}

// TestListAllV3PaginationLimit verifies ListAllV3 stops when a server keeps
// returning the same page instead of honouring offset.
func TestListAllV3PaginationLimit(t *testing.T) {
	savedMaxPages := maxListPages
	maxListPages = 3
	t.Cleanup(func() {
		maxListPages = savedMaxPages
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entity := struct {
			Metadata struct {
				UUID string `json:"uuid"`
			} `json:"metadata"`
		}{}
		entity.Metadata.UUID = "same-page"
		resp := map[string]interface{}{
			"entities": []interface{}{entity},
			"metadata": map[string]interface{}{"total_matches": maxListPages + 10},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	_, err := ListAllV3[struct {
		Metadata struct {
			UUID string `json:"uuid"`
		} `json:"metadata"`
	}](client, "vm", 1, nil)
	if err == nil {
		t.Fatal("expected pagination limit error")
	}
}

// TestSendUsesFixedLengthBody verifies POST requests declare Content-Length
// rather than chunked encoding for JSON bodies.
func TestSendUsesFixedLengthBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/nutanix/v3/clusters/list" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
			return
		}
		if r.TransferEncoding != nil {
			t.Errorf("expected fixed-length body, got transfer encoding %v", r.TransferEncoding)
		}
		if r.ContentLength <= 0 {
			t.Errorf("expected positive ContentLength, got %d", r.ContentLength)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	var result map[string]interface{}
	if status, err := client.Post(server.URL+"/mutate", map[string]interface{}{"a": 1}, &result); err != nil || status != http.StatusOK {
		t.Fatalf("Post failed: status=%d err=%v", status, err)
	}
}
