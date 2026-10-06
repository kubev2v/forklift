package settings

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// Environment variables.
const (
	CopyApplianceSSHUser        = "COPY_APPLIANCE_SSH_USER"
	CopyApplianceContainerImage = "COPY_APPLIANCE_CONTAINER_IMAGE"
	// Read by the controller, which runs one setup pod per appliance.
	CopyApplianceSetupImage           = "COPY_APPLIANCE_SETUP_IMAGE"
	CopyApplianceSetupTransferNetwork = "COPY_APPLIANCE_SETUP_TRANSFER_NETWORK"
)

// Environment variables the controller writes onto a setup pod and the setup
// pod reads back. They describe one appliance, not the deployment, so they are
// not part of the CopyAppliance settings.
const (
	CopyApplianceSetupAppliance = "COPY_APPLIANCE_SETUP_APPLIANCE"
	CopyApplianceSetupAddress   = "COPY_APPLIANCE_SETUP_ADDRESS"
	CopyApplianceSetupPullSpec  = "COPY_APPLIANCE_SETUP_PULL_SPEC"
	CopyApplianceSetupTag       = "COPY_APPLIANCE_SETUP_TAG"
	CopyApplianceSetupKeyDir    = "COPY_APPLIANCE_SETUP_KEY_DIR"
	CopyApplianceSetupNbdSsl    = "COPY_APPLIANCE_SETUP_NBD_SSL"
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
	// Fully-qualified pull spec for the nbd-container image. The setup pod that
	// reads it carries no registry credential, so it has to be readable without
	// one.
	ContainerImage string
	// Image the appliance setup pod runs. The controller image, which carries
	// both the setup binary and the nbd-orchestrator the setup installs.
	SetupImage string
	// Multus networks annotation to put on setup pods, copied from the
	// controller pod's own annotation. Empty means none.
	SetupTransferNetwork string
}

// Load settings.
func (r *CopyAppliance) Load() error {
	r.SSHUser = Lookup(CopyApplianceSSHUser, DefaultCopyApplianceSSHUser)
	r.ContainerImage = Lookup(CopyApplianceContainerImage, "")
	r.SetupImage = Lookup(CopyApplianceSetupImage, "")
	r.SetupTransferNetwork = Lookup(CopyApplianceSetupTransferNetwork, "")

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
