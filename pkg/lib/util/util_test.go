package util

import (
	"testing"
)

// TestExtractServerName tests the extractServerName function with various address formats.
func TestExtractServerName(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected string
	}{
		{
			name:     "IPv4 with port",
			address:  "192.168.1.1:443",
			expected: "192.168.1.1",
		},
		{
			name:     "IPv4 without port",
			address:  "192.168.1.1",
			expected: "192.168.1.1",
		},
		{
			name:     "Hostname with port",
			address:  "example.com:443",
			expected: "example.com",
		},
		{
			name:     "Hostname without port",
			address:  "example.com",
			expected: "example.com",
		},
		{
			name:     "IPv6 with brackets and port",
			address:  "[2620:52:0:2ef8:f2d4:e2ff:feea:4b6c]:443",
			expected: "2620:52:0:2ef8:f2d4:e2ff:feea:4b6c",
		},
		{
			name:     "IPv6 with brackets, no port",
			address:  "[2620:52:0:2ef8:f2d4:e2ff:feea:4b6c]",
			expected: "[2620:52:0:2ef8:f2d4:e2ff:feea:4b6c]",
		},
		{
			name:     "IPv6 loopback with port",
			address:  "[::1]:443",
			expected: "::1",
		},
		{
			name:     "IPv6 loopback without port",
			address:  "[::1]",
			expected: "[::1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractServerName(tt.address)
			if result != tt.expected {
				t.Errorf("extractServerName(%q) = %q, want %q", tt.address, result, tt.expected)
			}
		})
	}
}
