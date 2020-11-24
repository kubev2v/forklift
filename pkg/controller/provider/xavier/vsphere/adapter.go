package vsphere

import (
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	model "github.com/konveyor/virt-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/virt-controller/pkg/controller/provider/xavier/base"
)

//
// Model adapter.
type Adapter struct {
	// DB
	libmodel.DB
}

//
// Update the vSphere xavier body.
func (r *Adapter) UpdateBody(object base.Object, m libmodel.Model) (err error) {
	vm := m.(*model.VM)
	cluster, err := vm.Cluster(r.DB)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	object[base.VmName] = vm.Name
	object[base.DiskSpace] = vm.StorageUsed
	object[base.Memory] = vm.MemoryMB
	object[base.CpuCores] = vm.CpuCount
	object[base.GuestOs] = vm.GuestName
	object[base.BalloonedMemory] = vm.BalloonedMemory
	object[base.HasMemoryHotAdd] = vm.MemoryHotAddEnabled
	object[base.HasCpuHotAdd] = vm.CpuHotAddEnabled
	object[base.HasCpuHotRemove] = vm.CpuHotRemoveEnabled
	object[base.HasCpuAffinity] = r.hasCpuAffinity(vm)
	object[base.HasRdmDisk] = r.hasRdmDisk(vm)
	object[base.HasPassthrough] = vm.PassthroughSupported
	object[base.HasUsb] = vm.UsbSupported
	object[base.HasDrsEnabled] = cluster.DrsEnabled
	object[base.HasHaEnabled] = cluster.DasEnabled
	return
}

//
// Update VM concerns.
func (r *Adapter) UpdateConcerns(m libmodel.Model, concerns []string) {
	vm := m.(*model.VM)
	list := model.List{}
	for _, name := range concerns {
		list = append(
			list,
			model.Concern{
				Severity: model.Warning,
				Name:     name,
			})
	}
	vm.Concerns = list.Encode()
	vm.RevisionAnalyzed = vm.Revision

	return
}

func (r *Adapter) hasCpuAffinity(vm *model.VM) bool {
	list := model.List{}
	list.With(vm.CpuAffinity)
	return len(list) > 0
}

func (r *Adapter) hasRdmDisk(vm *model.VM) (has bool) {
	for _, disk := range vm.DecodeDisks() {
		if disk.RDM {
			has = true
			return
		}
	}

	return
}
