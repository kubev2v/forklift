package vsphere

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	liburl "net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"k8s.io/klog/v2"

	"github.com/google/uuid"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	utils "github.com/konveyor/forklift-controller/pkg/controller/plan/util"
	container "github.com/konveyor/forklift-controller/pkg/controller/provider/container/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	libcnd "github.com/konveyor/forklift-controller/pkg/lib/condition"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"
	libref "github.com/konveyor/forklift-controller/pkg/lib/ref"
	"github.com/konveyor/forklift-controller/pkg/settings"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BIOS types
const (
	Efi  = "efi"
	BIOS = "bios"
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
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
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
	WindowsPrefix  = "win"
)

// Annotations
const (
	// CDI import backing file annotation on PVC
	AnnImportBackingFile = "cdi.kubevirt.io/storage.import.backingFile"
)

const (
	Shareable = "shareable"
)

// Map of vmware guest ids to osinfo ids.
var osMap = map[string]string{
	"centos64Guest":              "centos5.11",
	"centos6_64Guest":            "centos6.10",
	"centos6Guest":               "centos6.10",
	"centos7_64Guest":            "centos7.0",
	"centos7Guest":               "centos7.0",
	"centos8_64Guest":            "centos8",
	"centos8Guest":               "centos8",
	"debian4_64Guest":            "debian4",
	"debian4Guest":               "debian4",
	"debian5_64Guest":            "debian5",
	"debian5Guest":               "debian5",
	"debian6_64Guest":            "debian6",
	"debian6Guest":               "debian6",
	"debian7_64Guest":            "debian7",
	"debian7Guest":               "debian7",
	"debian8_64Guest":            "debian8",
	"debian8Guest":               "debian8",
	"debian9_64Guest":            "debian9",
	"debian9Guest":               "debian9",
	"debian10_64Guest":           "debian10",
	"debian10Guest":              "debian10",
	"fedora64Guest":              "fedora31",
	"fedoraGuest":                "fedora31",
	"genericLinuxGuest":          "linux",
	"rhel6_64Guest":              "rhel6.10",
	"rhel6Guest":                 "rhel6.10",
	"rhel7_64Guest":              "rhel7.7",
	"rhel7Guest":                 "rhel7.7",
	"rhel8_64Guest":              "rhel8.1",
	"rhel9_64Guest":              "rhel9.4",
	"ubuntu64Guest":              "ubuntu18.04",
	"ubuntuGuest":                "ubuntu18.04",
	"win2000AdvServGuest":        "win2k",
	"win2000ProGuest":            "win2k",
	"win2000ServGuest":           "win2k",
	"windows7Guest":              "win7",
	"windows7Server64Guest":      "win2k8r2",
	"windows8_64Guest":           "win8",
	"windows8Guest":              "win8",
	"windows8Server64Guest":      "win2k12r2",
	"windows9_64Guest":           "win10",
	"windows9Guest":              "win10",
	"windows9Server64Guest":      "win2k19",
	"windows2019srv_64Guest":     "win2k19",
	"windows2019srvNext_64Guest": "win2k19",
	"windows2022srvNext_64Guest": "win2k22",
}

// Global list of legacy guest OS identifiers and names (OSes without native SHA-2 support)
var legacyIdentifiers = []string{
	"windows xp",
	"winXPProGuest",
	"server 2003",
	"winNetEnterpriseGuest",
	"winNetStandardGuest",
	"winNetEnterprise64Guest",
	"vista",
	"windowsVistaGuest",
	"server 2008",
	"longhornGuest",
	"windows 7",
	"windows7Guest",
	"server 2008 r2",
	"windows7Server64Guest",
}

// Regex which matches the snapshot identifier suffix of a
// vSphere disk backing file.
var backingFilePattern = regexp.MustCompile(`-\d\d\d\d\d\d.vmdk`)

// vSphere builder.
type Builder struct {
	*plancontext.Context
	// Host CRs.
	hosts map[string]*api.Host
	// MAC addresses already in use on the destination cluster. k=mac, v=vmName
	macConflictsMap map[string]string
}

// Get list of destination VMs with mac addresses that would
// conflict with this VM, if any exist.
func (r *Builder) macConflicts(vm *model.VM) (conflictingVMs []string, err error) {
	if r.macConflictsMap == nil {
		list := []ocp.VM{}
		err = r.Destination.Inventory.List(&list, base.Param{
			Key:   base.DetailParam,
			Value: "all",
		})
		if err != nil {
			return
		}

		r.macConflictsMap = make(map[string]string)
		for _, kVM := range list {
			for _, iface := range kVM.Object.Spec.Template.Spec.Domain.Devices.Interfaces {
				r.macConflictsMap[iface.MacAddress] = path.Join(kVM.Namespace, kVM.Name)
			}
		}
	}

	for _, nic := range vm.NICs {
		if conflictingVm, found := r.macConflictsMap[nic.MAC]; found {
			for i := range conflictingVMs {
				// ignore duplicates
				if conflictingVMs[i] == conflictingVm {
					continue
				}
			}
			conflictingVMs = append(conflictingVMs, conflictingVm)
		}
	}

	return
}

// Create DataVolume certificate configmap.
// No-op for vSphere.
func (r *Builder) ConfigMap(_ ref.Ref, _ *core.Secret, _ *core.ConfigMap) (err error) {
	return
}

func IsLegacyWindows(vm *model.VM) bool {

	guestID := strings.ToLower(vm.GuestID)
	guestName := strings.ToLower(vm.GuestName)

	for _, id := range legacyIdentifiers {
		if strings.Contains(guestID, id) || strings.Contains(guestName, id) {
			return true
		}
	}
	return false
}

