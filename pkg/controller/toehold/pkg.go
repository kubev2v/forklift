// Package toehold builds and uploads a vCenter VM template from a base
// containerdisk image. The CopyApplianceTemplate CR is its lifecycle.
package toehold

import (
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
)

// Name of the controller, used for its logger and its event source.
const Name = "toehold"

// Internal resource naming and pod security for the OVA build pod.
const (
	credsSecretSuffix = "-vcenter-creds"
	qemuUser          = int64(107)
	qemuGroup         = int64(107)
)

// Settings are the forklift settings the controller reads.
var Settings = &settings.Settings

var log = logging.WithName(Name)
