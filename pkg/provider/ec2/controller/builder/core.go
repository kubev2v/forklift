package builder

import (
	"fmt"

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

// getRegion extracts AWS region from provider secret (e.g., "us-east-1").
// Required for AWS SDK clients and Ec2VolumePopulator specs. Returns error if secret missing or region not configured.
func (r *Builder) getRegion() (string, error) {
	if r.Source.Secret == nil {
		return "", fmt.Errorf("provider secret is nil, cannot determine AWS region")
	}

	region, found := r.Source.Secret.Data["region"]
	if !found || len(region) == 0 {
		return "", fmt.Errorf("region not configured in provider secret, please add 'region' key")
	}

	return string(region), nil
}