func (r *Builder) PodEnvironment(vmRef ref.Ref, sourceSecret *core.Secret) (env []core.EnvVar, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	if !r.Context.Plan.Spec.MigrateSharedDisks {
		vm.RemoveSharedDisks()
	}
	macsToIps := ""
	if r.Plan.Spec.PreserveStaticIPs {
		macsToIps, err = r.mapMacStaticIps(vm)
		if err != nil {
			return
		}

		env = append(env, core.EnvVar{
			Name:  "V2V_preserveStaticIPs",
			Value: "true",
		})
	}

	useLegacyDrivers := false
	if r.Plan.Spec.InstallLegacyDrivers == nil {
		useLegacyDrivers = IsLegacyWindows(vm)
	} else {
		useLegacyDrivers = *r.Plan.Spec.InstallLegacyDrivers
	}

	if useLegacyDrivers {
		env = append(env, core.EnvVar{
			Name:  "VIRTIO_WIN",
			Value: "/usr/local/virtio-win-legacy.iso",
		})
	}

	libvirtURL, fingerprint, err := r.getSourceDetails(vm, sourceSecret)
	if err != nil {
		return
	}

	env = append(
		env,
		core.EnvVar{
			Name:  "V2V_vmName",
			Value: vm.Name,
		},
		core.EnvVar{
			Name:  "V2V_libvirtURL",
			Value: libvirtURL.String(),
		},
		core.EnvVar{
			Name:  "V2V_source",
			Value: "vSphere",
		},
		// The fingerpint/thumbprint is not confidential since one can retrieve
		// it from the server as we do, so we don't have to place it in a secret
		core.EnvVar{
			Name:  "V2V_fingerprint",
			Value: fingerprint,
		},
		core.EnvVar{
			Name:  "V2V_extra_args",
			Value: settings.Settings.Migration.VirtV2vExtraArgs,
		},
	)
	if macsToIps != "" {
		env = append(env, core.EnvVar{
			Name:  "V2V_staticIPs",
			Value: macsToIps,
		})
	}
	return
}

func (r *Builder) mapMacStaticIps(vm *model.VM) (ipMap string, err error) {
	// on windows machines we check if the interface origin is manual
	// on linux we collect all networks.
	isWindowsFlag := isWindows(vm)

	var configurations []string
	for _, guestNetwork := range vm.GuestNetworks {
		if !isWindowsFlag || guestNetwork.Origin == string(types.NetIpConfigInfoIpAddressOriginManual) {
			gateway := ""
			isIpv4 := net.IP.To4(net.ParseIP(guestNetwork.IP)) != nil
			for _, ipStack := range vm.GuestIpStacks {
				gwIpv4 := net.IP.To4(net.ParseIP(ipStack.Gateway)) != nil
				if gwIpv4 && !isIpv4 || !gwIpv4 && isIpv4 {
					// not the right IPv4 / IPv6 correlation
					continue
				}
				if ipStack.Device != guestNetwork.Device {
					continue
				}
				if ipStack.Network != "0.0.0.0" {
					continue
				}
				gateway = ipStack.Gateway
			}
			dnsString := strings.Join(guestNetwork.DNS, ",")
			configurationString := fmt.Sprintf("%s:ip:%s,%s,%d,%s", guestNetwork.MAC, guestNetwork.IP, gateway, guestNetwork.PrefixLength, dnsString)

			// if DNS is "", we get configurationString with trailing comma, use TrimSuffix to remove it.
			configurations = append(configurations, strings.TrimSuffix(configurationString, ","))
		}
	}
	return strings.Join(configurations, "_"), nil
}

func isWindows(vm *model.VM) bool {
	return strings.Contains(vm.GuestID, WindowsPrefix) || strings.Contains(vm.GuestName, WindowsPrefix)
}

func (r *Builder) getSourceDetails(vm *model.VM, sourceSecret *core.Secret) (libvirtURL liburl.URL, fingerprint string, err error) {
	host, err := r.host(vm.Host)
	if err != nil {
		return
	}

	sslVerify := ""
	if container.GetInsecureSkipVerifyFlag(sourceSecret) {
		sslVerify = "no_verify=1"
	}

	if hostDef, found := r.hosts[host.ID]; found {
		// Connect through ESXi
		var hostSecret *core.Secret
		if hostSecret, err = r.hostSecret(hostDef); err != nil {
			return
		}
		libvirtURL = liburl.URL{
			Scheme:   "esx",
			Host:     hostDef.Spec.IpAddress,
			User:     liburl.User(string(hostSecret.Data["user"])),
			Path:     "",
			RawQuery: sslVerify,
		}
		if r.Source.Provider.Spec.Settings[api.SDK] == api.ESXI {
			// the SDK of ESXi doesn't return a fingerprint/thumbprint for the host
			// so we take it from the provider instead
			fingerprint = r.Source.Provider.Status.Fingerprint
		} else {
			fingerprint = host.Thumbprint
		}
	} else if r.Source.Provider.Spec.Settings[api.SDK] == api.ESXI {
		libvirtURL = liburl.URL{
			Scheme:   "esx",
			Host:     host.Name,
			User:     liburl.User(string(sourceSecret.Data["user"])),
			Path:     "",
			RawQuery: sslVerify,
		}
		fingerprint = r.Source.Provider.Status.Fingerprint
	} else {
		// Connect through VCenter
		path := host.Path
		// Check parent resource
		if host.Parent.Kind == "Cluster" {
			parent := &model.Cluster{}
			if err = r.Source.Inventory.Get(parent, host.Parent.ID); err != nil {
				err = liberr.Wrap(err, "cluster", host.Parent.ID)
				return
			}
			if parent.Variant == "ComputeResource" {
				// This is a stand-alone host without a cluster. We
				// need to use path to the parent resource instead.
				path = parent.Path
			}
		}
		var url *liburl.URL
		if url, err = liburl.Parse(r.Source.Provider.Spec.URL); err != nil {
			err = liberr.Wrap(err)
			return
		}
		libvirtURL = liburl.URL{
			Scheme:   "vpx",
			Host:     url.Host,
			User:     liburl.User(string(sourceSecret.Data["user"])),
			Path:     path, // E.g.: /Datacenter/Cluster/host.example.com
			RawQuery: sslVerify,
		}
		fingerprint = r.Source.Provider.Status.Fingerprint
	}

	return
}

