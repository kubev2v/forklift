package builder

import (
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/builder/vsphere"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	core "k8s.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	vmio "kubevirt.io/vm-import-operator/pkg/apis/v2v/v1beta1"
)

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
// Builder factory.
func New(ctx *plancontext.Context) (builder Builder, err error) {
	//
	switch ctx.Source.Provider.Type() {
	case api.VSphere:
		b := &vsphere.Builder{Context: ctx}
		bErr := b.Load()
		if bErr != nil {
			err = liberr.Wrap(bErr)
		} else {
			builder = b
		}
	default:
		liberr.New("provider not supported.")
	}

	return
}
