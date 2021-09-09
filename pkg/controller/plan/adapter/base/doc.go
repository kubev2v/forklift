package base

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
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
	// Construct VM client.
	Client(ctx *plancontext.Context) (Client, error)
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
// Client API.
// Performs provider-specific actions on the source VM.
type Client interface {
	// Power on the source VM.
	PowerOn(vmRef ref.Ref) error
	// Power off the source VM.
	PowerOff(vmRef ref.Ref) error
	// Return the source VM's power state.
	PowerState(vmRef ref.Ref) (string, error)
	// Return whether the source VM is powered off.
	PoweredOff(vmRef ref.Ref) (bool, error)
	// Create a snapshot of the source VM.
	CreateSnapshot(vmRef ref.Ref) (string, error)
	// Remove a snapshot of the source VM.
	RemoveSnapshot(vmRef ref.Ref, snapshot string, all bool) error
	// Close connections to the provider API.
	Close()
}

//
// Validator API.
// Performs provider-specific validation.
type Validator interface {
	// Validate that a VM's disk backing storage has been mapped.
	StorageMapped(vmRef ref.Ref) (bool, error)
	// Validate that a VM's networks have been mapped.
	NetworksMapped(vmRef ref.Ref) (bool, error)
	// Validate that a VM's Host isn't in maintenance mode.
	MaintenanceMode(vmRef ref.Ref) (bool, error)
}