// Build the DataVolume credential secret.
func (r *Builder) Secret(vmRef ref.Ref, in, object *core.Secret) (err error) {
	hostID, err := r.hostID(vmRef)
	if err != nil {
		return
	}
	if hostDef, found := r.hosts[hostID]; found {
		if in, err := r.hostSecret(hostDef); err == nil {
			object.Data = map[string][]byte{
				"accessKeyId": in.Data["user"],
				"secretKey":   in.Data["password"],
			}
		} else {
			return err
		}
	} else {
		object.Data = map[string][]byte{
			"accessKeyId": in.Data["user"],
			"secretKey":   in.Data["password"],
		}
	}
	if cacert, ok := in.Data["cacert"]; ok {
		object.Data["cacert"] = cacert
	}
	return
}

// buildDatastoreMap builds a map of storage mappings keyed by source datastore ID
func (r *Builder) buildDatastoreMap() (map[string]*api.StoragePair, error) {
	dsMap := make(map[string]*api.StoragePair)
	dsMapIn := r.Context.Map.Storage.Spec.Map

	for i := range dsMapIn {
		mapped := &dsMapIn[i]
		ref := mapped.Source
		ds := &model.Datastore{}
		err := r.Source.Inventory.Find(ds, ref)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
		dsMap[ds.ID] = mapped
	}

	return dsMap, nil
}

// Create DataVolume specs for the VM.
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, _ *core.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *core.ConfigMap) (dvs []cdi.DataVolume, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	if !r.Context.Plan.Spec.MigrateSharedDisks {
		vm.RemoveSharedDisks()
	}
	url := r.Source.Provider.Spec.URL
	thumbprint := r.Source.Provider.Status.Fingerprint
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

	// Build datastore map for more efficient lookups
	dsMap, err := r.buildDatastoreMap()
	if err != nil {
		return
	}

	// Sort disks by bus, so we can match the disk index to the boot order.
	// Important: need to match order in mapDisks method
	disks := r.sortedDisksAsVmware(vm.Disks)

	for diskIndex, disk := range disks {
		mapped, found := dsMap[disk.Datastore.ID]
		if !found {
			continue
		}

		storageClass := mapped.Destination.StorageClass
		var dvSource cdi.DataVolumeSource
		useV2vForTransfer, vErr := r.Context.Plan.ShouldUseV2vForTransfer()
		if vErr != nil {
			err = vErr
			return
		}
		if useV2vForTransfer {
			// Let virt-v2v do the copying
			dvSource = cdi.DataVolumeSource{
				Blank: &cdi.DataVolumeBlankImage{},
			}
		} else {
			// Let CDI do the copying
			dvSource = cdi.DataVolumeSource{
				VDDK: &cdi.DataVolumeSourceVDDK{
					BackingFile:  baseVolume(disk.File, r.Plan.Spec.Warm),
					UUID:         vm.UUID,
					URL:          url,
					SecretRef:    secret.Name,
					Thumbprint:   thumbprint,
					InitImageURL: r.Source.Provider.Spec.Settings[api.VDDK],
				},
			}
		}
		dvSpec := cdi.DataVolumeSpec{
			Source: &dvSource,
			Storage: &cdi.StorageSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: *resource.NewQuantity(disk.Capacity, resource.BinarySI),
					},
				},
				StorageClassName: &storageClass,
			},
		}
		// set the access mode and volume mode if they were specified in the storage map.
		// otherwise, let the storage profile decide the default values.
		if mapped.Destination.AccessMode != "" {
			dvSpec.Storage.AccessModes = []core.PersistentVolumeAccessMode{mapped.Destination.AccessMode}
		}
		if mapped.Destination.VolumeMode != "" {
			dvSpec.Storage.VolumeMode = &mapped.Destination.VolumeMode
		}

		dv := dvTemplate.DeepCopy()
		dv.Spec = dvSpec
		if dv.ObjectMeta.Annotations == nil {
			dv.ObjectMeta.Annotations = make(map[string]string)
		}
		dv.ObjectMeta.Annotations[planbase.AnnDiskSource] = baseVolume(disk.File, r.Plan.Spec.Warm)
		if disk.Shared {
			dv.ObjectMeta.Labels[Shareable] = "true"
		}

		// Preserve the disk index as an annotation on the created DataVolume
		// Note: this annotation will be used to match the PVC to the VM disks by
		//       matching the disk and PVC index.
		dv.ObjectMeta.Annotations[planbase.AnnDiskIndex] = fmt.Sprintf("%d", diskIndex)

		// if exists, get the PVC generate name from the PlanSpec, generate the name
		// and update the GenerateName field in the DataVolume object.
		pvcNameTemplate := r.getPVCNameTemplate(vm)
		if pvcNameTemplate != "" {
			// Get the VM root disk index
			planVM := r.getPlanVM(vm)
			rootDiskIndex := 0
			if planVM != nil {
				rootDiskIndex = utils.GetBootDiskNumber(planVM.RootDisk)
			}

			// Create template data
			templateData := api.PVCNameTemplateData{
				VmName:        r.getPlenVMSafeName(vm),
				PlanName:      r.Plan.Name,
				DiskIndex:     diskIndex,
				RootDiskIndex: rootDiskIndex,
			}

			generatedName, err := r.executeTemplate(pvcNameTemplate, &templateData)
			useGenerateName := r.Plan.Spec.PVCNameTemplateUseGenerateName

			// Validate that template output is a valid k8s label
			errs := k8svalidation.IsDNS1123Label(generatedName)
			if len(errs) > 0 {
				// Raise an error if the generated name is not valid
				err = errors.New("generated PVC name is not valid")

				// Failed to generate PVC name using template
				r.Log.Info("Failed to generate PVC name using template", "template", pvcNameTemplate, "error", err)
			}

			if err == nil && generatedName != "" {
				if useGenerateName {
					// Ensure generatedName ends with "-"
					if !strings.HasSuffix(generatedName, "-") {
						generatedName = generatedName + "-"
					}
					dv.ObjectMeta.GenerateName = generatedName
				} else {
					// Ensure generatedName does not end with "-"
					if strings.HasSuffix(generatedName, "-") {
						generatedName = strings.Trim(generatedName, "-")
					}

					// Use the generated name as the PVC name
					dv.ObjectMeta.Name = generatedName
				}
			} else {
				// Failed to generate PVC name using template
				r.Log.Info("Failed to generate PVC name using template", "template", pvcNameTemplate, "error", err)
			}
		}

		if !useV2vForTransfer && vddkConfigMap != nil {
			dv.ObjectMeta.Annotations[planbase.AnnVddkExtraArgs] = vddkConfigMap.Name
		}
		dvs = append(dvs, *dv)
	}

	return
}

