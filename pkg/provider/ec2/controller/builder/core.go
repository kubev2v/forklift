package builder

import (
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

// Builder generates Kubernetes resource specs from EC2 instances and EBS volumes.
// Transforms EC2 instances→VirtualMachine, volumes→PVCs, snapshots→Ec2VolumePopulator CRs.
type Builder struct {
	*plancontext.Context                     // Plan context with provider config, mappings, target namespace
	log                  logging.LevelLogger // Structured logger with "builder|ec2" prefix
}

// New creates a new EC2 Builder with plan context for accessing provider secrets, mappings, and target namespace.
func New(ctx *plancontext.Context) *Builder {
	log := logging.WithName("builder|ec2")
	return &Builder{
		Context: ctx,
		log:     log,
	}
}
