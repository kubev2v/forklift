package openstack

import (
	"fmt"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	core "k8s.io/api/core/v1"
	cnv "kubevirt.io/client-go/api/v1"
)

// Openstack builder.
type Builder struct {
	*plancontext.Context
}

// Create the destination Kubevirt VM.
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []core.PersistentVolumeClaim) (err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}

	if object.Template == nil {
		object.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}
	r.mapDisks(vm, persistentVolumeClaims, object)
	// r.mapCPU(vm, object)
	// r.mapMemory(vm, object)
	// r.mapInput(object)
	if err != nil {
		return
	}

	return
}

func (r *Builder) mapDisks(vm *model.Workload, persistentVolumeClaims []core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk

	pvcMap := make(map[string]*core.PersistentVolumeClaim)
	for i, av := range vm.AttachedVolumes {
		pvc := pvcMap[av.ID]
		volumeName := fmt.Sprintf("vol-%v", i)
		volume := cnv.Volume{
			Name: volumeName,
			VolumeSource: cnv.VolumeSource{
				PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
				},
			},
		}
		kVolumes = append(kVolumes, volume)
	}
	object.Template.Spec.Domain.Devices.Disks = kDisks
}
