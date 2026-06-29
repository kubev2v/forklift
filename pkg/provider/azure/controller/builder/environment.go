package builder

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	core "k8s.io/api/core/v1"
)

// PodEnvironment sets environment variables for the virt-v2v conversion pod.
// Sets V2V_source to "azure" and V2V_vmName for proper disk naming during conversion.
// These environment variables are used by the virt-v2v entrypoint to determine
// the conversion method (disk-based in-place conversion for Azure).
func (r *Builder) PodEnvironment(vmRef ref.Ref, sourceSecret *core.Secret) ([]core.EnvVar, error) {
	return []core.EnvVar{
		{Name: "V2V_source", Value: "azure"},
		{Name: "V2V_vmName", Value: vmRef.Name},
	}, nil
}
