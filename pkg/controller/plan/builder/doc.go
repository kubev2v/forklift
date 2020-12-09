package builder

import (
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/builder/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	vmio "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Builder API.
// Builds/updates objects as needed with provider
// specific constructs.
type Builder interface {
	// Build secret.
	Secret(vmRef ref.Ref, in, object *core.Secret) error
	// Build VMIO import spec.
	Import(vmRef ref.Ref, mp *plan.Map, object *vmio.VirtualMachineImportSpec) error
	// Build tasks.
	Tasks(vmRef ref.Ref) ([]*plan.Task, error)
}

//
// Builder factory.
func New(
	client client.Client,
	inventory web.Client,
	provider *api.Provider) (builder Builder, err error) {
	//
	switch provider.Type() {
	case api.VSphere:
		b := &vsphere.Builder{
			Client:    client,
			Inventory: inventory,
			Provider:  provider,
		}
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
