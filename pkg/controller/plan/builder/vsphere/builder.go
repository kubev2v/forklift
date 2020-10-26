package vsphere

import (
	"context"
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	libitr "github.com/konveyor/controller/pkg/itinerary"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1/plan"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/vsphere"
	vmio "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/vmware/govmomi/vim25"
	"gopkg.in/yaml.v2"
	core "k8s.io/api/core/v1"
	"net/http"
	liburl "net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// vSphere builder.
type Builder struct {
	// Client.
	Client client.Client
	// Provider API client.
	Inventory web.Client
	// Source provider.
	Provider *api.Provider
	// Host map.
	HostMap map[string]*api.Host
}

//
// Build the VMIO secret.
func (r *Builder) Secret(vmID string, in, object *core.Secret) (err error) {
	url := r.Provider.Spec.URL
	var hostFound bool
	var host *api.Host
	host, hostFound, err = r.host(vmID)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if hostFound {
		hostURL := liburl.URL{
			Scheme: "https",
			Host: host.Spec.IpAddress,
			Path: vim25.Path,
		}
		hostSecret, nErr := r.hostSecret(host)
		if nErr != nil {
			err = liberr.Wrap(nErr)
			return
		}
		url = hostURL.String()
		in = hostSecret

	}
	content, mErr := yaml.Marshal(
		map[string]string{
			"apiUrl":     url,
			"username":   string(in.Data["user"]),
			"password":   string(in.Data["password"]),
			"thumbprint": string(in.Data["thumbprint"]),
		})
	if mErr != nil {
		err = liberr.Wrap(mErr)
		return
	}
	object.StringData = map[string]string{
		"vmware": string(content),
	}

	return
}

//
// Get the (optional) host and secret.
func (r *Builder) hostSecret(host *api.Host) (secret *core.Secret, err error) {
	ref := host.Spec.Secret
	secret = &core.Secret{}
	err = r.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		secret)
	if err != nil {
		err = liberr.Wrap(err)
	}

	return
}

//
// Find host ID for VM.
func (r *Builder) host(vmID string) (host *api.Host, found bool, err error) {
	vm := &vsphere.VM{}
	status, pErr := r.Inventory.Get(vm, vmID)
	if pErr != nil {
		err = liberr.Wrap(pErr)
		return
	}
	switch status {
	case http.StatusOK:
		host, found = r.HostMap[vm.Host.ID]
	}

	return
}

//
// Build the VMIO ResourceMapping CR.
func (r *Builder) Mapping(mp *plan.Map, object *vmio.ResourceMapping) (err error) {
	netMap := []vmio.NetworkResourceMappingItem{}
	dsMap := []vmio.StorageResourceMappingItem{}
	for i := range mp.Networks {
		network := &mp.Networks[i]
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
	for i := range mp.Datastores {
		ds := &mp.Datastores[i]
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
	status, pErr := r.Inventory.Get(vm, vmID)
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
	status, pErr := r.Inventory.Get(vm, vmID)
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
