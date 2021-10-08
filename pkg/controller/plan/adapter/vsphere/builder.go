package vsphere

import (
	"context"
	"errors"
	"fmt"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	libitr "github.com/konveyor/controller/pkg/itinerary"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	cnv "kubevirt.io/client-go/api/v1"
	cdi "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	liburl "net/url"
	"path"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"strings"
)

// BIOS types
const (
	Efi = "efi"
)

// Bus types
const (
	Virtio = "virtio"
)

// Input types
const (
	Tablet = "tablet"
)

// Network types
const (
	Pod    = "pod"
	Multus = "multus"
)

// Template labels
const (
	TemplateOSLabel       = "os.template.kubevirt.io/%s"
	TemplateWorkloadLabel = "workload.template.kubevirt.io/server"
	TemplateFlavorLabel   = "flavor.template.kubevirt.io/medium"
)

// Operating Systems
const (
	DefaultWindows = "win10"
	DefaultLinux   = "rhel8.1"
	Unknown        = "unknown"
)

// Map of vmware guest ids to osinfo ids.
var osMap = map[string]string{
	"centos64Guest":         "centos5.11",
	"centos6_64Guest":       "centos6.10",
	"centos6Guest":          "centos6.10",
	"centos7_64Guest":       "centos7.0",
	"centos7Guest":          "centos7.0",
	"centos8_64Guest":       "centos8",
	"centos8Guest":          "centos8",
	"debian4_64Guest":       "debian4",
	"debian4Guest":          "debian4",
	"debian5_64Guest":       "debian5",
	"debian5Guest":          "debian5",
	"debian6_64Guest":       "debian6",
	"debian6Guest":          "debian6",
	"debian7_64Guest":       "debian7",
	"debian7Guest":          "debian7",
	"debian8_64Guest":       "debian8",
	"debian8Guest":          "debian8",
	"debian9_64Guest":       "debian9",
	"debian9Guest":          "debian9",
	"debian10_64Guest":      "debian10",
	"debian10Guest":         "debian10",
	"fedora64Guest":         "fedora31",
	"fedoraGuest":           "fedora31",
	"genericLinuxGuest":     "linux",
	"rhel6_64Guest":         "rhel6.10",
	"rhel6Guest":            "rhel6.10",
	"rhel7_64Guest":         "rhel7.7",
	"rhel7Guest":            "rhel7.7",
	"rhel8_64Guest":         "rhel8.1",
	"ubuntu64Guest":         "ubuntu18.04",
	"ubuntuGuest":           "ubuntu18.04",
	"win2000AdvServGuest":   "win2k",
	"win2000ProGuest":       "win2k",
	"win2000ServGuest":      "win2k",
	"windows7Guest":         "win7",
	"windows7Server64Guest": "win2k8r2",
	"windows8_64Guest":      "win8",
	"windows8Guest":         "win8",
	"windows8Server64Guest": "win2k12r2",
	"windows9_64Guest":      "win10",
	"windows9Guest":         "win10",
	"windows9Server64Guest": "win2k19",
}

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
// Create DataVolume certificate configmap.
// No-op for vSphere.
func (r *Builder) ConfigMap(_ ref.Ref, _ *core.Secret, _ *core.ConfigMap) (err error) {
	return
}

//
// Build the DataVolume credential secret.
func (r *Builder) Secret(vmRef ref.Ref, in, object *core.Secret) (err error) {
	hostID, err := r.hostID(vmRef)
	if err != nil {
		return
	}
	if hostDef, found := r.hosts[hostID]; found {
		hostSecret, nErr := r.hostSecret(hostDef)
		if nErr != nil {
			err = nErr
			return
		}
		in = hostSecret
	}

	object.StringData = map[string]string{
		"accessKeyId": string(in.Data["user"]),
		"secretKey":   string(in.Data["password"]),
	}

	return
}

