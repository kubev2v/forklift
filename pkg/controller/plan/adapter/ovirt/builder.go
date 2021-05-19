package ovirt

import (
	"context"
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/ovirt"
	"gopkg.in/yaml.v2"
	core "k8s.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	vmio "kubevirt.io/vm-import-operator/pkg/apis/v2v/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// oVirt builder.
type Builder struct {
	*plancontext.Context
	// Provisioner CRs.
	provisioners map[string]*api.Provisioner
}

//
// Build the VMIO secret.
func (r *Builder) Secret(vmRef ref.Ref, in, object *core.Secret) (err error) {
	url := r.Source.Provider.Spec.URL

	content, mErr := yaml.Marshal(
		map[string]string{
			"apiUrl":   url,
			"username": string(in.Data["user"]),
			"password": string(in.Data["password"]),
			"ca.cert":  string(in.Data["cacert"]),
		})
	if mErr != nil {
		err = liberr.Wrap(mErr)
		return
	}
	object.StringData = map[string]string{
		"ovirt": string(content),
	}

	return
}

//
// Build the VMIO VM Import Spec.
func (r *Builder) Import(vmRef ref.Ref, object *vmio.VirtualMachineImportSpec) (err error) {
	vm := &model.VM{}
	pErr := r.Source.Inventory.Find(vm, vmRef)
	if pErr != nil {
		err = liberr.New(
			fmt.Sprintf(
				"VM %s lookup failed: %s",
				vmRef.String(),
				pErr.Error()))
		return
	}

	object.TargetVMName = &vm.Name
	object.Source.Ovirt = &vmio.VirtualMachineImportOvirtSourceSpec{
		VM: vmio.VirtualMachineImportOvirtSourceVMSpec{
			ID: &vm.ID,
		},
	}
	object.Source.Ovirt.Mappings, err = r.mapping(vm)
	if err != nil {
		return
	}

	return
}

func (r *Builder) mapping(vm *model.VM) (out *vmio.OvirtMappings, err error) {
	netMap := []vmio.NetworkResourceMappingItem{}
	storageMap := []vmio.StorageResourceMappingItem{}
	netMapIn := r.Context.Map.Network.Spec.Map
	for i := range netMapIn {
		mapped := &netMapIn[i]
		ref := mapped.Source
		network := &model.Network{}
		fErr := r.Source.Inventory.Find(network, ref)
		if err != nil {
			err = fErr
			return
		}
		needed := false
		//for _, net := range vm.Networks {
		//	if net.ID == network.ID {
		//		needed = true
		//		break
		//	}
		//}
		if !needed {
			continue
		}
		netMap = append(
			netMap,
			vmio.NetworkResourceMappingItem{
				Source: vmio.Source{
					ID: &network.ID,
				},
				Target: vmio.ObjectIdentifier{
					Namespace: &mapped.Destination.Namespace,
					Name:      mapped.Destination.Name,
				},
				Type: &mapped.Destination.Type,
			})
	}
	storageMapIn := r.Context.Map.Storage.Spec.Map
	for i := range storageMapIn {
		mapped := &storageMapIn[i]
		ref := mapped.Source
		domain := &model.StorageDomain{}
		fErr := r.Source.Inventory.Find(domain, ref)
		if fErr != nil {
			err = fErr
			return
		}
		needed := false
		//for _, disk := range vm.Disks {
		//	if disk.StorageDomain.ID == domain.ID {
		//		needed = true
		//		break
		//	}
		//}
		if !needed {
			continue
		}
		mErr := r.defaultModes(&mapped.Destination)
		if mErr != nil {
			err = mErr
			return
		}
		item := vmio.StorageResourceMappingItem{
			Source: vmio.Source{
				ID: &domain.ID,
			},
			Target: vmio.ObjectIdentifier{
				Name: mapped.Destination.StorageClass,
			},
		}
		if mapped.Destination.VolumeMode != "" {
			item.VolumeMode = &mapped.Destination.VolumeMode
		}
		if mapped.Destination.AccessMode != "" {
			item.AccessMode = &mapped.Destination.AccessMode
		}
		storageMap = append(storageMap, item)
	}
	out = &vmio.OvirtMappings{
		NetworkMappings: &netMap,
		StorageMappings: &storageMap,
	}

	return
}


//
// Set volume and access modes.
func (r *Builder) defaultModes(dm *api.DestinationStorage) (err error) {
	model := &ocp.StorageClass{}
	ref := ref.Ref{Name: dm.StorageClass}
	err = r.Destination.Inventory.Find(model, ref)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if dm.VolumeMode == "" || dm.AccessMode == "" {
		if provisioner, found := r.provisioners[model.Object.Provisioner]; found {
			volumeMode := provisioner.VolumeMode(dm.VolumeMode)
			accessMode := volumeMode.AccessMode(dm.AccessMode)
			if dm.VolumeMode == "" {
				dm.VolumeMode = volumeMode.Name
			}
			if dm.AccessMode == "" {
				dm.AccessMode = accessMode.Name
			}
		}
	}

	return
}

//
// Build tasks.
func (r *Builder) Tasks(vmRef ref.Ref) (list []*plan.Task, err error) {
	vm := &model.VM{}
	pErr := r.Source.Inventory.Find(vm, vmRef)
	if pErr != nil {
		err = liberr.New(
			fmt.Sprintf(
				"VM %s lookup failed: %s",
				vmRef.String(),
				pErr.Error()))
		return
	}
	//for _, disk := range vm.Disks {
	//	mB := disk.Capacity / 0x100000
	//	list = append(
	//		list,
	//		&plan.Task{
	//			Name: disk.ID,
	//			Progress: libitr.Progress{
	//				Total: mB,
	//			},
	//			Annotations: map[string]string{
	//				"unit": "MB",
	//			},
	//		})
	//}

	return
}

//
// Return a stable identifier for a DataVolume.
func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return dv.Spec.Source.Imageio.DiskID
}

func (r *Builder) Load() (err error) {
	return r.loadProvisioners()
}

//
// Load provisioner CRs.
func (r *Builder) loadProvisioners() (err error) {
	list := &api.ProvisionerList{}
	err = r.List(
		context.TODO(),
		list,
		&client.ListOptions{
			Namespace: r.Source.Provider.Namespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.provisioners = map[string]*api.Provisioner{}
	for i := range list.Items {
		p := &list.Items[i]
		r.provisioners[p.Spec.Name] = p
	}

	return
}