// Create the destination Kubevirt VM.
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) (err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
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
	if !r.Context.Plan.Spec.MigrateSharedDisks {
		sharedPVCs, missingDiskPVCs, err := findSharedPVCs(r.Destination.Client, vm, r.Plan.Spec.TargetNamespace)
		if err != nil {
			return liberr.Wrap(err)
		}
		for _, disk := range missingDiskPVCs {
			// This is one of the last steps of migration we should not fail as users can migrate the disk later and reattach it manually.
			r.Log.Error(err, "Failed to find shared PVCs", "vm", vmRef.String(), "disk", disk.File)
			vm.RemoveDisk(disk)
		}
		if sharedPVCs != nil {
			persistentVolumeClaims = append(persistentVolumeClaims, sharedPVCs...)
		}
	}

	var conflicts []string
	conflicts, err = r.macConflicts(vm)
	if err != nil {
		return
	}
	if len(conflicts) > 0 {
		err = liberr.New(
			fmt.Sprintf("Source VM has a mac address conflict with one or more destination VMs: %s", conflicts))
		return
	}

	host, err := r.host(vm.Host)
	if err != nil {
		return
	}

	if object.Template == nil {
		object.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}
	err = r.mapDisks(vm, vmRef, persistentVolumeClaims, object, sortVolumesByLibvirt)
	if err != nil {
		return
	}
	r.mapFirmware(vm, object)
	if !usesInstanceType {
		r.mapCPU(vm, object)
		r.mapMemory(vm, object)
	}
	r.mapClock(host, object)
	r.mapInput(object)
	r.mapTpm(vm, object)
	err = r.mapNetworks(vm, object)
	if err != nil {
		return
	}

	return
}

