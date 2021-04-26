package base

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	core "k8s.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	vmio "kubevirt.io/vm-import-operator/pkg/apis/v2v/v1beta1"
)

//
// Adapter API.
// Constructs provider-specific implementations
// of the Builder and Validator.
type Adapter interface {
	// Construct builder.
	Builder(ctx *plancontext.Context) (Builder, error)
	// Construct validator.
	Validator(plan *api.Plan) (Validator, error)
}

//
// Builder API.
// Builds/updates objects as needed with provider
// specific constructs.
type Builder interface {
	// Build secret.
	Secret(vmRef ref.Ref, in, object *core.Secret) error
	// Build VMIO import spec.
	Import(vmRef ref.Ref, object *vmio.VirtualMachineImportSpec) error
	// Build tasks.
	Tasks(vmRef ref.Ref) ([]*plan.Task, error)
	// Return a stable identifier for a DataVolume.
	ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string
}

//
// Validator API.
// Performs provider-specific validation.
type Validator interface {
	// Validate that a VM's disk backing storage has been mapped.
	StorageMapped(vmRef ref.Ref) (bool, error)
	// Validate that a VM's networks have been mapped.
	NetworksMapped(vmRef ref.Ref) (bool, error)
}