//
// Create DataVolume specs for the VM.
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, _ *core.ConfigMap) (dvs []cdi.DataVolumeSpec, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}

	url := r.Source.Provider.Spec.URL
	thumbprint := string(r.Source.Secret.Data["thumbprint"])
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
		h, nErr := r.host(hostID)
		if nErr != nil {
			err = nErr
			return
		}
		thumbprint = h.Thumbprint
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
		mErr := r.defaultModes(&mapped.Destination)
		if mErr != nil {
			err = mErr
			return
		}
		for _, disk := range vm.Disks {
			if disk.Datastore.ID == ds.ID {
				storageClass := mapped.Destination.StorageClass
				volumeMode := core.PersistentVolumeFilesystem
				if mapped.Destination.VolumeMode != "" {
					volumeMode = mapped.Destination.VolumeMode
				}
				accessMode := core.ReadWriteOnce
				if mapped.Destination.AccessMode != "" {
					accessMode = mapped.Destination.AccessMode
				}
				dvSpec := cdi.DataVolumeSpec{
					Source: cdi.DataVolumeSource{
						VDDK: &cdi.DataVolumeSourceVDDK{
							BackingFile: disk.File,
							UUID:        vm.UUID,
							URL:         url,
							SecretRef:   secret.Name,
							Thumbprint:  thumbprint,
						},
					},
					Storage: &cdi.StorageSpec{
						AccessModes: []core.PersistentVolumeAccessMode{
							accessMode,
						},
						VolumeMode: &volumeMode,
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: *resource.NewQuantity(disk.Capacity, resource.BinarySI),
							},
						},
						StorageClassName: &storageClass,
					},
				}
				dvs = append(dvs, dvSpec)
			}
		}
	}

	return
}

//
// Create the destination Kubevirt VM.
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, dataVolumes []cdi.DataVolume) (err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}
	if vm.IsTemplate {
		err = liberr.New(
			fmt.Sprintf(
				"VM %s is a template",
				vmRef.String()))
		return
	}
	if types.VirtualMachineConnectionState(vm.ConnectionState) != types.VirtualMachineConnectionStateConnected {
		err = liberr.New(
			fmt.Sprintf(
				"VM %s is not connected",
				vmRef.String()))
		return
	}
	if r.Plan.Spec.Warm && !vm.ChangeTrackingEnabled {
		err = liberr.New(
			fmt.Sprintf(
				"Changed Block Tracking (CBT) is disabled for VM %s",
				vmRef.String()))
		return
	}

	host, err := r.host(vm.Host)
	if err != nil {
		return
	}

	object.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	r.mapDisks(vm, dataVolumes, object)
	r.mapFirmware(vm, object)
	r.mapCPU(vm, object)
	r.mapMemory(vm, object)
	r.mapClock(host, object)
	r.mapInput(object)
	err = r.mapNetworks(vm, object)
	if err != nil {
		return
	}

	return
}

func (r *Builder) mapNetworks(vm *model.VM, object *cnv.VirtualMachineSpec) (err error) {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

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
		networkName := fmt.Sprintf("net-%v", i)
		kNetwork := cnv.Network{
			Name: networkName,
		}
		kInterface := cnv.Interface{
			Name:  networkName,
			Model: Virtio,
		}
		switch mapped.Destination.Type {
		case Pod:
			kNetwork.Pod = &cnv.PodNetwork{}
			kInterface.Masquerade = &cnv.InterfaceMasquerade{}
		case Multus:
			kNetwork.Multus = &cnv.MultusNetwork{
				NetworkName: path.Join(mapped.Destination.Namespace, mapped.Destination.Name),
			}
			kInterface.Bridge = &cnv.InterfaceBridge{}
		}
		kNetworks = append(kNetworks, kNetwork)
		kInterfaces = append(kInterfaces, kInterface)
	}
	object.Template.Spec.Networks = kNetworks
	object.Template.Spec.Domain.Devices.Interfaces = kInterfaces
	return
}

func (r *Builder) mapInput(object *cnv.VirtualMachineSpec) {
	tablet := cnv.Input{
		Type: Tablet,
		Name: Tablet,
		Bus:  Virtio,
	}
	object.Template.Spec.Domain.Devices.Inputs = []cnv.Input{tablet}
}