func (r *Builder) mapNetworks(vm *model.VM, object *cnv.VirtualMachineSpec) (err error) {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

	numNetworks := 0
	netMapIn := r.Context.Map.Network.Spec.Map
	for i := range netMapIn {
		mapped := &netMapIn[i]

		// Skip network mappings with destination type 'Ignored'
		if mapped.Destination.Type == Ignored {
			continue
		}

		ref := mapped.Source
		network := &model.Network{}
		fErr := r.Source.Inventory.Find(network, ref)
		if fErr != nil {
			err = fErr
			return
		}

		needed := []vsphere.NIC{}
		for _, nic := range vm.NICs {
			switch network.Variant {
			case vsphere.NetDvPortGroup, vsphere.OpaqueNetwork:
				if nic.Network.ID == network.Key {
					needed = append(needed, nic)
				}
			default:
				if nic.Network.ID == network.ID {
					needed = append(needed, nic)
				}
			}
		}
		if len(needed) == 0 {
			continue
		}
		for _, nic := range needed {
			networkName := fmt.Sprintf("net-%v", numNetworks)

			// If the network name template is set, use it to generate the network name.
			networkNameTemplate := r.getNetworkNameTemplate(vm)
			if networkNameTemplate != "" {
				// Create template data
				templateData := api.NetworkNameTemplateData{
					NetworkName:      mapped.Destination.Name,
					NetworkNamespace: mapped.Destination.Namespace,
					NetworkType:      mapped.Destination.Type,
					NetworkIndex:     numNetworks,
				}

				networkName, err = r.executeTemplate(networkNameTemplate, &templateData)
				if err != nil {
					// Failed to generate network name using template
					r.Log.Info("Failed to generate network name using template, using default name", "template", networkNameTemplate, "error", err)

					// Fallback to default name and reset error
					networkName = fmt.Sprintf("net-%v", numNetworks)
					err = nil
				}
			}

			numNetworks++
			kNetwork := cnv.Network{
				Name: networkName,
			}
			kInterface := cnv.Interface{
				Name:       networkName,
				Model:      Virtio,
				MacAddress: nic.MAC,
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
	if host.Timezone != "" {
		if object.Template.Spec.Domain.Clock == nil {
			object.Template.Spec.Domain.Clock = &cnv.Clock{}
		}
		tz := cnv.ClockOffsetTimezone(host.Timezone)
		object.Template.Spec.Domain.Clock.ClockOffset.Timezone = &tz
	}
}

func (r *Builder) mapMemory(vm *model.VM, object *cnv.VirtualMachineSpec) {
	memoryBytes := int64(vm.MemoryMB) * 1024 * 1024
	reservation := resource.NewQuantity(memoryBytes, resource.BinarySI)
	object.Template.Spec.Domain.Memory = &cnv.Memory{Guest: reservation}
}

func (r *Builder) mapCPU(vm *model.VM, object *cnv.VirtualMachineSpec) {
	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: uint32(vm.CpuCount / vm.CoresPerSocket),
		Cores:   uint32(vm.CoresPerSocket),
	}
}

func (r *Builder) mapFirmware(vm *model.VM, object *cnv.VirtualMachineSpec) {
	firmware := &cnv.Firmware{
		Serial: vm.UUID,
	}
	switch vm.Firmware {
	case Efi:
		firmware.Bootloader = &cnv.Bootloader{
			EFI: &cnv.EFI{
				SecureBoot: &vm.SecureBoot,
			}}
		if vm.SecureBoot {
			object.Template.Spec.Domain.Features = &cnv.Features{
				SMM: &cnv.FeatureState{
					Enabled: &vm.SecureBoot,
				},
			}
		}
	default:
		firmware.Bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	}
	object.Template.Spec.Domain.Firmware = firmware
}

func (r *Builder) filterDisksWithBus(disks []vsphere.Disk, bus string) []vsphere.Disk {
	var resp []vsphere.Disk
	for _, disk := range disks {
		if disk.Bus == bus {
			resp = append(resp, disk)
		}
	}
	return resp
}

// The disks are first sorted by the buses going in order SCSI, SATA and IDE and within the controller the
// disks are sorted by the key. This needs to be done because the virt-v2v outputs the files in an order,
// which it gets from libvirt. The libvirt orders the devices starting with SCSI, SATA and IDE.
// When we were sorting by the keys the order was IDE, SATA and SCSI. This cause that some PVs were populated by
// incorrect disks.
// https://github.com/libvirt/libvirt/blob/master/src/vmx/vmx.c#L1713
func (r *Builder) sortedDisksByBusses(disks []vsphere.Disk, buses []string) []vsphere.Disk {
	var resp []vsphere.Disk
	for _, bus := range buses {
		disksWithBus := r.filterDisksWithBus(disks, bus)
		sort.Slice(disksWithBus, func(i, j int) bool {
			return disksWithBus[i].Key < disksWithBus[j].Key
		})
		resp = append(resp, disksWithBus...)
	}
	return resp
}

func (r *Builder) sortedDisksAsLibvirt(disks []vsphere.Disk) []vsphere.Disk {
	var buses = []string{container.SCSI, container.SATA, container.IDE}
	return r.sortedDisksByBusses(disks, buses)
}

func (r *Builder) sortedDisksAsVmware(disks []vsphere.Disk) []vsphere.Disk {
	var buses = []string{container.SATA, container.IDE, container.SCSI}
	return r.sortedDisksByBusses(disks, buses)
}

func (r *Builder) mapDisks(vm *model.VM, vmRef ref.Ref, persistentVolumeClaims []*core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec, sortByLibvirt bool) error {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk
	var templateErr error
	var disks []vsphere.Disk

	if sortByLibvirt {
		disks = r.sortedDisksAsLibvirt(vm.Disks)
	} else {
		disks = r.sortedDisksAsVmware(vm.Disks)
	}
	pvcMap := make(map[string]*core.PersistentVolumeClaim)
	for i := range persistentVolumeClaims {
		pvc := persistentVolumeClaims[i]
		// the PVC BackingFile value has already been trimmed.
		if source, ok := pvc.Annotations[planbase.AnnDiskSource]; ok {
			pvcMap[trimBackingFileName(source)] = pvc
		} else {
			pvcMap[trimBackingFileName(pvc.Annotations[AnnImportBackingFile])] = pvc
		}
	}

	var bootDisk int
	for _, vmConf := range r.Plan.Spec.VMs {
		if vmConf.ID == vmRef.ID {
			bootDisk = utils.GetBootDiskNumber(vmConf.RootDisk)
			break
		}
	}

	for i, disk := range disks {
		// If the user creates in middle of migration snapshot the disk file name gets the snapshot suffix.
		// This is updated in the inventory as it's the current disk state, but all PVCs and DVs were created with
		// the original name. The trim will remove the suffix from the disk name showing the original name.
		pvc := pvcMap[trimBackingFileName(disk.File)]
		if pvc == nil {
			return fmt.Errorf("failed to find persistent volume for disk %s", disk.File)
		}
		volumeName := fmt.Sprintf("vol-%v", i)

		// If the volume name template is set, use it to generate the volume name.
		volumeNameTemplate := r.getVolumeNameTemplate(vm)
		if volumeNameTemplate != "" {
			// Create template data
			templateData := api.VolumeNameTemplateData{
				PVCName:     pvc.Name,
				VolumeIndex: i,
			}

			volumeName, templateErr = r.executeTemplate(volumeNameTemplate, &templateData)
			if templateErr != nil {
				// Failed to generate volume name using template
				r.Log.Info("Failed to generate volume name using template, using default name", "template", volumeNameTemplate, "error", templateErr)

				// fallback to default name and reset error
				volumeName = fmt.Sprintf("vol-%v", i)
			}

			// check if the generated name is longer then 63 characters
			if len(volumeName) > 63 {
				// if the generated name is longer then 63 characters, trancate it
				newVolumeName := volumeName[:63]
				r.Log.Info("Generated volume name is longer than 63 characters, sanitizing it", "volumeName", volumeName, "newVolumeName", newVolumeName)

				volumeName = newVolumeName
			}
		}

		volume := cnv.Volume{
			Name: volumeName,
			VolumeSource: cnv.VolumeSource{
				PersistentVolumeClaim: &cnv.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: core.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			},
		}
		kubevirtDisk := cnv.Disk{
			Name: volumeName,
			DiskDevice: cnv.DiskDevice{
				Disk: &cnv.DiskTarget{
					Bus: cnv.DiskBusVirtio,
				},
			},
		}
		if disk.Shared {
			kubevirtDisk.Shareable = ptr.To(true)
			kubevirtDisk.Cache = cnv.CacheNone
		}
		kVolumes = append(kVolumes, volume)
		kDisks = append(kDisks, kubevirtDisk)
	}
	// For multiboot VMs, if the selected boot device is the current disk,
	// set it as the first in the boot order.
	kDisks[bootDisk].BootOrder = ptr.To(uint(1))

	object.Template.Spec.Volumes = kVolumes
	object.Template.Spec.Domain.Devices.Disks = kDisks
	return nil
}

func (r *Builder) mapTpm(vm *model.VM, object *cnv.VirtualMachineSpec) {
	if vm.TpmEnabled {
		persistData := true
		object.Template.Spec.Domain.Devices.TPM = &cnv.TPMDevice{Persistent: &persistData}
	}
}

// Build tasks.
func (r *Builder) Tasks(vmRef ref.Ref) (list []*plan.Task, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	if !r.Context.Plan.Spec.MigrateSharedDisks {
		vm.RemoveSharedDisks()
	}
	for _, disk := range vm.Disks {
		mB := disk.Capacity / 0x100000
		list = append(
			list,
			&plan.Task{
				Name: baseVolume(disk.File, r.Plan.Spec.Warm),
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

func (r *Builder) PreferenceName(vmRef ref.Ref, configMap *core.ConfigMap) (name string, err error) {
	var os string
	for _, vmConf := range r.Migration.Status.VMs {
		if vmConf.ID == vmRef.ID {
			os = vmConf.OperatingSystem
			break
		}
	}
	name, ok := configMap.Data[os]
	if !ok {
		err = liberr.Wrap(err, "vm", vmRef.String())
	}
	return
}

func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	var os string
	for _, vmConf := range r.Migration.Status.VMs {
		if vmConf.ID == vmRef.ID {
			os = vmConf.OperatingSystem
			break
		}
	}

	if os != "" {
		os = osMap[os]
	} else if strings.Contains(os, "linux") || strings.Contains(os, "rhel") {
		os = DefaultLinux
	} else if strings.Contains(os, WindowsPrefix) {
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

// Return a stable identifier for a VDDK DataVolume.
func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return baseVolume(dv.ObjectMeta.Annotations[planbase.AnnDiskSource], r.Plan.Spec.Warm)
}

// Return a stable identifier for a PersistentDataVolume.
func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	return baseVolume(pvc.Annotations[AnnImportBackingFile], r.Plan.Spec.Warm)
}

// Load
func (r *Builder) Load() (err error) {
	err = r.loadHosts()
	if err != nil {
		return
	}

	return
}

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
		if !libref.Equals(&host.Spec.Provider, &r.Plan.Spec.Provider.Source) {
			continue
		}

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

// Find host ID for VM.
func (r *Builder) hostID(vmRef ref.Ref) (hostID string, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	hostID = vm.Host

	return
}

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

// Find host in the inventory.
func (r *Builder) host(hostID string) (host *model.Host, err error) {
	host = &model.Host{}
	err = r.Source.Inventory.Get(host, hostID)
	if err != nil {
		err = liberr.Wrap(err, "host", hostID)
	}

	return
}

// Build LUN PVs.
func (r *Builder) LunPersistentVolumes(vmRef ref.Ref) (pvs []core.PersistentVolume, err error) {
	// do nothing
	return
}

// Build LUN PVCs.
func (r *Builder) LunPersistentVolumeClaims(vmRef ref.Ref) (pvcs []core.PersistentVolumeClaim, err error) {
	// do nothing
	return
}

// FIXME rgolan - the behaviour needs to be per disk hense this method is flawed. Needs a bigger change.
// For now this method returns true, if there's a mapping (backend by copy-offload-mapping ConfigMap, that
// maps StoragetClasses to Vsphere data stores
func (r *Builder) SupportsVolumePopulators() bool {
	if !settings.Settings.Features.CopyOffload || r.Plan.Spec.Warm {
		return false
	}
	dsMapIn := r.Context.Map.Storage.Spec.Map
	for _, m := range dsMapIn {
		ref := m.Source
		ds := &model.Datastore{}
		err := r.Source.Inventory.Find(ds, ref)
		if err != nil {
			klog.Errorf("failed to get datastore to detect volume populators support: %s", err)
			return false
		}

		if m.OffloadPlugin != nil && m.OffloadPlugin.VSphereXcopyPluginConfig != nil {
			klog.V(2).Infof("found offload plugin: config %+v on ds map  %+v", m.OffloadPlugin.VSphereXcopyPluginConfig, dsMapIn)
			return true

		}
	}
	return false
}

// PopulatorVolumes creates PVC in case the their are needed for the disks
// in context, and according to the offload plugin configuration in the StorageMap
func (r *Builder) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) (pvcs []*core.PersistentVolumeClaim, err error) {
	vm := &model.VM{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
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

		pvblock := core.PersistentVolumeBlock
		for _, disk := range vm.Disks {
			if disk.Datastore.ID == ds.ID {
				storageClass := mapped.Destination.StorageClass

				r.Log.Info(fmt.Sprintf("getting storage mapping by storage class %q and datastore %v datastore name %s datastore", storageClass, disk.Datastore, disk.Datastore))
				vsphereInstance := r.Context.Plan.Provider.Source.GetName()
				storageVendorProduct := mapped.OffloadPlugin.VSphereXcopyPluginConfig.StorageVendorProduct
				storageVendorSecretRef := mapped.OffloadPlugin.VSphereXcopyPluginConfig.SecretRef

				r.Log.Info(fmt.Sprintf("vsphere provider %v storage vendor product %v storage secret name %v ", vsphereInstance, storageVendorProduct, storageVendorSecretRef))

				if vsphereInstance == "" || storageVendorProduct == "" || storageVendorSecretRef == "" {
					return nil, fmt.Errorf(
						"The offload pluging configuration has missing details. Can't continue with PVC and populator resources creation.")
				}

				namespace := r.Plan.Spec.TargetNamespace
				// pvs names needs to be less than 63, this leaves 53 chars
				// for the plan and vm name (2 dashes and 8 chars uuid)
				commonName := fmt.Sprintf("%s-%s-%s", r.Plan.Name, vm.Name, uuid.New().String()[:8])
				labels := map[string]string{
					"migration": string(r.Migration.UID),
					// we need uniqness and a value which is less than 64 chars, hence using vmRef.id + disk.key
					"vmdkKey": fmt.Sprint(disk.Key),
					"vmID":    vmRef.ID,
				}
				r.Log.Info("target namespace for migration", "namespace", namespace)
				pvc := core.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:        commonName,
						Namespace:   namespace,
						Labels:      labels,
						Annotations: annotations,
					},
					Spec: core.PersistentVolumeClaimSpec{
						AccessModes:      []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
						StorageClassName: &storageClass,
						VolumeMode:       &pvblock,
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: *resource.NewQuantity(disk.Capacity, resource.BinarySI),
							},
						},
						DataSourceRef: &core.TypedObjectReference{
							APIGroup: &api.SchemeGroupVersion.Group,
							Kind:     api.VSphereXcopyVolumePopulatorKind,
							Name:     commonName,
						},
					},
				}

				if annotations == nil {
					pvc.Annotations = make(map[string]string)
				} else {
					pvc.Annotations = annotations
				}
				pvc.Annotations[planbase.AnnDiskSource] = baseVolume(disk.File, false)
				pvc.Annotations["copy-offload"] = baseVolume(disk.File, false)
				pvcs = append(pvcs, &pvc)

				vp := api.VSphereXcopyVolumePopulator{
					ObjectMeta: metav1.ObjectMeta{
						Name:      commonName,
						Namespace: namespace,
						Labels:    labels,
					},
					Spec: api.VSphereXcopyVolumePopulatorSpec{
						VmdkPath:             disk.File,
						SecretName:           secretName,
						StorageVendorProduct: string(storageVendorProduct),
					},
				}

				// Ensure a Secret combining Vsphere and Storage secrets
				err = r.mergeSecrets(secretName, namespace, storageVendorSecretRef, r.Source.Provider.Namespace)
				if err != nil {
					return nil, fmt.Errorf("failed to merge secrets for popoulators %w", err)
				}
				// TODO should we handle if already exists due to re-entry? if the former
				// reconcile was successful in creating the pvc but failed after that, e.g when
				// creating the volumepopulator resouce failed
				r.Log.Info("Creating pvc", "pvc", pvc)
				err = r.Destination.Client.Create(context.TODO(), &pvc, &client.CreateOptions{})
				if err != nil {
					// ignore if already exists?
					return nil, err
				}

				r.Log.Info("Ensuring a populator service account")
				err := r.ensurePopulatorServiceAccount(namespace)
				if err != nil {
					return nil, err
				}
				r.Log.Info("Creating the populator resource", "VSphereXcopyVolumePopulator", vp)
				err = r.Destination.Client.Create(context.TODO(), &vp, &client.CreateOptions{})
				if err != nil {
					return nil, err
				}

			}
		}
	}

	return pvcs, nil
}

