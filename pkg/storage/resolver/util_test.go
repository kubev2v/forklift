package resolver

import "testing"

func TestEnsureScheme(t *testing.T) {
	tests := []struct{ host, want string }{
		{"10.46.246.90", "https://10.46.246.90"},
		{"host:8080", "https://host:8080"},
		{"https://host", "https://host"},
		{"http://host:8080", "http://host:8080"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := EnsureScheme(tt.host); got != tt.want {
			t.Errorf("EnsureScheme(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}
