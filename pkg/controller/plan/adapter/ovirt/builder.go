package ovirt

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
// oVirt builder.
type Builder struct {
	*plancontext.Context
	// Provisioner CRs.
	provisioners map[string]*api.Provisioner
	// Host CRs.
	hosts map[string]*api.Host
}

//
// Build the VMIO secret.
func (r *Builder) Secret(vmRef ref.Ref, in, object *core.Secret) (err error) {
	return
}

//
// Build the VMIO VM Import Spec.
func (r *Builder) Import(vmRef ref.Ref, object *vmio.VirtualMachineImportSpec) (err error) {
	return
}

//
// Build tasks.
func (r *Builder) Tasks(vmRef ref.Ref) (list []*plan.Task, err error) {
	return
}

//
// Return a stable identifier for a DataVolume.
func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return ""
}
