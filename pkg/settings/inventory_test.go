package settings

import (
	"os"
	"testing"
)

func TestInventoryLoad_SchemeNormalization(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		unset bool
		want  string
	}{
		{"unset => https", "", true, "https"},
		{"http", "http", false, "http"},
		{"HTTPS upper", "HTTPS", false, "https"},
		{"trimmed http", "  http  ", false, "http"},
		{"invalid => https", "ftp", false, "https"},
		{"empty string => https", "   ", false, "https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unset {
				_ = os.Unsetenv(Scheme)
			} else {
				t.Setenv(Scheme, tt.env)
			}
			var inv Inventory
			if err := inv.Load(); err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if inv.Scheme != tt.want {
				t.Fatalf("Scheme = %q, want %q", inv.Scheme, tt.want)
			}
		})
	}
}