func (r *Builder) mapClock(host *model.Host, object *cnv.VirtualMachineSpec) {
	clock := &cnv.Clock{
		Timer: &cnv.Timer{},
	}
	if host.Timezone != "" {
		tz := cnv.ClockOffsetTimezone(host.Timezone)
		clock.ClockOffset.Timezone = &tz
	}
	object.Template.Spec.Domain.Clock = clock
}

func (r *Builder) mapMemory(vm *model.VM, object *cnv.VirtualMachineSpec) {
	memoryBytes := int64(vm.MemoryMB) * 1024 * 1024
	reservation := resource.NewQuantity(memoryBytes, resource.BinarySI)
	object.Template.Spec.Domain.Resources = cnv.ResourceRequirements{
		Requests: map[core.ResourceName]resource.Quantity{
			core.ResourceMemory: *reservation,
		},
	}
}

func (r *Builder) mapCPU(vm *model.VM, object *cnv.VirtualMachineSpec) {
	object.Template.Spec.Domain.Machine = &cnv.Machine{Type: "q35"}
	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: uint32(vm.CpuCount / vm.CoresPerSocket),
		Cores:   uint32(vm.CoresPerSocket),
	}
}

func (r *Builder) mapFirmware(vm *model.VM, object *cnv.VirtualMachineSpec) {
	features := &cnv.Features{}
	firmware := &cnv.Firmware{
		Serial: vm.UUID,
	}
	switch vm.Firmware {
	case Efi:
		smmEnabled := true
		features.SMM = &cnv.FeatureState{
			Enabled: &smmEnabled,
		}
		firmware.Bootloader = &cnv.Bootloader{EFI: &cnv.EFI{}}
	default:
		firmware.Bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	}
	object.Template.Spec.Domain.Features = features
	object.Template.Spec.Domain.Firmware = firmware
}

func (r *Builder) mapDisks(vm *model.VM, dataVolumes []cdi.DataVolume, object *cnv.VirtualMachineSpec) {
	disks := vm.Disks
	sort.Slice(disks, func(i, j int) bool {
		return disks[i].Key < disks[j].Key
	})
	dvMap := make(map[string]*cdi.DataVolume)
	for i := range dataVolumes {
		dv := &dataVolumes[i]
		dvMap[r.trimBackingFileName(dv.Spec.Source.VDDK.BackingFile)] = dv
	}
	for i, disk := range disks {
		dv := dvMap[r.trimBackingFileName(disk.File)]
		volumeName := fmt.Sprintf("vol-%v", i)
		volume := cnv.Volume{
			Name: volumeName,
			VolumeSource: cnv.VolumeSource{
				DataVolume: &cnv.DataVolumeSource{
					Name: dv.Name,
				},
			},
		}
		kubevirtDisk := cnv.Disk{
			Name: volumeName,
			DiskDevice: cnv.DiskDevice{
				Disk: &cnv.DiskTarget{
					Bus: Virtio,
				},
			},
		}
		object.Template.Spec.Volumes = append(object.Template.Spec.Volumes, volume)
		object.Template.Spec.Domain.Devices.Disks = append(object.Template.Spec.Domain.Devices.Disks, kubevirtDisk)
	}
}

//
// Build tasks.
func (r *Builder) Tasks(vmRef ref.Ref) (list []*plan.Task, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
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

func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}

	var os string
	if vm.GuestID != "" {
		os = osMap[vm.GuestID]
	} else if strings.Contains(vm.GuestName, "linux") || strings.Contains(vm.GuestName, "rhel") {
		os = DefaultLinux
	} else if strings.Contains(vm.GuestName, "win") {
		os = DefaultWindows
	} else {
		os = Unknown
	}

	labels = make(map[string]string)
	labels[fmt.Sprintf(TemplateOSLabel, os)] = "true"
	labels[TemplateWorkloadLabel] = "true"
	labels[TemplateFlavorLabel] = "true"

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
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}

	hostID = vm.Host

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
	err = r.Source.Inventory.Get(host, hostID)
	if err != nil {
		err = liberr.Wrap(
			err,
			"Host lookup failed.",
			"host",
			hostID)
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
