package conversion

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
)

func TestConversionSeccompProfile(t *testing.T) {
	tests := []struct {
		name             string
		setting          string
		openShift        bool
		wantType         core.SeccompProfileType
		wantLocalhostRef string
	}{
		{
			name:     "unset on kubernetes keeps the runtime default",
			wantType: core.SeccompProfileTypeRuntimeDefault,
		},
		{
			name:             "unset on openshift keeps the unshare profile",
			openShift:        true,
			wantType:         core.SeccompProfileTypeLocalhost,
			wantLocalhostRef: "profiles/unshare.json",
		},
		{
			name:             "setting selects a localhost profile on kubernetes",
			setting:          "profiles/unshare.json",
			wantType:         core.SeccompProfileTypeLocalhost,
			wantLocalhostRef: "profiles/unshare.json",
		},
		{
			name:             "setting overrides the openshift default",
			setting:          "profiles/custom.json",
			openShift:        true,
			wantType:         core.SeccompProfileTypeLocalhost,
			wantLocalhostRef: "profiles/custom.json",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			savedProfile := settings.Settings.Migration.VirtV2vSeccompProfile
			savedOpenShift := settings.Settings.OpenShift
			defer func() {
				settings.Settings.Migration.VirtV2vSeccompProfile = savedProfile
				settings.Settings.OpenShift = savedOpenShift
			}()
			settings.Settings.Migration.VirtV2vSeccompProfile = tc.setting
			settings.Settings.OpenShift = tc.openShift

			profile := conversionSeccompProfile()
			if profile.Type != tc.wantType {
				t.Errorf("type = %v, want %v", profile.Type, tc.wantType)
			}
			switch {
			case tc.wantLocalhostRef == "":
				if profile.LocalhostProfile != nil {
					t.Errorf("LocalhostProfile = %q, want nil", *profile.LocalhostProfile)
				}
			case profile.LocalhostProfile == nil:
				t.Errorf("LocalhostProfile = nil, want %q", tc.wantLocalhostRef)
			case *profile.LocalhostProfile != tc.wantLocalhostRef:
				t.Errorf("LocalhostProfile = %q, want %q", *profile.LocalhostProfile, tc.wantLocalhostRef)
			}
		})
	}
}
