package vsphere

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libitr "github.com/konveyor/controller/pkg/itinerary"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
	"gopkg.in/yaml.v2"
	core "k8s.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	vmio "kubevirt.io/vm-import-operator/pkg/apis/v2v/v1beta1"
	liburl "net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// Regex which matches the snapshot identifier suffix of a
// vSphere disk backing file.
var backingFilePattern = regexp.MustCompile("-\\d+.vmdk")

//
// vSphere builder.
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
	url := r.Source.Provider.Spec.URL
	hostID, err := r.hostID(vmRef)
	if err != nil {
		return
	}
	if hostDef, found := r.hosts[hostID]; found {
		hostURL := liburl.URL{
			Scheme: "https",
			Host:   hostDef.Spec.IpAddress,
			Path:   vim25.Path,
		}
		url = hostURL.String()
		hostSecret, nErr := r.hostSecret(hostDef)
		if nErr != nil {
			err = nErr
			return
		}
		h, nErr := r.host(hostID)
		if nErr != nil {
			err = nErr
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
	if types.VirtualMachineConnectionState(vm.ConnectionState) != types.VirtualMachineConnectionStateConnected {
		err = liberr.New(
			fmt.Sprintf(
				"VM %s is not connected",
				vmRef.String()))
		return
	}
	if r.Plan.Spec.Warm && !vm.changeTrackingEnabled {
		err = liberr.New(
			fmt.Sprintf(
				"Changed Block Tracking (CBT) is disabled for VM %s",
				vmRef.String()))
		return
	}
	uuid := vm.UUID
	object.TargetVMName = &vm.Name
	start := vm.PowerState == string(types.VirtualMachinePowerStatePoweredOn)
	object.StartVM = &start
	object.Source.Vmware = &vmio.VirtualMachineImportVmwareSourceSpec{
		VM: vmio.VirtualMachineImportVmwareSourceVMSpec{
			ID: &uuid,
		},
	}
	object.Source.Vmware.Mappings, err = r.mapping(vm)
	if err != nil {
		return
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
	for _, disk := range vm.Disks {
		mB := disk.Capacity / 0x100000
		list = append(
			list,
			&plan.Task{
				Name: r.trimBackingFileName(disk.File),
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
// Return a stable identifier for a VDDK DataVolume.
func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return r.trimBackingFileName(dv.Spec.Source.VDDK.BackingFile)
}

//
// Load
func (r *Builder) Load() (err error) {
	err = r.loadProvisioners()
	if err != nil {
		return
	}
	err = r.loadHosts()
	if err != nil {
		return
	}

	return
}

//
// Load host CRs.
func (r *Builder) loadHosts() (err error) {
	list := &api.HostList{}
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
	hostMap := map[string]*api.Host{}
	for i := range list.Items {
		host := &list.Items[i]
		ref := host.Spec.Ref
		if !host.Status.HasCondition(libcnd.Ready) {
			continue
		}
		m := &model.Host{}
		pErr := r.Source.Inventory.Find(m, ref)
		if pErr != nil {
			if errors.As(pErr, &web.NotFoundError{}) {
				continue
			} else {
				err = pErr
				return
			}
		}
		ref.ID = m.ID
		ref.Name = m.Name
		hostMap[ref.ID] = host
	}

	r.hosts = hostMap

	return
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

//
// Find host ID for VM.
func (r *Builder) hostID(vmRef ref.Ref) (hostID string, err error) {
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

	hostID = vm.Host.ID

	return
}

//
// Find host CR secret.
func (r *Builder) hostSecret(host *api.Host) (secret *core.Secret, err error) {
	ref := host.Spec.Secret
	secret = &core.Secret{}
	err = r.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		secret)
	err = liberr.Wrap(err)

	return
}

//
// Find host in the inventory.
func (r *Builder) host(hostID string) (host *model.Host, err error) {
	host = &model.Host{}
	pErr := r.Source.Inventory.Get(host, hostID)
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
func (r *Builder) mapping(vm *model.VM) (out *vmio.VmwareMappings, err error) {
	netMap := []vmio.NetworkResourceMappingItem{}
	dsMap := []vmio.StorageResourceMappingItem{}
	netMapIn := r.Context.Map.Network.Spec.Map
	for i := range netMapIn {
		mapped := &netMapIn[i]
		ref := mapped.Source
		network := &model.Network{}
		fErr := r.Source.Inventory.Find(network, ref)
		if fErr != nil {
			err = fErr
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
			err = pErr
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
	dsMapIn := r.Context.Map.Storage.Spec.Map
	for i := range dsMapIn {
		mapped := &dsMapIn[i]
		ref := mapped.Source
		ds := &model.Datastore{}
		fErr := r.Source.Inventory.Find(ds, ref)
		if fErr != nil {
			err = fErr
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
			err = pErr
			return
		}
		mErr := r.defaultModes(&mapped.Destination)
		if mErr != nil {
			err = mErr
			return
		}
		item := vmio.StorageResourceMappingItem{
			Source: vmio.Source{
				ID: &id,
			},
			Target: vmio.ObjectIdentifier{
				Name: mapped.Destination.StorageClass,
			},
		}
		if mapped.Destination.VolumeMode != "" {
			item.VolumeMode = &mapped.Destination.VolumeMode
		}
		/* VMIO > 0.2.5 needed.
		if mapped.Destination.AccessMode != "" {
			item = &mapped.Destination.AccessMode
		}*/
		dsMap = append(dsMap, item)
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
			err = hErr
			return
		}
		hostID, hErr := host.networkID(network)
		if hErr != nil {
			err = hErr
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
			err = hErr
			return
		}
		hostID, hErr := host.DatastoreID(ds)
		if hErr != nil {
			err = hErr
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
	url := r.Source.Provider.Spec.URL
	hostDef, found := r.hosts[vm.Host.ID]
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
		err = nErr
		return
	}
	secret.Data["thumbprint"] = []byte(hostModel.Thumbprint)
	esxHost = &EsxHost{
		Secret: secret,
		URL:    url,
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
// Trims the snapshot suffix from a disk backing file name if there is one.
//	Example:
// 	Input: 	[datastore13] my-vm/disk-name-000015.vmdk
//	Output: [datastore13] my-vm/disk-name.vmdk
func (r *Builder) trimBackingFileName(fileName string) string {
	return backingFilePattern.ReplaceAllString(fileName, ".vmdk")
}
