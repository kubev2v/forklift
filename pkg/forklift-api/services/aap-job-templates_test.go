package services

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestServeAAPJobTemplates(t *testing.T) {
	t.Parallel()

	old := aapProxyHTTPClient
	t.Cleanup(func() { aapProxyHTTPClient = old })
	aapProxyHTTPClient = &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != aapJobTemplatesPath {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer the-token" {
				t.Errorf("missing or wrong Authorization header: %q", r.Header.Get("Authorization"))
			}
			if got := r.URL.RawQuery; got != "page=2" {
				t.Errorf("query forwarded: got %q want page=2", got)
			}
			if r.URL.Scheme != "https" || r.URL.Host != "aap.example.com" {
				t.Errorf("unexpected URL: %s", r.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"count":0}`)),
			}, nil
		}),
	}

	q := url.Values{}
	q.Set("url", "https://aap.example.com")
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

func TestServeAAPJobTemplates_HTTPSchemeRejected(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/aap/job-templates?url=http://aap.example.com", nil)
	req.Header.Set(headerAAPToken, "tok")
	rec := httptest.NewRecorder()
	serveAAPJobTemplates(rec, req, fake.NewClientBuilder().Build())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestServeAAPJobTemplates_LoopbackRejected(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/aap/job-templates?url=https://127.0.0.1/", nil)
	req.Header.Set(headerAAPToken, "tok")
	rec := httptest.NewRecorder()
	serveAAPJobTemplates(rec, req, fake.NewClientBuilder().Build())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAAPJobTemplatesTargetURL(t *testing.T) {
	t.Parallel()
	base, err := url.Parse("https://aap.example.com")
	if err != nil {
		t.Fatal(err)
	}
	got, err := aapJobTemplatesTargetURL(base, "page=1")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://aap.example.com/api/controller/v2/job_templates/?page=1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAAPJobTemplatesTargetURL_WithQueryInBaseURLString(t *testing.T) {
	t.Parallel()
	// Caller must not put ?query inside url=; if they do, upstreamQuery overwrites RawQuery on the copy.
	base, err := url.Parse("https://aap.example.com/")
	if err != nil {
		t.Fatal(err)
	}
	got, err := aapJobTemplatesTargetURL(base, "page=2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "page=2") || !strings.Contains(got, "/api/controller/v2/job_templates/") {
		t.Fatalf("got %q", got)
	}
}
