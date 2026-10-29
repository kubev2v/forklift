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
				prev, present := os.LookupEnv(Scheme)
				t.Cleanup(func() {
					if present {
						if err := os.Setenv(Scheme, prev); err != nil {
							t.Errorf("failed to restore env %s: %v", Scheme, err)
						}
					} else {
						if err := os.Unsetenv(Scheme); err != nil {
							t.Errorf("failed to unset env %s: %v", Scheme, err)
						}
					}
				})
				if err := os.Unsetenv(Scheme); err != nil {
					t.Fatalf("failed to unset env %s: %v", Scheme, err)
				}
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

func TestInventoryLoad_VsphereExcludedVMProperties(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		unset bool
		want  []string
	}{
		{"unset => empty slice", "", true, []string{}},
		{"empty string => empty slice", "", false, []string{}},
		{"whitespace string => empty slice", "   ,  ", false, []string{}},
		{"single property", "customValue", false, []string{"customValue"}},
		{"multiple properties comma separated", "customValue,availableField", false, []string{"customValue", "availableField"}},
		{"with spaces around properties", " customValue , availableField ", false, []string{"customValue", "availableField"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unset {
				prev, present := os.LookupEnv(VsphereExcludedVMPropertiesEv)
				t.Cleanup(func() {
					if present {
						if err := os.Setenv(VsphereExcludedVMPropertiesEv, prev); err != nil {
							t.Errorf("failed to restore env %s: %v", VsphereExcludedVMPropertiesEv, err)
						}
					} else {
						if err := os.Unsetenv(VsphereExcludedVMPropertiesEv); err != nil {
							t.Errorf("failed to unset env %s: %v", VsphereExcludedVMPropertiesEv, err)
						}
					}
				})
				if err := os.Unsetenv(VsphereExcludedVMPropertiesEv); err != nil {
					t.Fatalf("failed to unset env %s: %v", VsphereExcludedVMPropertiesEv, err)
				}
			} else {
				t.Setenv(VsphereExcludedVMPropertiesEv, tt.env)
			}
			var inv Inventory
			if err := inv.Load(); err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if len(inv.VsphereExcludedVMProperties) != len(tt.want) {
				t.Fatalf("VsphereExcludedVMProperties length = %d, want %d", len(inv.VsphereExcludedVMProperties), len(tt.want))
			}
			for i, v := range tt.want {
				if inv.VsphereExcludedVMProperties[i] != v {
					t.Fatalf("VsphereExcludedVMProperties[%d] = %q, want %q", i, inv.VsphereExcludedVMProperties[i], v)
				}
			}
		})
	}
}
