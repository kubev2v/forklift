package services

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServeAAPJobTemplates(t *testing.T) {
	t.Parallel()

	aap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/controller/v2/job_templates/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer the-token" {
			t.Errorf("missing or wrong Authorization header: %q", r.Header.Get("Authorization"))
		}
		if got := r.URL.RawQuery; got != "page=2" {
			t.Errorf("query forwarded: got %q want page=2", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"count":0}`))
	}))
	defer aap.Close()

	u, err := url.Parse(aap.URL)
	if err != nil {
		t.Fatal(err)
	}
	q := url.Values{}
	q.Set("url", u.Scheme+"://"+u.Host)
	q.Set("page", "2")
	req := httptest.NewRequest(http.MethodGet, "/aap/job-templates?"+q.Encode(), nil)
	req.Header.Set(headerAAPToken, "the-token")

	rec := httptest.NewRecorder()
	serveAAPJobTemplates(rec, req, fake.NewClientBuilder().Build())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("Content-Type = %q", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != `{"count":0}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestServeAAPJobTemplates_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "/aap/job-templates?url=https://aap.example.com", nil)
	req.Header.Set(headerAAPToken, "x")
	rec := httptest.NewRecorder()
	serveAAPJobTemplates(rec, req, fake.NewClientBuilder().Build())
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", rec.Code)
	}
}

func TestServeAAPJobTemplates_MissingToken(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/aap/job-templates?url=https://aap.example.com", nil)
	rec := httptest.NewRecorder()
	serveAAPJobTemplates(rec, req, fake.NewClientBuilder().Build())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}
