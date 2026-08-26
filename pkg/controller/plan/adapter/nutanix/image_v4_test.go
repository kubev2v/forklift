package nutanix

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateRedirectToken(t *testing.T) {
	valid := "NTNX_IGW_SESSION=" + strings.Repeat("a", 1450)
	cases := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"valid live size", valid, false},
		{"empty", "", false},
		{"newline", "NTNX_IGW_SESSION=foo\nbar", true},
		{"carriage return", "NTNX_IGW_SESSION=foo\rbar", true},
		{"nul", "NTNX_IGW_SESSION=foo\x00bar", true},
		{"too long", strings.Repeat("x", redirectTokenMaxLen+1), true},
		{"max length", strings.Repeat("x", redirectTokenMaxLen), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateRedirectToken(c.token)
			if (err != nil) != c.wantErr {
				t.Fatalf("validateRedirectToken(%q): err=%v, wantErr=%v", c.name, err, c.wantErr)
			}
		})
	}
}

func TestResolveImageV4DownloadURL_RejectsOversizedToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/clusters/list"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entities":[]}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/file"):
			w.Header().Set("Location", "https://pe.example/entity_download")
			w.Header().Set("X-Redirect-Token", strings.Repeat("x", redirectTokenMaxLen+1))
			w.WriteHeader(http.StatusFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newConnectedTestClient(t, server.URL)
	_, _, err := client.resolveImageV4DownloadURL("image-1")
	if err == nil {
		t.Fatal("expected error for redirect token exceeding maximum length")
	}
}
