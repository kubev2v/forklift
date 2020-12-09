package vsphere

import (
	"context"
	"errors"
	"fmt"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libitr "github.com/konveyor/controller/pkg/itinerary"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	vmio "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1beta1"
	"github.com/vmware/govmomi/vim25"
	"gopkg.in/yaml.v2"
	core "k8s.io/api/core/v1"
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
	hostMap map[string]*api.Host
}

//
// Build the VMIO secret.
func (r *Builder) Secret(vmRef ref.Ref, in, object *core.Secret) (err error) {
	url := r.Provider.Spec.URL
	hostID, err := r.hostID(vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if hostDef, found := r.hostMap[hostID]; found {
		hostURL := liburl.URL{
			Scheme: "https",
			Host:   hostDef.Spec.IpAddress,
			Path:   vim25.Path,
		}
		url = hostURL.String()
		if libref.RefSet(hostDef.Spec.Secret) {
			hostSecret, nErr := r.hostSecret(hostDef)
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
// Build the VMIO VM Import Spec.
func (r *Builder) Import(vmRef ref.Ref, mp *plan.Map, object *vmio.VirtualMachineImportSpec) (err error) {
	vm := &model.VM{}
	pErr := r.Inventory.Find(vm, vmRef)
	if pErr != nil {
		err = liberr.New(
			fmt.Sprintf(
				"VM %s lookup failed: %s",
				vmRef.String(),
				pErr.Error()))
		return
	}
	uuid := vm.UUID
	object.TargetVMName = &vm.Name
	object.Source.Vmware = &vmio.VirtualMachineImportVmwareSourceSpec{
		VM: vmio.VirtualMachineImportVmwareSourceVMSpec{
			ID: &uuid,
		},
	}
	object.Source.Vmware.Mappings, err = r.mapping(mp, vm)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

//
// Build tasks.
func (r *Builder) Tasks(vmRef ref.Ref) (list []*plan.Task, err error) {
	vm := &model.VM{}
	pErr := r.Inventory.Find(vm, vmRef)
	if pErr != nil {
		err = liberr.New(
			fmt.Sprintf(
				"VM %s lookup failed: %s",
				vmRef.String(),
				pErr.Error()))
		return
	}
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

	return
}

//
// Load (as needed).
func (r *Builder) Load() (err error) {
	list := &api.HostList{}
	err = r.Client.List(
		context.TODO(),
		&client.ListOptions{
			Namespace: r.Provider.Namespace,
		},
		list)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	hostMap := map[string]*api.Host{}
	for _, host := range list.Items {
		ref := host.Spec.Ref
		if !host.Status.HasCondition(libcnd.Ready) {
			continue
		}
		m := &model.Host{}
		pErr := r.Inventory.Find(m, ref)
		if pErr != nil {
			if errors.Is(pErr, web.NotFoundErr) {
				continue
			} else {
				err = liberr.Wrap(pErr)
				return
			}
		}
		ref.ID = m.ID
		ref.Name = m.Name
		hostMap[ref.ID] = &host
	}

	r.hostMap = hostMap

	return
}

//
// Find host ID for VM.
func (r *Builder) hostID(vmRef ref.Ref) (hostID string, err error) {
	vm := &model.VM{}
	pErr := r.Inventory.Find(vm, vmRef)
	if pErr != nil {
		err = liberr.New(
			fmt.Sprintf(
				"VM %s lookup failed: %s",
				vmRef.String(),
				pErr.Error()))
		return
	}

	hostID = vm.Host.ID

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
func (r *Builder) host(hostID string) (host *model.Host, err error) {
	host = &model.Host{}
	pErr := r.Inventory.Get(host, hostID)
	if pErr != nil {
		err = liberr.New(
			fmt.Sprintf(
				"Host %s lookup failed: %s",
				hostID,
				pErr.Error()))
		return
	}

	return
}

//
// Build the VMIO ResourceMapping CR.
func (r *Builder) mapping(in *plan.Map, vm *model.VM) (out *vmio.VmwareMappings, err error) {
	netMap := []vmio.NetworkResourceMappingItem{}
	dsMap := []vmio.StorageResourceMappingItem{}
	for i := range in.Networks {
		mapped := &in.Networks[i]
		ref := mapped.Source
		network := &model.Network{}
		fErr := r.Inventory.Find(network, ref)
		if fErr != nil {
			err = liberr.Wrap(fErr)
			return
		}
		needed := false
		for _, net := range vm.Networks {
			if net.ID == network.ID {
				needed = true
				break
			}
		}
		if !needed {
			continue
		}
		id, pErr := r.networkID(vm, network)
		if pErr != nil {
			err = liberr.Wrap(pErr)
			return
		}
		netMap = append(
			netMap,
			vmio.NetworkResourceMappingItem{
				Source: vmio.Source{
					ID: &id,
				},
				Target: vmio.ObjectIdentifier{
					Namespace: &mapped.Destination.Namespace,
					Name:      mapped.Destination.Name,
				},
				Type: &mapped.Destination.Type,
			})
	}
	for i := range in.Datastores {
		mapped := &in.Datastores[i]
		ref := mapped.Source
		ds := &model.Datastore{}
		fErr := r.Inventory.Find(ds, ref)
		if fErr != nil {
			err = liberr.Wrap(fErr)
			return
		}
		needed := false
		for _, disk := range vm.Disks {
			if disk.Datastore.ID == ds.ID {
				needed = true
				break
			}
		}
		if !needed {
			continue
		}
		id, pErr := r.datastoreID(vm, ds)
		if pErr != nil {
			err = liberr.Wrap(pErr)
			return
		}
		dsMap = append(
			dsMap,
			vmio.StorageResourceMappingItem{
				Source: vmio.Source{
					ID: &id,
				},
				Target: vmio.ObjectIdentifier{
					Name: mapped.Destination.StorageClass,
				},
			})
	}
	out = &vmio.VmwareMappings{
		NetworkMappings: &netMap,
		StorageMappings: &dsMap,
	}

	return
}

//
// Network ID.
// Translated to the ESX host oriented ID as needed.
func (r *Builder) networkID(vm *model.VM, network *model.Network) (id string, err error) {
	if host, found, hErr := r.esxHost(vm); found {
		if hErr != nil {
			err = liberr.Wrap(hErr)
			return
		}
		hostID, hErr := host.networkID(network)
		if hErr != nil {
			err = liberr.Wrap(hErr)
			return
		}
		id = hostID
	} else {
		id = network.ID
	}

	return
}

//
// Datastore ID.
// Translated to the ESX host oriented ID as needed.
func (r *Builder) datastoreID(vm *model.VM, ds *model.Datastore) (id string, err error) {
	if host, found, hErr := r.esxHost(vm); found {
		if hErr != nil {
			err = liberr.Wrap(hErr)
			return
		}
		hostID, hErr := host.DatastoreID(ds)
		if hErr != nil {
			err = liberr.Wrap(hErr)
			return
		}
		id = hostID
	} else {
		id = ds.ID
	}

	return
}

//
// Get ESX host.
// Find may matching a `Host` CR.
func (r *Builder) esxHost(vm *model.VM) (esxHost *EsxHost, found bool, err error) {
	url := r.Provider.Spec.URL
	hostDef, found := r.hostMap[vm.Host.ID]
	if !found {
		return
	}
	hostURL := liburl.URL{
		Scheme: "https",
		Host:   hostDef.Spec.IpAddress,
		Path:   vim25.Path,
	}
	url = hostURL.String()
	secret, err := r.hostSecret(hostDef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	hostModel, nErr := r.host(vm.Host.ID)
	if nErr != nil {
		err = liberr.Wrap(nErr)
		return
	}
	secret.Data["thumbprint"] = []byte(hostModel.Thumbprint)
	esxHost = &EsxHost{
		inventory: r.Inventory,
		secret:    secret,
		url:       url,
	}

	return
}
