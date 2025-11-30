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

// findStorageMapping looks up target storage class for EBS volume type (gp2, gp3, io1, etc).
// Returns storage class name from StorageMap or empty string if no mapping found.
func (r *Builder) findStorageMapping(volumeType string) string {
	if r.Map.Storage == nil {
		return ""
	}

	for _, mapping := range r.Map.Storage.Spec.Map {
		if mapping.Source.Name == volumeType {
			return mapping.Destination.StorageClass
		}
	}

	return ""
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

// getTargetAvailabilityZone retrieves AZ where new EBS volumes will be created (e.g., "us-east-1a").
// Must match target cluster's storage AZ since EBS volumes can only attach within same AZ.
// Returns from Provider.Spec.Settings["target-az"] or error if not configured.
func (r *Builder) getTargetAvailabilityZone() (string, error) {
	if r.Source.Provider.Spec.Settings == nil {
		return "", fmt.Errorf("provider settings not configured, target-az is required")
	}

	az, found := r.Source.Provider.Spec.Settings["target-az"]
	if !found || az == "" {
		return "", fmt.Errorf("target-az not configured in provider settings, this is required for EBS volume creation")
	}

	return az, nil
}

// ptrBool converts bool to *bool pointer for Kubernetes API structures that distinguish false from unset (nil).
func ptrBool(b bool) *bool {
	return &b
}
