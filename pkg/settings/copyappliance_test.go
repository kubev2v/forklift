package settings

import (
	"testing"
)

func TestCopyApplianceDefaults(t *testing.T) {
	applied := CopyAppliance{}
	if err := applied.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if applied.SSHUser != DefaultCopyApplianceSSHUser {
		t.Errorf("SSHUser = %q, want %q", applied.SSHUser, DefaultCopyApplianceSSHUser)
	}
	if applied.ContainerImage != "" {
		t.Errorf("ContainerImage = %q, want it unset", applied.ContainerImage)
	}
}

func TestCopyApplianceFromEnvironment(t *testing.T) {
	t.Setenv(CopyApplianceSSHUser, "appliance")
	t.Setenv(CopyApplianceContainerImage, "copy-appliance:latest")

	applied := CopyAppliance{}
	if err := applied.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if applied.SSHUser != "appliance" {
		t.Errorf("SSHUser = %q, want the configured user", applied.SSHUser)
	}
	if applied.ContainerImage != "copy-appliance:latest" {
		t.Errorf("ContainerImage = %q, want the configured image", applied.ContainerImage)
	}
}
