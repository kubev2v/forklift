package conversion

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	convctx "github.com/kubev2v/forklift/pkg/controller/conversion/context"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Pins what the override must not break: OpenShift keeps the profile it had,
// and a silent cluster keeps the runtime default rather than a profile its
// nodes may not carry.
func TestVirtV2vSeccompProfile(t *testing.T) {
	tests := []struct {
		name        string
		openShift   bool
		profileType string
		profilePath string
		wantType    core.SeccompProfileType
		wantPath    string
	}{
		{
			name:      "openshift default is unchanged",
			openShift: true,
			wantType:  core.SeccompProfileTypeLocalhost,
			wantPath:  settings.DefaultUnshareSeccompProfilePath,
		},
		{
			name:     "kubernetes default is unchanged",
			wantType: core.SeccompProfileTypeRuntimeDefault,
		},
		{
			name:        "kubernetes opts in to a localhost profile",
			profileType: string(core.SeccompProfileTypeLocalhost),
			profilePath: "profiles/unshare.json",
			wantType:    core.SeccompProfileTypeLocalhost,
			wantPath:    "profiles/unshare.json",
		},
		{
			name:        "localhost with no path uses the shipped profile",
			profileType: string(core.SeccompProfileTypeLocalhost),
			wantType:    core.SeccompProfileTypeLocalhost,
			wantPath:    settings.DefaultUnshareSeccompProfilePath,
		},
		{
			name:        "openshift can be overridden back to the runtime default",
			openShift:   true,
			profileType: string(core.SeccompProfileTypeRuntimeDefault),
			wantType:    core.SeccompProfileTypeRuntimeDefault,
		},
		{
			name:        "unconfined carries no path",
			profileType: string(core.SeccompProfileTypeUnconfined),
			wantType:    core.SeccompProfileTypeUnconfined,
		},
	}

	origOpenShift := settings.Settings.OpenShift
	origType := settings.Settings.Migration.VirtV2vSeccompProfileType
	origPath := settings.Settings.Migration.VirtV2vSeccompProfilePath
	t.Cleanup(func() {
		settings.Settings.OpenShift = origOpenShift
		settings.Settings.Migration.VirtV2vSeccompProfileType = origType
		settings.Settings.Migration.VirtV2vSeccompProfilePath = origPath
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings.Settings.OpenShift = tt.openShift
			settings.Settings.Migration.VirtV2vSeccompProfileType = tt.profileType
			settings.Settings.Migration.VirtV2vSeccompProfilePath = tt.profilePath

			got := virtV2vSeccompProfile()

			if got.Type != tt.wantType {
				t.Errorf("type = %q, want %q", got.Type, tt.wantType)
			}
			switch {
			case tt.wantPath == "":
				if got.LocalhostProfile != nil {
					t.Errorf("localhost profile = %q, want nil", *got.LocalhostProfile)
				}
			case got.LocalhostProfile == nil:
				t.Errorf("localhost profile = nil, want %q", tt.wantPath)
			case *got.LocalhostProfile != tt.wantPath:
				t.Errorf("localhost profile = %q, want %q", *got.LocalhostProfile, tt.wantPath)
			}
		})
	}
}

