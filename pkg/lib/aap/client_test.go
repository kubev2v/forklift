package aap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizePathPrefix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"/api/v2/", "/api/v2"},
		{"/api/v2", "/api/v2"},
		{"https://tower.example.com/api/v2/", "/api/v2"},
		{"v2", "/v2"},
	}
	for _, tc := range cases {
		got := normalizePathPrefix(tc.in)
		if got != tc.want {
			t.Errorf("normalizePathPrefix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestClientResolveAPIPrefixFromGetAPI(t *testing.T) {
	apiRootHits := 0
	jtHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/" || r.URL.Path == "/api":
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", r.Method)
			}
			apiRootHits++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"current_version": "/api/v2/"}`))
		case r.URL.Path == "/api/v2/job_templates/":
			jtHits++
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"count": 0, "next": null, "previous": null, "results": []}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	cl := NewClient(srv.URL, "tok", 0)
	_, err := cl.ListJobTemplates(context.Background(), 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if apiRootHits != 1 {
		t.Fatalf("expected exactly one GET /api, got %d", apiRootHits)
	}
	if jtHits != 1 {
		t.Fatalf("expected one GET to discovered job_templates path, got %d", jtHits)
	}
}

func TestWithPathPrefixSkipsDiscovery(t *testing.T) {
	called := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		if r.URL.Path == "/static/job_templates/" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"count": 0, "next": null, "previous": null, "results": []}`))
			return
		}
		if r.URL.Path == "/api/" {
			_, _ = w.Write([]byte(`{"current_version": "/nope"}`))
			return
		}
		t.Fatalf("unexpected: %s", r.URL.Path)
	}))
	defer srv.Close()

	cl := NewClient(srv.URL, "tok", 0, WithPathPrefix("/static"))
	_, err := cl.ListJobTemplates(context.Background(), 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	// /api/ should not be called when prefix is set manually; only /static/job_templates.
	if called != 1 {
		t.Fatalf("expected 1 request (no GET /api), got %d", called)
	}
}
