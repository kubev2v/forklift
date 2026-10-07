// Package copyappliance deploys and tears down the copy appliance VM that a
// migration reads source disks through. The appliance is a clone of a template
// in the source vCenter, and the CopyAppliance CR is its lifecycle.
package copyappliance

import (
	"fmt"
	"net"
	"regexp"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
)

// Name of the controller, used for its logger and its event source.
const Name = "copy-appliance"

// ApplianceContainerImageName is where the image is filed in the appliance's
// podman store. Deliberately not the cluster pull spec: the appliance has no
// reason to carry the in-cluster registry's hostname, and a digest reference
// cannot be a tag in a docker archive.
const ApplianceContainerImageName = "localhost/forklift-copy-appliance"

// AppliancePodmanLoadCommand reads a docker archive on its standard input and
// adds what it finds to the appliance's podman store.
const AppliancePodmanLoadCommand = "/usr/local/bin/copy-appliance-template-podman load"

// An appliance is cloned under the name its CR was given, and vCenter rejects a
// VM name over 80 characters. The API server appends a ~5-char suffix to a
// GenerateName, so the prefixes below stay well short of that. Nothing reads
// these names back; appliances are found by their labels.
const (
	rootResourcePool = "Resources"
	appliancePrefix  = "forklift-copy-"
	checkPrefix      = "forklift-copy-check-"
)

// Paths for the nbd-orchestrator. The controller image and the appliance both
// keep the binary at orchestratorBinary (see build/forklift-controller/Containerfile).
const (
	orchestratorBinary  = "/usr/local/bin/nbd-orchestrator"
	orchestratorUnit    = "nbd-orchestrator.service"
	orchestratorService = "/etc/systemd/system/" + orchestratorUnit
	applianceCertsDir   = "/etc/pki/nbd"
	// Staging beside the destination: cannot overwrite a running binary, and a
	// rename from /tmp would keep a label systemd will not execute.
	orchestratorStaging = "/usr/local/bin/.nbd-orchestrator.tmp"

	// applianceAnnouncePort is the port the orchestrator serves its export list
	// on. The appliance image is built with this port alone.
	applianceAnnouncePort = "8443"
	applianceBasePort     = 10809
)

// setupBinary is the appliance setup entry point in the controller image,
// which the setup pod runs in place of the image's own entry point (see
// build/forklift-controller/Containerfile).
const setupBinary = "/usr/local/bin/copy-appliance-setup"

// sshPrivateKeyData is the key the appliance's SSH secret holds its private key
// under. It matches the name the provider's SSH key secrets use.
const sshPrivateKeyData = "private-key"

// sshTimeout bounds one login and the commands run over it. A reconcile must
// not sit on an appliance that is not answering; the step is re-entered on the
// next pass.
const sshTimeout = 30 * time.Second

// SSHFileTransferTimeout bounds image load and similar long SSH transfers.
const SSHFileTransferTimeout = 30 * time.Minute

// ApplianceSSHPort is the port sshd listens on in the appliance image.
const ApplianceSSHPort = "22"

// Settings are the forklift settings the controller reads.
var Settings = &settings.Settings

var log = logging.WithName(Name)

// snapshotVMDKPattern matches VMware snapshot delta suffixes such as -000003.vmdk.
var snapshotVMDKPattern = regexp.MustCompile(`-\d{6}\.vmdk$`)

// NbdURI builds a TCP NBD connection URI for an appliance export.
func NbdURI(host string, port int32, ssl bool) string {
	scheme := "nbd"
	if ssl {
		scheme = "nbds"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, port)
}

// BaseVMDKPath strips a VMware snapshot suffix from a backing file path.
func BaseVMDKPath(path string) string {
	if path == "" {
		return path
	}
	return snapshotVMDKPattern.ReplaceAllString(path, ".vmdk")
}

// ExportNbdConnections maps each attached VMDK path to its NBD connection URI.
func ExportNbdConnections(appliance *api.CopyAppliance, ssl bool) (map[string]string, error) {
	if appliance == nil {
		return nil, liberr.New("copy appliance is not set")
	}
	if len(appliance.Status.Addresses) == 0 {
		return nil, liberr.New("copy appliance has no guest address yet")
	}
	host := appliance.Status.Addresses[0].IP
	if host == "" {
		return nil, liberr.New("copy appliance guest address is empty")
	}
	if net.ParseIP(host) == nil {
		return nil, liberr.New("copy appliance guest address is not a valid IP", "address", host)
	}
	if len(appliance.Status.Exports) == 0 {
		return nil, liberr.New("copy appliance has no disk exports yet")
	}
	attached := appliance.Spec.AttachDisks
	if len(appliance.Status.Exports) < len(attached) {
		return nil, liberr.New(
			"copy appliance exports are incomplete",
			"exports", fmt.Sprintf("%d", len(appliance.Status.Exports)),
			"attached", fmt.Sprintf("%d", len(attached)))
	}

	connections := map[string]string{}
	for _, export := range appliance.Status.Exports {
		if export.VMDKPath == "" {
			return nil, liberr.New("copy appliance export is missing a VMDK path")
		}
		uri := NbdURI(host, export.Port, ssl)
		connections[export.VMDKPath] = uri
		if base := BaseVMDKPath(export.VMDKPath); base != export.VMDKPath {
			connections[base] = uri
		}
	}
	return connections, nil
}

// IsDeployReady reports whether the appliance finished deploying and can serve exports.
func IsDeployReady(appliance *api.CopyAppliance) bool {
	if appliance == nil {
		return false
	}
	if appliance.Status.Phase != api.PhaseDeployCompleted {
		return false
	}
	ready := appliance.Status.FindCondition(libcnd.Ready)
	return ready != nil && ready.Status == libcnd.True
}
