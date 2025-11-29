package ensurer

import (
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
)

// Ensurer creates and verifies Kubernetes resources for EC2 migrations.
// Creates PVCs, Ec2VolumePopulator CRs, secrets with AWS credentials, RBAC resources. Idempotent.
type Ensurer struct {
	*plancontext.Context                     // Plan context with target namespace, client, labeler
	log                  logging.LevelLogger // Structured logger for resource tracking
}

// New creates a new EC2 Ensurer with plan context for resource creation in target namespace.
func New(ctx *plancontext.Context) *Ensurer {
	log := logging.WithName("ensurer|ec2")
	return &Ensurer{
		Context: ctx,
		log:     log,
	}
}

// SharedConfigMaps is a no-op for EC2.
// EC2 VMs don't use shared ConfigMaps - configuration derived from EC2 instance metadata.
func (r *Ensurer) SharedConfigMaps(vm *planapi.VMStatus, configMaps []core.ConfigMap) error {
	// EC2 provider doesn't use shared config maps currently
	return nil
}

// SharedSecrets is a no-op for EC2.
// EC2 uses per-VM populator secrets (via EnsurePopulatorSecret), not shared secrets across VMs.
func (r *Ensurer) SharedSecrets(vm *planapi.VMStatus, secrets []core.Secret) error {
	// EC2 provider doesn't use shared secrets currently (populator secrets are handled separately)
	return nil
}
