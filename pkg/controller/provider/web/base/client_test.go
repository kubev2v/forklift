package base

import (
	"testing"
)

func TestRestClientURL_UsesConfiguredScheme(t *testing.T) {
	tests := []struct {
		name   string
		scheme string
		host   string
		port   int
		path   string
		want   string
	}{
		{
			name:   "http scheme",
			scheme: "http",
			host:   "api.example.com",
			port:   8080,
			path:   "/api/v1/vms",
			want:   "http://api.example.com:8080/api/v1/vms",
		},
		{
			name:   "https scheme",
			scheme: "https",
			host:   "api.example.com",
			port:   8443,
			path:   "/api/v1/providers",
			want:   "https://api.example.com:8443/api/v1/providers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore globals to avoid cross-test interference.
			origHost, origPort, origScheme := Settings.Inventory.Host, Settings.Inventory.Port, Settings.Inventory.Scheme
			t.Cleanup(func() {
				Settings.Inventory.Host = origHost
				Settings.Inventory.Port = origPort
				Settings.Inventory.Scheme = origScheme
			})

			// Set up inventory settings
			Settings.Inventory.Host = tt.host
			Settings.Inventory.Port = tt.port
			Settings.Inventory.Scheme = tt.scheme

			c := &RestClient{Host: ""} // Empty Host forces use of Settings
			got := c.url(tt.path)

			if got != tt.want {
				t.Fatalf("expected URL %q, got %q", tt.want, got)
			}
		})
	}
}