// Unset must stay nil on every platform: no cluster changes confinement by
// upgrading alone.
func TestVirtV2vAppArmorProfile(t *testing.T) {
	tests := []struct {
		name        string
		openShift   bool
		profileType string
		profilePath string
		wantNil     bool
		wantType    core.AppArmorProfileType
		wantPath    string
	}{
		{name: "unset on kubernetes", wantNil: true},
		{name: "unset on openshift", openShift: true, wantNil: true},
		{
			name:        "localhost with an explicit name",
			profileType: string(core.AppArmorProfileTypeLocalhost),
			profilePath: "forklift-virt-v2v-unshare",
			wantType:    core.AppArmorProfileTypeLocalhost,
			wantPath:    "forklift-virt-v2v-unshare",
		},
		{
			name:        "localhost with no name uses the shipped profile",
			profileType: string(core.AppArmorProfileTypeLocalhost),
			wantType:    core.AppArmorProfileTypeLocalhost,
			wantPath:    settings.DefaultUnshareAppArmorProfile,
		},
		{
			name:        "unconfined carries no name",
			profileType: string(core.AppArmorProfileTypeUnconfined),
			wantType:    core.AppArmorProfileTypeUnconfined,
		},
	}

	origOpenShift := settings.Settings.OpenShift
	origType := settings.Settings.Migration.VirtV2vAppArmorProfileType
	origPath := settings.Settings.Migration.VirtV2vAppArmorProfilePath
	t.Cleanup(func() {
		settings.Settings.OpenShift = origOpenShift
		settings.Settings.Migration.VirtV2vAppArmorProfileType = origType
		settings.Settings.Migration.VirtV2vAppArmorProfilePath = origPath
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings.Settings.OpenShift = tt.openShift
			settings.Settings.Migration.VirtV2vAppArmorProfileType = tt.profileType
			settings.Settings.Migration.VirtV2vAppArmorProfilePath = tt.profilePath

			got := virtV2vAppArmorProfile()

			if tt.wantNil {
				if got != nil {
					t.Fatalf("profile = %+v, want nil", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("profile = nil, want type %q", tt.wantType)
			}
			if got.Type != tt.wantType {
				t.Errorf("type = %q, want %q", got.Type, tt.wantType)
			}
			switch {
			case tt.wantPath == "":
				if got.LocalhostProfile != nil {
					t.Errorf("localhost profile = %q, want nil", *got.LocalhostProfile)
				}
			case got.LocalhostProfile == nil:
				t.Errorf("localhost profile = nil, want %q", tt.wantPath)
			case *got.LocalhostProfile != tt.wantPath:
				t.Errorf("localhost profile = %q, want %q", *got.LocalhostProfile, tt.wantPath)
			}
		})
	}
}

// Guards the wiring, not the resolution: profiles can be computed correctly and
// still never reach either pod spec.
func TestVirtV2vPodSpecCarriesSecurityProfiles(t *testing.T) {
	origOpenShift := settings.Settings.OpenShift
	origSeccompType := settings.Settings.Migration.VirtV2vSeccompProfileType
	origSeccompPath := settings.Settings.Migration.VirtV2vSeccompProfilePath
	origAppArmorType := settings.Settings.Migration.VirtV2vAppArmorProfileType
	origAppArmorPath := settings.Settings.Migration.VirtV2vAppArmorProfilePath
	t.Cleanup(func() {
		settings.Settings.OpenShift = origOpenShift
		settings.Settings.Migration.VirtV2vSeccompProfileType = origSeccompType
		settings.Settings.Migration.VirtV2vSeccompProfilePath = origSeccompPath
		settings.Settings.Migration.VirtV2vAppArmorProfileType = origAppArmorType
		settings.Settings.Migration.VirtV2vAppArmorProfilePath = origAppArmorPath
	})

	// The builder parses these unconditionally, so they cannot be left empty.
	origResources := [4]string{
		settings.Settings.Migration.VirtV2vContainerRequestsCpu,
		settings.Settings.Migration.VirtV2vContainerRequestsMemory,
		settings.Settings.Migration.VirtV2vContainerLimitsCpu,
		settings.Settings.Migration.VirtV2vContainerLimitsMemory,
	}
	t.Cleanup(func() {
		settings.Settings.Migration.VirtV2vContainerRequestsCpu = origResources[0]
		settings.Settings.Migration.VirtV2vContainerRequestsMemory = origResources[1]
		settings.Settings.Migration.VirtV2vContainerLimitsCpu = origResources[2]
		settings.Settings.Migration.VirtV2vContainerLimitsMemory = origResources[3]
	})
	settings.Settings.Migration.VirtV2vContainerRequestsCpu = "1000m"
	settings.Settings.Migration.VirtV2vContainerRequestsMemory = "1Gi"
	settings.Settings.Migration.VirtV2vContainerLimitsCpu = "4000m"
	settings.Settings.Migration.VirtV2vContainerLimitsMemory = "8Gi"

	settings.Settings.OpenShift = false
	settings.Settings.Migration.VirtV2vSeccompProfileType = string(core.SeccompProfileTypeLocalhost)
	settings.Settings.Migration.VirtV2vSeccompProfilePath = "profiles/unshare.json"
	settings.Settings.Migration.VirtV2vAppArmorProfileType = string(core.AppArmorProfileTypeLocalhost)
	settings.Settings.Migration.VirtV2vAppArmorProfilePath = "forklift-virt-v2v-unshare"

	builder := &Builder{Config: convctx.PodConfig{
		TargetNamespace: "vms",
		Image:           "quay.io/kubev2v/forklift-virt-v2v:latest",
		GenerateName:    "probe-",
	}}
	vm := &plan.VMStatus{}
	secret := &core.Secret{ObjectMeta: meta.ObjectMeta{Name: "v2v-secret", Namespace: "vms"}}

	assertProfiles := func(t *testing.T, name string, sc *core.PodSecurityContext) {
		t.Helper()
		if sc == nil {
			t.Fatalf("%s: pod security context is nil", name)
		}
		if sc.SeccompProfile == nil {
			t.Fatalf("%s: seccomp profile is nil", name)
		}
		if sc.SeccompProfile.Type != core.SeccompProfileTypeLocalhost {
			t.Errorf("%s: seccomp type = %q, want Localhost", name, sc.SeccompProfile.Type)
		}
		if sc.SeccompProfile.LocalhostProfile == nil ||
			*sc.SeccompProfile.LocalhostProfile != "profiles/unshare.json" {
			t.Errorf("%s: seccomp profile path not carried through", name)
		}
		if sc.AppArmorProfile == nil {
			t.Fatalf("%s: apparmor profile is nil", name)
		}
		if sc.AppArmorProfile.Type != core.AppArmorProfileTypeLocalhost {
			t.Errorf("%s: apparmor type = %q, want Localhost", name, sc.AppArmorProfile.Type)
		}
		if sc.AppArmorProfile.LocalhostProfile == nil ||
			*sc.AppArmorProfile.LocalhostProfile != "forklift-virt-v2v-unshare" {
			t.Errorf("%s: apparmor profile name not carried through", name)
		}
	}

	conversionPod, _, err := builder.GetVirtV2vPodSpec(vm, nil, nil, nil, secret, false)
	if err != nil {
		t.Fatalf("conversion pod spec: %v", err)
	}
	assertProfiles(t, "conversion", conversionPod.Spec.SecurityContext)

	inspectionPod, err := builder.BuildVirtV2vInspectionPod(conversionPod, nil, vm)
	if err != nil {
		t.Fatalf("inspection pod spec: %v", err)
	}
	assertProfiles(t, "inspection", inspectionPod.Spec.SecurityContext)
}
