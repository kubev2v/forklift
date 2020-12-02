package vsphere

import (
	"context"
	"fmt"
	liberr "github.com/konveyor/controller/pkg/error"
	libitr "github.com/konveyor/controller/pkg/itinerary"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
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
	hostID, err := r.hostID(vmID)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if host, found := r.HostMap[hostID]; found {
		hostURL := liburl.URL{
			Scheme: "https",
			Host:   host.Spec.IpAddress,
			Path:   vim25.Path,
		}
		url = hostURL.String()
		hostSecret, nErr := r.hostSecret(host)
		if nErr != nil {
			err = liberr.Wrap(nErr)
			return
		}
		h, nErr := r.host(hostID)
		if nErr != nil {
			err = liberr.Wrap(nErr)
			return
		}
		hostSecret.Data["thumbprint"] = []byte(h.Thumbprint)
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
// Find host ID for VM.
func (r *Builder) hostID(vmID string) (hostID string, err error) {
	vm := &vsphere.VM{}
	status, pErr := r.Inventory.Get(vm, vmID)
	if pErr != nil {
		err = liberr.Wrap(pErr)
		return
	}
	switch status {
	case http.StatusOK:
		hostID = vm.Host.ID
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
// Find host CR secret.
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
// Find host in the inventory.
func (r *Builder) host(hostID string) (host *vsphere.Host, err error) {
	host = &vsphere.Host{}
	status, err := r.Inventory.Get(host, hostID)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	switch status {
	case http.StatusOK:
	default:
		err = liberr.New(
			fmt.Sprintf(
				"Host %s lookup failed: %s",
				hostID,
				http.StatusText(status)))
		return
	}

	return
}

//
// Build the VMIO ResourceMapping CR.
func (r *Builder) Mapping(mp *plan.Map, object *vmio.VirtualMachineImportSpec) (err error) {
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
				Type: &network.Destination.Type,
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
	object.Source.Vmware.Mappings = &vmio.VmwareMappings{
		NetworkMappings: &netMap,
		StorageMappings: &dsMap,
	}

	return
}

//
// Build the VMIO VM Import Spec.
func (r *Builder) Import(vmID string, mp *plan.Map, object *vmio.VirtualMachineImportSpec) (err error) {
	vm := &vsphere.VM{}
	status, pErr := r.Inventory.Get(vm, vmID)
	if pErr != nil {
		err = liberr.Wrap(pErr)
		return
	}
	switch status {
	case http.StatusOK:
		uuid := vm.UUID
		object.TargetVMName = &vm.Name
		object.Source.Vmware = &vmio.VirtualMachineImportVmwareSourceSpec{
			VM: vmio.VirtualMachineImportVmwareSourceVMSpec{
				ID: &uuid,
			},
		}
		r.Mapping(mp, object)
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
