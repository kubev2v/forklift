package settings

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// Environment variables.
const (
	CopyApplianceSSHUser        = "COPY_APPLIANCE_SSH_USER"
	CopyApplianceContainerImage = "COPY_APPLIANCE_CONTAINER_IMAGE"
)

// Defaults.
const (
	// DefaultCopyApplianceSSHUser is the account the appliance image installs
	// the public key for.
	DefaultCopyApplianceSSHUser = "root"
)

// CopyAppliance settings. These describe the appliance image itself, which is
// the same for every copy appliance in a deployment. Placement is derived per
// appliance from the source VM and provider settings, and the shape of the VM
// comes from the template it is cloned from.
type CopyAppliance struct {
	// Account the controller logs in to the appliance as.
	SSHUser string
	// Fully-qualified pull spec (or ImageStreamTag) for the nbd-container image.
	ContainerImage string
}

// Load settings.
func (r *CopyAppliance) Load() error {
	r.SSHUser = Lookup(CopyApplianceSSHUser, DefaultCopyApplianceSSHUser)
	r.ContainerImage = Lookup(CopyApplianceContainerImage, "")

	return nil
}

// EnabledForPlan reports whether disk transfer should use a copy appliance
// exporting source disks over NBD. VDDK takes priority when configured.
func (r *CopyAppliance) EnabledForPlan(p *api.Plan) bool {
	if !Settings.Features.Toehold {
		return false
	}
	if r.ContainerImage == "" {
		return false
	}
	if !p.IsSourceProviderVSphere() {
		return false
	}
	if p.IsUsingOffloadPlugin() {
		return false
	}
	var providerSettings map[string]string
	if p.Provider.Source != nil {
		providerSettings = p.Provider.Source.Spec.Settings
	}
	if GetVDDKImage(providerSettings) != "" {
		return false
	}
	return true
}
