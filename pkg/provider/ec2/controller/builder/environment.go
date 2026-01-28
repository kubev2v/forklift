package builder

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/inventory"
	core "k8s.io/api/core/v1"
)

// PodEnvironment sets environment variables for the virt-v2v conversion pod.
// Sets V2V_source to "ec2" and V2V_vmName for proper disk naming during conversion.
// These environment variables are used by the virt-v2v entrypoint to determine
// the conversion method (disk-based in-place conversion for EC2).
func (r *Builder) PodEnvironment(vmRef ref.Ref, sourceSecret *core.Secret) (env []core.EnvVar, err error) {
	instance, err := inventory.GetAWSInstance(r.Source.Inventory, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	vmName := inventory.GetInstanceName(instance)

	env = append(env,
		core.EnvVar{
			Name:  "V2V_source",
			Value: "ec2",
		},
		core.EnvVar{
			Name:  "V2V_vmName",
			Value: vmName,
		},
	)
	return
}