func (r *Builder) PrePopulateActions(c planbase.Client, vmRef ref.Ref) (ready bool, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) PopulatorTransferredBytes(pvc *core.PersistentVolumeClaim) (transferredBytes int64, err error) {
	vmdkKey := pvc.Labels["vmdkKey"]
	vmId := pvc.Labels["vmID"]
	populatorCr, err := r.getVolumePopulator(vmId, vmdkKey)
	if err != nil {
		return
	}

	progressPercentage, err := strconv.ParseInt(populatorCr.Status.Progress, 10, 64)
	if err != nil {
		r.Log.Error(err, "Couldn't parse the progress percentage.", "pvcName", pvc.Name, "progressPercentage", progressPercentage)
		transferredBytes = 0
		err = nil
		return
	}

	pvcSize := pvc.Spec.Resources.Requests["storage"]
	transferredBytes = (progressPercentage * pvcSize.Value()) / 100

	return
}

func (r *Builder) getVolumePopulator(vmId, vmdkKey string) (api.VSphereXcopyVolumePopulator, error) {
	list := api.VSphereXcopyVolumePopulatorList{}
	err := r.Destination.Client.List(context.TODO(), &list, &client.ListOptions{
		Namespace: r.Plan.Spec.TargetNamespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"migration": string(r.Migration.UID),
			"vmdkKey":   vmdkKey,
			"vmID":      vmId,
		}),
	})
	if err != nil {
		return api.VSphereXcopyVolumePopulator{}, liberr.Wrap(err)
	}
	if len(list.Items) == 0 {
		return api.VSphereXcopyVolumePopulator{},
			k8serr.NewNotFound(
				api.SchemeGroupVersion.WithResource("VSphereXcopyVolumePopulator").GroupResource(), vmdkKey)
	}
	if len(list.Items) > 1 {
		return api.VSphereXcopyVolumePopulator{},
			liberr.New(
				"Multiple VSphereXcopyVolumePopulator CRs found for the same VMDK disk (with special chars replaced with _)",
				"vmdkKey",
				vmdkKey)
	}
	return list.Items[0], nil
}

