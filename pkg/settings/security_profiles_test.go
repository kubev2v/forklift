package settings

import (
	"testing"

	core "k8s.io/api/core/v1"
)

// An unset type must stay unset, so the caller keeps its per-platform default.
func TestMigration_VirtV2vSeccompProfile(t *testing.T) {
	tests := []struct {
		name      string
		envType   string
		envPath   string
		wantType  string
		wantPath  string
		wantError bool
	}{
		{name: "unset keeps the platform default", wantType: "", wantPath: ""},
		{
			name:     "localhost without a path falls back to the shipped profile",
			envType:  string(core.SeccompProfileTypeLocalhost),
			wantType: string(core.SeccompProfileTypeLocalhost),
			wantPath: DefaultUnshareSeccompProfilePath,
		},
		{
			name:     "localhost with an explicit path",
			envType:  string(core.SeccompProfileTypeLocalhost),
			envPath:  "operator/mtv-unshare.json",
			wantType: string(core.SeccompProfileTypeLocalhost),
			wantPath: "operator/mtv-unshare.json",
		},
		{
			name:     "whitespace is trimmed",
			envType:  "  Localhost  ",
			envPath:  "  profiles/x.json  ",
			wantType: string(core.SeccompProfileTypeLocalhost),
			wantPath: "profiles/x.json",
		},
		{
			name:     "runtime default",
			envType:  string(core.SeccompProfileTypeRuntimeDefault),
			wantType: string(core.SeccompProfileTypeRuntimeDefault),
		},
		{
			name:     "unconfined",
			envType:  string(core.SeccompProfileTypeUnconfined),
			wantType: string(core.SeccompProfileTypeUnconfined),
		},
		{name: "unknown type", envType: "Localhost/profiles/x.json", wantError: true},
		{name: "lowercase type", envType: "localhost", wantError: true},
		{name: "path without a type", envPath: "profiles/unshare.json", wantError: true},
		{
			name:      "path with a non-localhost type",
			envType:   string(core.SeccompProfileTypeRuntimeDefault),
			envPath:   "profiles/unshare.json",
			wantError: true,
		},
		{
			name:      "absolute path",
			envType:   string(core.SeccompProfileTypeLocalhost),
			envPath:   "/var/lib/kubelet/seccomp/profiles/unshare.json",
			wantError: true,
		},
		{
			name:      "path escaping the seccomp root",
			envType:   string(core.SeccompProfileTypeLocalhost),
			envPath:   "../../../etc/passwd",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(VirtV2vSeccompProfileType, tt.envType)
			t.Setenv(VirtV2vSeccompProfilePath, tt.envPath)
			t.Setenv(VirtV2vAppArmorProfileType, "")
			t.Setenv(VirtV2vAppArmorProfilePath, "")

			r := &Migration{}
			err := r.loadVirtV2vSecurityProfiles()

			if tt.wantError {
				if err == nil {
					t.Fatalf("expected an error, got type=%q path=%q",
						r.VirtV2vSeccompProfileType, r.VirtV2vSeccompProfilePath)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.VirtV2vSeccompProfileType != tt.wantType {
				t.Errorf("type = %q, want %q", r.VirtV2vSeccompProfileType, tt.wantType)
			}
			if r.VirtV2vSeccompProfilePath != tt.wantPath {
				t.Errorf("path = %q, want %q", r.VirtV2vSeccompProfilePath, tt.wantPath)
			}
		})
	}
}

// An AppArmor name is a kernel profile, not a file, so a path-shaped value is
// legitimate here and must survive the traversal check that seccomp gets.
func TestMigration_VirtV2vAppArmorProfile(t *testing.T) {
	tests := []struct {
		name      string
		envType   string
		envPath   string
		wantType  string
		wantPath  string
		wantError bool
	}{
		{name: "unset leaves the pod spec untouched", wantType: "", wantPath: ""},
		{
			name:     "localhost without a name falls back to the shipped profile",
			envType:  string(core.AppArmorProfileTypeLocalhost),
			wantType: string(core.AppArmorProfileTypeLocalhost),
			wantPath: DefaultUnshareAppArmorProfile,
		},
		{
			name:     "localhost with an explicit name",
			envType:  string(core.AppArmorProfileTypeLocalhost),
			envPath:  "mtv-virt-v2v",
			wantType: string(core.AppArmorProfileTypeLocalhost),
			wantPath: "mtv-virt-v2v",
		},
		{
			name:     "an AppArmor name may look like a path",
			envType:  string(core.AppArmorProfileTypeLocalhost),
			envPath:  "/usr/sbin/passt//sandbox",
			wantType: string(core.AppArmorProfileTypeLocalhost),
			wantPath: "/usr/sbin/passt//sandbox",
		},
		{
			name:     "unconfined",
			envType:  string(core.AppArmorProfileTypeUnconfined),
			wantType: string(core.AppArmorProfileTypeUnconfined),
		},
		{name: "unknown type", envType: "disabled", wantError: true},
		{name: "name without a type", envPath: "mtv-virt-v2v", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(VirtV2vSeccompProfileType, "")
			t.Setenv(VirtV2vSeccompProfilePath, "")
			t.Setenv(VirtV2vAppArmorProfileType, tt.envType)
			t.Setenv(VirtV2vAppArmorProfilePath, tt.envPath)

			r := &Migration{}
			err := r.loadVirtV2vSecurityProfiles()

			if tt.wantError {
				if err == nil {
					t.Fatalf("expected an error, got type=%q path=%q",
						r.VirtV2vAppArmorProfileType, r.VirtV2vAppArmorProfilePath)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.VirtV2vAppArmorProfileType != tt.wantType {
				t.Errorf("type = %q, want %q", r.VirtV2vAppArmorProfileType, tt.wantType)
			}
			if r.VirtV2vAppArmorProfilePath != tt.wantPath {
				t.Errorf("name = %q, want %q", r.VirtV2vAppArmorProfilePath, tt.wantPath)
			}
		})
	}
}
