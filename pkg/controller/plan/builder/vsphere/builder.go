package vsphere

import (
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	libitr "github.com/konveyor/controller/pkg/itinerary"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1/plan"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/vsphere"
	vmio "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"gopkg.in/yaml.v2"
	core "k8s.io/api/core/v1"
	"net/http"
)

//
// Annotations.
const (
	Disk = "Disk"
)

//
// vSphere builder.
type Builder struct {
	// Provider.
	Provider *api.Provider
	// Client.
	Client web.Client
}

//
// Build the VMIO secret.
func (r *Builder) Secret(in, object *core.Secret) (err error) {
	content, mErr := yaml.Marshal(
		map[string]string{
			"apiUrl":     r.Provider.Spec.URL,
			"username":   string(in.Data["user"]),
			"password":   string(in.Data["password"]),
			"thumbprint": string(in.Data["thumbprint"]),
		})
	if mErr != nil {
		mErr = liberr.Wrap(err)
		return
	}
	object.StringData = map[string]string{
		"vmware": string(content),
	}

	return
}

//
// Build the VMIO ResourceMapping CR.
func (r *Builder) Mapping(mp *plan.Map, object *vmio.ResourceMapping) (err error) {
	netMap := []vmio.NetworkResourceMappingItem{}
	dsMap := []vmio.StorageResourceMappingItem{}
	for _, network := range mp.Networks {
		netMap = append(
			netMap,
			vmio.NetworkResourceMappingItem{
				Source: vmio.Source{
					ID: &network.Source.ID,
				},
				Target: vmio.ObjectIdentifier{
					Namespace: &network.Destination.Namespace,
					Name:      network.Destination.Name,
				},
			})
	}
	for _, ds := range mp.Datastores {
		dsMap = append(
			dsMap,
			vmio.StorageResourceMappingItem{
				Source: vmio.Source{
					ID: &ds.Source.ID,
				},
				Target: vmio.ObjectIdentifier{
					Name: ds.Destination.StorageClass,
				},
			})
	}
	object.Spec.VmwareMappings = &vmio.VmwareMappings{
		NetworkMappings: &netMap,
		StorageMappings: &dsMap,
	}

	return
}

//
// Build the VMIO VM Source.
func (r *Builder) Source(vmID string, object *vmio.VirtualMachineImportSourceSpec) (err error) {
	vm := &vsphere.VM{}
	status, pErr := r.Client.Get(vm, vmID)
	if pErr != nil {
		err = liberr.Wrap(pErr)
		return
	}
	switch status {
	case http.StatusOK:
		uuid := vm.UUID
		object.Vmware = &vmio.VirtualMachineImportVmwareSourceSpec{
			VM: vmio.VirtualMachineImportVmwareSourceVMSpec{
				ID: &uuid,
			},
		}
	default:
		err = liberr.New(
			fmt.Sprintf(
				"VM %s lookup failed: %s",
				vmID,
				http.StatusText(status)))
	}

	return
}

//
// Build tasks.
func (r *Builder) Tasks(vmID string) (list []*plan.Task, err error) {
	vm := &vsphere.VM{}
	status, pErr := r.Client.Get(vm, vmID)
	if pErr != nil {
		err = liberr.Wrap(pErr)
		return
	}
	switch status {
	case http.StatusOK:
		for _, disk := range vm.Disks {
			mB := disk.Capacity / 0x100000
			list = append(
				list,
				&plan.Task{
					Name: disk.File,
					Progress: libitr.Progress{
						Total: mB,
					},
					Annotations: map[string]string{
						"unit": "MB",
					},
				})
		}
	default:
		err = liberr.New(
			fmt.Sprintf(
				"VM %s lookup failed: %s",
				vmID,
				http.StatusText(status)))
	}

	return
}