func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) (err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error) {
	// copy-offload only
	taskName, _ = pvc.Annotations[planbase.AnnDiskSource]
	return
}

// getPlanVM get the plan VM for the given vsphere VM
func (r *Builder) getPlanVM(vm *model.VM) *plan.VM {
	for _, planVM := range r.Plan.Spec.VMs {
		if planVM.ID == vm.ID {
			return &planVM
		}
	}

	return nil
}

// getPlanVMStatus get the plan VM status for the given vsphere VM
func (r *Builder) getPlanVMStatus(vm *model.VM) *plan.VMStatus {
	for _, planVMStatus := range r.Plan.Status.Migration.VMs {
		if planVMStatus.ID == vm.ID {
			return planVMStatus
		}
	}

	return nil
}

func (r *Builder) executeTemplate(templateText string, templateData any) (string, error) {
	var buf bytes.Buffer

	// Parse template syntax
	tmpl, err := template.New("template").Parse(templateText)
	if err != nil {
		return "", err
	}

	// Execute template
	err = tmpl.Execute(&buf, templateData)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GetPVCNameTemplate returns the PVC name template
func (r *Builder) getPVCNameTemplate(vm *model.VM) string {
	// Get plan VM
	planVM := r.getPlanVM(vm)
	if planVM == nil {
		return ""
	}

	// if vm.PVCNameTemplate is set, use it
	if planVM.PVCNameTemplate != "" {
		return planVM.PVCNameTemplate
	}

	// if planSpec.PVCNameTemplate is set, use it
	if r.Plan.Spec.PVCNameTemplate != "" {
		return r.Plan.Spec.PVCNameTemplate
	}

	return ""
}

// getPlenVMSafeName returns a safe name for the VM
// that can be used in the template output
// The name is sanitized to be a valid k8s label
func (r *Builder) getPlenVMSafeName(vm *model.VM) string {
	// Default to vm name
	newName := vm.Name

	// Get plan VM
	planVM := r.getPlanVMStatus(vm)

	// if plan VM status has a new name, use it
	if planVM != nil && planVM.NewName != "" {
		newName = planVM.NewName
	}

	// New name is a valid subdomain name,
	// but we need to check if it is a valid k8s label

	// Check if new vm name is valid k8s label
	if len(newName) > 63 {
		// if the new name is longer then 63 characters, trancate it
		newName = newName[:63]
	}

	// Validate that template output is a valid k8s label
	errs := k8svalidation.IsDNS1123Label(newName)
	if len(errs) > 0 {
		// if the new name replace "." with "-"
		newName = strings.ReplaceAll(newName, ".", "-")
	}

	return newName
}

// getVolumeNameTemplate returns the volume name template
func (r *Builder) getVolumeNameTemplate(vm *model.VM) string {
	// Get plan VM
	planVM := r.getPlanVM(vm)
	if planVM == nil {
		return ""
	}

	// if vm.VolumeNameTemplate is set, use it
	if planVM.VolumeNameTemplate != "" {
		return planVM.VolumeNameTemplate
	}

	// if planSpec.VolumeNameTemplate is set, use it
	if r.Plan.Spec.VolumeNameTemplate != "" {
		return r.Plan.Spec.VolumeNameTemplate
	}

	return ""
}

// getNetworkNameTemplate returns the network name template
func (r *Builder) getNetworkNameTemplate(vm *model.VM) string {
	// Get plan VM
	planVM := r.getPlanVM(vm)
	if planVM == nil {
		return ""
	}

	// if vm.NetworkNameTemplate is set, use it
	if planVM.NetworkNameTemplate != "" {
		return planVM.NetworkNameTemplate
	}

	// if planSpec.NetworkNameTemplate is set, use it
	if r.Plan.Spec.NetworkNameTemplate != "" {
		return r.Plan.Spec.NetworkNameTemplate
	}

	return ""
}

// MergeSecrets merges the storage vendor secret into the migration secret
func (r *Builder) mergeSecrets(migrationSecret, migrationSecretNS, storageVendorSecret, storageVendorSecretNS string) error {
	dst := &core.Secret{}
	if err := r.Destination.Get(context.Background(), client.ObjectKey{
		Name:      migrationSecret,
		Namespace: migrationSecretNS}, dst); err != nil {
		return fmt.Errorf("failed to get migration secret: %w", err)
	}

	src := &core.Secret{}
	if err := r.Destination.Get(context.Background(), client.ObjectKey{
		Name:      storageVendorSecret,
		Namespace: storageVendorSecretNS},
		src); err != nil {
		return fmt.Errorf("failed to get storage secret: %w", err)
	}

	// Merge the data from storage secret into migration secret
	if dst.Data == nil {
		dst.Data = make(map[string][]byte)
	}
	for key, value := range src.Data {
		if _, exists := dst.Data[key]; exists {
			r.Log.Info(fmt.Sprintf("secret key %s is going to be overriden in secret %s", key, dst.Name))
		}
		dst.Data[key] = value
	}

	// copy the keys into the keys the populator needs
	for key, value := range dst.Data {
		switch key {
		case "url":
			h, err := liburl.Parse(string(value))
			if err != nil {
				// ignore and try to use as is
				dst.Data["GOVMOMI_HOSTNAME"] = value
			}
			dst.Data["GOVMOMI_HOSTNAME"] = []byte(h.Hostname())
		case "user":
			dst.Data["GOVMOMI_USERNAME"] = value
		case "password":
			dst.Data["GOVMOMI_PASSWORD"] = value
		case "insecureSkipVerify":
			dst.Data["GOVMOMI_INSECURE"] = value
		}
	}
	// Update secret1 with the merged data.
	if err := r.Destination.Update(context.Background(), dst); err != nil {
		return fmt.Errorf("failed to update secret1: %w", err)
	}

	return nil
}

func (r *Builder) ensurePopulatorServiceAccount(namespace string) error {
	r.Log.Info("Ensuring a ServiceAccount for the volume-populator SA")
	sa := core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "populator",
			Namespace: namespace,
		},
	}
	err := r.Destination.Client.Create(context.TODO(), &sa, &client.CreateOptions{})
	if err != nil && !k8serr.IsAlreadyExists(err) {
		return err
	}
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "populator-pvc-reader",
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
	err = r.Destination.Client.Create(context.TODO(), &role, &client.CreateOptions{})
	if err != nil && !k8serr.IsAlreadyExists(err) {
		return err
	}

	// Create the RoleBinding to bind the ServiceAccount to the ClusterRole
	binding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "populator-pvc-reader-binding",
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "populator",
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     "populator-pvc-reader",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	err = r.Destination.Client.Create(context.TODO(), &binding, &client.CreateOptions{})
	if err != nil && !k8serr.IsAlreadyExists(err) {
		return err
	}
	return nil
}
