package vsphere

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	liburl "net/url"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"k8s.io/klog/v2"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	basecontroller "github.com/kubev2v/forklift/pkg/controller/base"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	utils "github.com/kubev2v/forklift/pkg/controller/plan/util"
	container "github.com/kubev2v/forklift/pkg/controller/provider/container/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"github.com/kubev2v/forklift/pkg/lib/util"
	"github.com/kubev2v/forklift/pkg/settings"
	"github.com/kubev2v/forklift/pkg/templateutil"
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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// BIOS types
const (
	Efi  = "efi"
	BIOS = "bios"
)

// Bus types
const (
	Virtio = "virtio"
	E1000e = "e1000e"
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
	TemplateNAALabel      = "volume.csi.k8s.io/affinity-source-naa"
)

// Operating Systems
const (
	DefaultWindows = "win10"
	DefaultLinux   = "rhel8.1"
	Unknown        = "unknown"
	WindowsPrefix  = "win"
)

const (
	Shareable = "shareable"
)

const (
	ManagementNetwork = "Management Network"
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
	"windows7_64Guest":           "win7",
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
	} else if isWindows(vm) { // We check for multiple IPs per NIC only on Windows VMs
		macIPCount := make(map[string]int)

		for _, gn := range vm.GuestNetworks {
			//IS ipv4
			if gn.Origin == string(types.NetIpConfigInfoIpAddressOriginManual) && net.IP.To4(net.ParseIP(gn.IP)) != nil {
				macIPCount[gn.MAC]++
			}
		}

		for _, count := range macIPCount {
			if count > 1 {
				env = append(env, core.EnvVar{
					Name:  "V2V_multipleIPsPerNic",
					Value: "true",
				})
				break // stop after the first match
			}
		}
	}

	if vm.HostName != "" {
		env = append(env, core.EnvVar{
			Name:  "V2V_HOSTNAME",
			Value: vm.HostName,
		})
	}
	planVM := r.getPlanVM(vm)
	if planVM != nil && planVM.NbdeClevis {
		env = append(env, core.EnvVar{
			Name:  "V2V_NBDE_CLEVIS",
			Value: "true",
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
		core.EnvVar{
			Name:  "V2V_inspector_extra_args",
			Value: settings.Settings.Migration.VirtV2vInspectorExtraArgs,
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

// formatHostAddress wraps IPv6 addresses in brackets for URL compatibility.
func formatHostAddress(address string) string {
	ip := net.ParseIP(address)
	if ip != nil && ip.To4() == nil {
		// IPv6 address - wrap in brackets
		return "[" + address + "]"
	}
	// IPv4 address or hostname - return as-is
	return address
}

func (r *Builder) getSourceDetails(vm *model.VM, sourceSecret *core.Secret) (libvirtURL liburl.URL, fingerprint string, err error) {
	host, err := r.host(vm.Host)
	if err != nil {
		return
	}

	var sslVerify string
	if basecontroller.GetInsecureSkipVerifyFlag(sourceSecret) {
		sslVerify = "no_verify=1"
	} else {
		// This path is created by linkCertificates in the v2v container containes either the provider cert or pod certs.
		sslVerify = "cacert=/opt/ca-bundle.crt"
	}

	if hostDef, found := r.hosts[host.ID]; found {
		// Connect through ESXi
		var hostSecret *core.Secret
		if hostSecret, err = r.hostSecret(hostDef); err != nil {
			return
		}
		libvirtURL = liburl.URL{
			Scheme:   "esx",
			Host:     formatHostAddress(hostDef.Spec.IpAddress),
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
		// For ESXi SDK endpoint, use the provider URL directly instead of
		// extracting management IP from inventory
		var url *liburl.URL
		if url, err = liburl.Parse(r.Source.Provider.Spec.URL); err != nil {
			err = liberr.Wrap(err)
			return
		}
		libvirtURL = liburl.URL{
			Scheme:   "esx",
			Host:     formatHostAddress(url.Hostname()),
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
			Host:   formatHostAddress(hostDef.Spec.IpAddress),
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

	// For storage offload warm migrations, match this DataVolume to the
	// existing PVC via the backing file name.
	var pvcMap map[string]core.PersistentVolumeClaim
	if r.Plan.IsWarm() && r.SupportsVolumePopulators() {
		pvcMap = make(map[string]core.PersistentVolumeClaim)
		pvcs := &core.PersistentVolumeClaimList{}
		pvcLabels := map[string]string{
			"vmID":      vmRef.ID,
			"migration": string(r.Migration.UID),
		}

		err = r.Context.Destination.Client.List(
			context.TODO(),
			pvcs,
			&client.ListOptions{
				Namespace:     r.Plan.Spec.TargetNamespace,
				LabelSelector: labels.SelectorFromSet(pvcLabels),
			},
		)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}

		for _, pvc := range pvcs.Items {
			if copyOffload, present := pvc.Annotations["copy-offload"]; present && copyOffload != "" {
				pvcMap[baseVolume(copyOffload, r.Plan.IsWarm())] = pvc
			}
		}
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
			vddkImage := settings.GetVDDKImage(r.Source.Provider.Spec.Settings)

			// Let CDI do the copying
			dvSource = cdi.DataVolumeSource{
				VDDK: &cdi.DataVolumeSourceVDDK{
					BackingFile:  baseVolume(disk.File, r.Plan.IsWarm()),
					UUID:         vm.UUID,
					URL:          url,
					SecretRef:    secret.Name,
					Thumbprint:   thumbprint,
					InitImageURL: vddkImage,
				},
			}
		}
		alignedCapacity := utils.RoundUp(disk.Capacity, utils.DefaultAlignBlockSize)
		dvSpec := cdi.DataVolumeSpec{
			Source: &dvSource,
			Storage: &cdi.StorageSpec{
				Resources: core.VolumeResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: *resource.NewQuantity(alignedCapacity, resource.BinarySI),
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
		dv.ObjectMeta.Annotations[planbase.AnnDiskSource] = baseVolume(disk.File, r.Plan.IsWarm())
		if disk.Shared {
			dv.ObjectMeta.Labels[Shareable] = "true"
		}

		// Preserve the disk index as an annotation on the created DataVolume
		// Note: this annotation will be used to match the PVC to the VM disks by
		//       matching the disk and PVC index.
		dv.ObjectMeta.Annotations[planbase.AnnDiskIndex] = fmt.Sprintf("%d", diskIndex)

		if pvcMap != nil && dvSource.VDDK != nil {
			// In a warm migration with storage offload, the PVC has already been created with
			// the name template. Copy the result to the DataVolume so it can adopt the PVC.
			if pvc, present := pvcMap[dvSource.VDDK.BackingFile]; present {
				dv.ObjectMeta.Name = pvc.Name
			}
		} else {
			// Set PVC name/generateName using template if configured
			if err := r.setPVCNameFromTemplate(&dv.ObjectMeta, vm, diskIndex, disk); err != nil {
				r.Log.Info("Failed to set PVC name from template", "error", err)
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
	if r.Plan.IsWarm() && !vm.ChangeTrackingEnabled {
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

func isIPv4(address string) bool {
	ip := net.ParseIP(address)
	return ip != nil && ip.To4() != nil
}

func (r *Builder) findInterfaceIps(vm *model.VM, nic vsphere.NIC) []string {
	var interfaceIps []string
	for _, net := range vm.GuestNetworks {
		if net.DeviceConfigId == nic.DeviceKey {
			if isIPv4(net.IP) {
				interfaceIps = append(interfaceIps, net.IP)
			}
		}
	}
	return interfaceIps
}

func (r *Builder) mapNetworks(vm *model.VM, object *cnv.VirtualMachineSpec) (err error) {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface
	var staticIpInterfaces = make(map[string][]string)

	numNetworks := 0
	hasUDN := r.Plan.DestinationHasUdnNetwork(r.Destination)
	netMapIn := r.Context.Map.Network.Spec.Map

	for _, nic := range vm.NICs {
		mapped := r.findNetworkMapping(nic, netMapIn)

		// Skip if no valid mapping found or the destination type is Ignored
		if mapped == nil || mapped.Destination.Type == Ignored {
			continue
		}

		networkName := fmt.Sprintf("net-%v", numNetworks)

		// Generate network name using template if configured
		if templatedName, templateErr := r.setNetworkNameFromTemplate(vm, mapped, numNetworks); templateErr != nil {
			r.Log.Info("Failed to generate network name using template, using default", "error", templateErr)
		} else if templatedName != "" {
			networkName = templatedName
		}
		numNetworks++
		kNetwork := cnv.Network{Name: networkName}
		interfaceModel := Virtio
		if useCompatibilityModeBus(r.Plan) {
			interfaceModel = E1000e
		}
		kInterface := cnv.Interface{
			Name:  networkName,
			Model: interfaceModel,
		}

		if !hasUDN || settings.Settings.UdnSupportsMac {
			kInterface.MacAddress = nic.MAC
		}
		switch mapped.Destination.Type {
		case Pod:
			kNetwork.Pod = &cnv.PodNetwork{}
			if hasUDN {
				kInterface.Binding = &cnv.PluginBinding{
					Name: planbase.UdnL2bridge,
				}
				if r.Plan.Spec.PreserveStaticIPs {
					ips := r.findInterfaceIps(vm, nic)
					if len(ips) > 0 {
						staticIpInterfaces[networkName] = ips
					}
				}
			} else {
				kInterface.Masquerade = &cnv.InterfaceMasquerade{}
			}
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

	if settings.Settings.StaticUdnIpAddresses && hasUDN && r.Plan.Spec.PreserveStaticIPs && len(staticIpInterfaces) > 0 {
		var staticIpInterfacesAnnotation []byte
		staticIpInterfacesAnnotation, err = json.Marshal(staticIpInterfaces)
		if err != nil {
			return err
		}
		if object.Template.ObjectMeta.Annotations == nil {
			object.Template.ObjectMeta.Annotations = make(map[string]string)
		}
		object.Template.ObjectMeta.Annotations[planbase.AnnStaticUdnIp] = string(staticIpInterfacesAnnotation)
	}
	return
}

func (r *Builder) findNetworkMapping(nic vsphere.NIC, netMap []api.NetworkPair) *api.NetworkPair {
	for i := range netMap {
		candidate := &netMap[i]
		network := &model.Network{}
		if err := r.Source.Inventory.Find(network, candidate.Source); err != nil {
			continue
		}

		if (network.Variant == vsphere.NetDvPortGroup || network.Variant == vsphere.OpaqueNetwork) &&
			nic.Network.ID == network.Key || nic.Network.ID == network.ID {
			return candidate
		}
	}
	return nil
}

func (r *Builder) mapInput(object *cnv.VirtualMachineSpec) {
	bus := cnv.InputBusVirtio
	if useCompatibilityModeBus(r.Plan) {
		bus = cnv.InputBusUSB
	}
	tablet := cnv.Input{
		Type: Tablet,
		Name: Tablet,
		Bus:  bus,
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
	if vm.NestedHVEnabled {
		//FIXME: Replace in future with single feature flag for nested virt https://issues.redhat.com/browse/CNV-60150
		var features []cnv.CPUFeature
		features = append(features, cnv.CPUFeature{
			Name:   "vmx",
			Policy: "optional",
		})
		features = append(features, cnv.CPUFeature{
			Name:   "svm",
			Policy: "optional",
		})
		object.Template.Spec.Domain.CPU.Features = features
	}
}

func (r *Builder) getSystemSerial(vm *model.VM) string {
	// On deployments where VMware serial number formtting is enabled,
	if settings.Settings.VmwareSystemSerialNumber {
		// we use the UUID to generate a VMware serial number.
		return UUIDToVMwareSerial(vm.UUID)
	}

	// Default to using .config.uuid as the system serial number
	return vm.UUID
}

func (r *Builder) mapFirmware(vm *model.VM, object *cnv.VirtualMachineSpec) {
	firmware := &cnv.Firmware{
		Serial: r.getSystemSerial(vm),
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
	var buses = []string{container.SCSI, container.SATA, container.IDE, container.NVME}
	return r.sortedDisksByBusses(disks, buses)
}

func (r *Builder) sortedDisksAsVmware(disks []vsphere.Disk) []vsphere.Disk {
	var buses = []string{container.SATA, container.IDE, container.SCSI, container.NVME}
	return r.sortedDisksByBusses(disks, buses)
}

func (r *Builder) mapDisks(vm *model.VM, vmRef ref.Ref, persistentVolumeClaims []*core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec, sortByLibvirt bool) error {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk
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
			pvcMap[trimBackingFileName(pvc.Annotations[planbase.AnnImportBackingFile])] = pvc
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
			r.Log.Info("No matching PVC found for disk", "diskFile", disk.File, "trimmedFile", trimBackingFileName(disk.File))
			return fmt.Errorf("failed to find persistent volume for disk %s", disk.File)
		}
		volumeName := fmt.Sprintf("vol-%v", i)

		// Generate volume name using template if configured
		if templatedName, templateErr := r.setVolumeNameFromTemplate(vm, pvc.Name, i); templateErr != nil {
			// Failed to generate volume name using template
			r.Log.Info("Failed to generate volume name using template, using default name", "error", templateErr)
		} else if templatedName != "" {
			volumeName = templatedName
		}

		// Check if the generated name is longer than 63 characters
		if len(volumeName) > 63 {
			// If the generated name is longer than 63 characters, trancate it
			newVolumeName := volumeName[:63]
			r.Log.Info("Generated volume name is longer than 63 characters, truncating", "volumeName", volumeName, "newVolumeName", newVolumeName)

			volumeName = newVolumeName
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
		bus := cnv.DiskBusVirtio
		if useCompatibilityModeBus(r.Plan) {
			bus = cnv.DiskBusSATA
		}
		kubevirtDisk := cnv.Disk{
			Name: volumeName,
			DiskDevice: cnv.DiskDevice{
				Disk: &cnv.DiskTarget{
					Bus: bus,
				},
			},
		}
		if disk.Shared {
			kubevirtDisk.Shareable = ptr.To(true)
			kubevirtDisk.Cache = cnv.CacheNone
		}
		// Disk serial numbers are only presented to the source guest if
		// the disk is connected with SCSI and disk.EnableUUID is set to
		// TRUE, so only save the serial number for those disks. If the
		// destination VM is configured with VirtIO or SATA, the resulting
		// serial number will be truncated to 20 characters.
		if vm.DiskEnableUuid && cnv.DiskBus(disk.Bus) == cnv.DiskBusSCSI {
			kubevirtDisk.Serial = disk.Serial
		}
		kVolumes = append(kVolumes, volume)
		kDisks = append(kDisks, kubevirtDisk)
	}
	if len(kDisks) == 0 {
		r.Log.Info("No disks were successfully mapped", "vm", vm.Name, "vmID", vmRef.ID)
		for _, d := range disks {
			r.Log.Info("Unmapped disk", "diskFile", d.File)
		}
		for key, pvc := range pvcMap {
			r.Log.Info("Available PVC mapping", "diskKey", key, "pvcName", pvc.Name)
		}
		return fmt.Errorf("no disks were successfully mapped for VM %s", vm.Name)
	} else if bootDisk < len(kDisks) {
		// For multiboot VMs, if the selected boot device is the current disk,
		// set it as the first in the boot order.
		kDisks[bootDisk].BootOrder = ptr.To(uint(1))
	} else {
		r.Log.Info("Boot disk index out of range", "bootDisk", bootDisk, "diskCount", len(kDisks), "vm", vm.Name)
	}

	object.Template.Spec.Volumes = kVolumes
	object.Template.Spec.Domain.Devices.Disks = kDisks
	return nil
}

func (r *Builder) mapTpm(vm *model.VM, object *cnv.VirtualMachineSpec) {
	if vm.TpmEnabled {
		// If the VM has vTPM enabled, we need to set Persistent in the VM spec.
		object.Template.Spec.Domain.Devices.TPM = &cnv.TPMDevice{Persistent: ptr.To(true)}
	} else {
		// Force disable the vTPM
		// MTV-2014 - win 2022 fails to boot with vTPM enabled
		object.Template.Spec.Domain.Devices.TPM = &cnv.TPMDevice{Enabled: ptr.To(false)}
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
		mB := utils.RoundUp(disk.Capacity, 0x100000) / 0x100000
		list = append(
			list,
			&plan.Task{
				Name: baseVolume(disk.File, r.Plan.IsWarm()),
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

func (r *Builder) ConfigMaps(vmRef ref.Ref) (list []core.ConfigMap, err error) {
	return nil, nil
}

func (r *Builder) Secrets(vmRef ref.Ref) (list []core.Secret, err error) {
	return nil, nil
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
	return baseVolume(dv.ObjectMeta.Annotations[planbase.AnnDiskSource], r.Plan.IsWarm())
}

// Return a stable identifier for a PersistentDataVolume.
func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	if source, ok := pvc.Annotations[planbase.AnnDiskSource]; ok {
		return baseVolume(source, r.Plan.IsWarm())
	}
	return baseVolume(pvc.Annotations[planbase.AnnImportBackingFile], r.Plan.IsWarm())
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
	if !settings.Settings.Features.CopyOffload {
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

	// Get a list of existing PVCs to avoid creating duplicates
	pvcLabels := map[string]string{
		"migration": string(r.Migration.UID),
		"vmID":      vmRef.ID,
	}
	pvcList := &core.PersistentVolumeClaimList{}
	err = r.Destination.Client.List(
		context.TODO(),
		pvcList,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(pvcLabels),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if !r.Context.Plan.Spec.MigrateSharedDisks {
		vm.RemoveSharedDisks()
	}
	// Get sorted disks to maintain consistent indexing with other parts of the system
	sortedDisks := r.sortedDisksAsVmware(vm.Disks)

	dsMapIn := r.Context.Map.Storage.Spec.Map
	dsNaaMap := make(map[string]string)
	for i := range dsMapIn {
		mapped := &dsMapIn[i]
		sourceRef := mapped.Source
		ds := &model.Datastore{}
		fErr := r.Source.Inventory.Find(ds, sourceRef)
		if fErr != nil {
			err = fErr
			return
		}

		pvblock := core.PersistentVolumeBlock
		for diskIndex, disk := range sortedDisks {
			if disk.Datastore.ID == ds.ID {
				naa, ok := dsNaaMap[ds.ID]
				if !ok {
					vsphereClient := &Client{Context: r.Context}
					err = vsphereClient.connect()
					if err != nil {
						r.Log.Error(err, "failed to connect to vSphere client, continue without storage affinity label")
					}
					naa, err = vsphereClient.getNAAFromDatastore(context.TODO(), ref.Ref{ID: ds.ID, Name: ds.Name})
					defer vsphereClient.Close()
					if err != nil {
						r.Log.Error(err, "failed to get NAA from datastore %s, continue without storage affinity label", ds.Name)
					}
					dsNaaMap[ds.ID] = naa
				}
				storageClass := mapped.Destination.StorageClass
				r.Log.Info(fmt.Sprintf("getting storage mapping by storage class %q and datastore %v datastore name %s datastore", storageClass, disk.Datastore, disk.Datastore))
				vsphereInstance := r.Context.Plan.Provider.Source.GetName()
				storageVendorProduct := mapped.OffloadPlugin.VSphereXcopyPluginConfig.StorageVendorProduct
				storageVendorSecretRef := mapped.OffloadPlugin.VSphereXcopyPluginConfig.SecretRef

				r.Log.Info(fmt.Sprintf("vsphere provider %v storage vendor product %v storage secret name %v ", vsphereInstance, storageVendorProduct, storageVendorSecretRef))

				if vsphereInstance == "" || storageVendorProduct == "" || storageVendorSecretRef == "" {
					return nil, fmt.Errorf(
						"the offload plugin configuration has missing details, cannot continue with PVC and populator resource creation")
				}

				namespace := r.Plan.Spec.TargetNamespace
				// we need uniqueness and a value which is less than 64 chars, hence using vmRef.id + disk.key
				labels := r.Labeler.VMLabelsWithExtra(vmRef, map[string]string{
					"vmdkKey": fmt.Sprint(disk.Key),
				})
				// Only add the NAA label if it's a valid Kubernetes label value
				if errs := k8svalidation.IsValidLabelValue(naa); len(errs) == 0 {
					labels[TemplateNAALabel] = naa
				}

				r.Log.Info("target namespace for migration", "namespace", namespace)
				pvc := core.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:   namespace,
						Labels:      labels,
						Annotations: annotations,
					},
					Spec: core.PersistentVolumeClaimSpec{
						StorageClassName: &storageClass,
						VolumeMode:       &pvblock,
						Resources: core.VolumeResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: *resource.NewQuantity(disk.Capacity, resource.BinarySI),
							},
						},
						DataSourceRef: &core.TypedObjectReference{
							APIGroup: &api.SchemeGroupVersion.Group,
							Kind:     api.VSphereXcopyVolumePopulatorKind,
						},
					},
				}
				if mapped.Destination.AccessMode != "" {
					pvc.Spec.AccessModes = []core.PersistentVolumeAccessMode{mapped.Destination.AccessMode}
				} else {
					pvc.Spec.AccessModes = []core.PersistentVolumeAccessMode{core.ReadWriteMany}
				}

				if annotations == nil {
					pvc.Annotations = make(map[string]string)
				} else {
					pvc.Annotations = annotations
				}
				pvc.Annotations[planbase.AnnDiskSource] = baseVolume(disk.File, r.Plan.IsWarm())
				pvc.Annotations["copy-offload"] = baseVolume(disk.File, r.Plan.IsWarm())

				// Apply PVC template naming if configured, replacing the commonName
				if err := r.setColdMigrationDefaultPVCName(&pvc.ObjectMeta, vm, diskIndex, disk); err != nil {
					r.Log.Info("Failed to set PVC name from template for populator volume, using default name", "error", err)
				}
				if pvc.ObjectMeta.GenerateName != "" {
					suffix := r.generatePopulatorSuffix(string(r.Migration.UID), vmRef.ID, disk.Key, disk.File, diskIndex)
					pvc.ObjectMeta.Name = strings.TrimSuffix(pvc.ObjectMeta.GenerateName, "-") + "-" + suffix
					pvc.ObjectMeta.GenerateName = ""
				}

				// populator name is the name of the populator, and we can't use generateName for the populator
				populatorName := pvc.ObjectMeta.Name
				r.Log.V(2).Info("Initial populator name from new PVC", "populatorName", populatorName, "pvcName", pvc.ObjectMeta.Name)

				// For warm migration, add annotations to jump-start the DataVolume
				v := r.getPlanVMStatus(vm)
				if v != nil && v.Warm != nil {
					pvc.Annotations[planbase.AnnEndpoint] = r.Source.Provider.Spec.URL
					pvc.Annotations[planbase.AnnImportBackingFile] = baseVolume(disk.File, r.Plan.IsWarm())
					pvc.Annotations[planbase.AnnUUID] = vm.UUID
					pvc.Annotations[planbase.AnnThumbprint] = r.Source.Provider.Status.Fingerprint
					pvc.Annotations[planbase.AnnVddkInitImageURL] = settings.GetVDDKImage(r.Source.Provider.Spec.Settings)
					pvc.Annotations[planbase.AnnPodPhase] = "Succeeded"
					pvc.Annotations[planbase.AnnSource] = "vddk"

					n := len(v.Warm.Precopies)
					if n > 0 { // Should be 1 at this point
						snapshot := v.Warm.Precopies[n-1].Snapshot
						pvc.Annotations[planbase.AnnFinalCheckpoint] = "false"
						pvc.Annotations[planbase.AnnCurrentCheckpoint] = snapshot
						pvc.Annotations[planbase.AnnPreviousCheckpoint] = ""

						copied := fmt.Sprintf("%s.%s", planbase.AnnCheckpointsCopied, snapshot)
						pvc.Annotations[copied] = "xcopy-initial-offload"                // Any value should work here
						pvc.Annotations[planbase.AnnImportPod] = "xcopy-initial-offload" // Should match above
					}
				}

				// Update DataSourceRef to point to the volume populator
				pvc.Spec.DataSourceRef.Name = populatorName
				diskSecretName := fmt.Sprintf("%s-%d", secretName, diskIndex)
				pvc.Annotations[planbase.AnnSecret] = diskSecretName
				pvcs = append(pvcs, &pvc)
				vp := api.VSphereXcopyVolumePopulator{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "PersistentVolumeClaim",
								Name:       pvc.Name,
								UID:        pvc.UID,
							},
						},
						Name:      populatorName,
						Namespace: namespace,
						Labels:    labels,
					},
					Spec: api.VSphereXcopyVolumePopulatorSpec{
						VmId:                 vmRef.ID,
						VmdkPath:             baseVolume(disk.File, r.Plan.IsWarm()),
						SecretName:           diskSecretName,
						StorageVendorProduct: string(storageVendorProduct),
					},
				}
				createdPVC := &core.PersistentVolumeClaim{}
				// Check if a PVC was created for the current disk
				if !r.isPVCExistsInList(&pvc, pvcList) {
					r.Log.Info("Creating pvc", "pvc", pvc)
					err = r.Destination.Client.Create(context.TODO(), &pvc, &client.CreateOptions{})
					if err != nil {
						if k8serr.IsAlreadyExists(err) {
							r.Log.Info("PVC already exists in Kubernetes, skipping", "pvcName", pvc.ObjectMeta.Name)
							continue
						}
						return nil, err
					}
				}
				// Fetch the PVC back to get the UID assigned by Kubernetes
				err = r.Destination.Client.Get(context.TODO(), client.ObjectKey{
					Namespace: pvc.Namespace,
					Name:      pvc.Name,
				}, createdPVC)
				if err != nil {
					return nil, err
				}

				vp.OwnerReferences[0].UID = createdPVC.UID
				err = r.mergeSecrets(secretName, namespace, storageVendorSecretRef, r.Source.Provider.Namespace, diskSecretName, createdPVC)
				if err != nil {
					return nil, fmt.Errorf("failed to merge secrets for populators %w", err)
				}

				r.Log.Info("Ensuring a populator service account")
				err = r.ensurePopulatorServiceAccount(namespace)
				if err != nil {
					return nil, err
				}
				err = r.ensureXCopyVolumePopulator(&vp)
				if err != nil {
					return nil, err
				}
			}
		}
		if len(pvcs) > 0 {
			secret := &core.Secret{}
			err = r.Destination.Client.Get(context.TODO(), client.ObjectKey{
				Namespace: r.Plan.Spec.TargetNamespace,
				Name:      secretName,
			}, secret)
			if err != nil {
				return nil, err
			}
			err := controllerutil.SetOwnerReference(pvcs[0], secret, r.Scheme())
			if err != nil {
				r.Log.Error(err, "Failed to set pvc as owner reference for migration secret '%s'", secret.Name)
			} else {
				err = r.Destination.Client.Update(context.TODO(), secret)
				if err != nil {
					r.Log.Error(err, "Failed to update migration secret '%s' with owner reference", secret.Name)
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
			liberr.New(
				"No VSphereXcopyVolumePopulator CR found - populator may not have been created or was deleted",
				"namespace", r.Plan.Spec.TargetNamespace,
				"migration", string(r.Migration.UID),
				"vmID", vmId,
				"vmdkKey", vmdkKey)
	}
	if len(list.Items) > 1 {
		names := make([]string, len(list.Items))
		for i, item := range list.Items {
			names[i] = item.Name
		}
		return api.VSphereXcopyVolumePopulator{},
			liberr.New(
				"Multiple VSphereXcopyVolumePopulator CRs found for the same VMDK disk",
				"namespace", r.Plan.Spec.TargetNamespace,
				"migration", string(r.Migration.UID),
				"vmID", vmId,
				"vmdkKey", vmdkKey,
				"populators", strings.Join(names, ", "))
	}
	return list.Items[0], nil
}

func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) (err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error) {
	// copy-offload only
	taskName = baseVolume(pvc.Annotations[planbase.AnnDiskSource], r.Plan.IsWarm())
	return
}

// getPlanVM get the plan VM for the given vsphere VM
func (r *Builder) getPlanVM(vm *model.VM) *plan.VM {
	for i := range r.Plan.Spec.VMs {
		planVM := &r.Plan.Spec.VMs[i]
		if planVM.ID != "" && planVM.ID == vm.ID {
			return planVM
		}
	}
	// Fallback: match by Name when the spec VM has no ID
	for i := range r.Plan.Spec.VMs {
		planVM := &r.Plan.Spec.VMs[i]
		if planVM.ID == "" && planVM.Name != "" && planVM.Name == vm.Name {
			return planVM
		}
	}

	return nil
}

// getPlanVMStatus get the plan VM status for the given vsphere VM
func (r *Builder) getPlanVMStatus(vm *model.VM) *plan.VMStatus {
	if r.Plan == nil || r.Plan.Status.Migration.VMs == nil {
		return nil
	}

	for _, planVMStatus := range r.Plan.Status.Migration.VMs {
		if planVMStatus.ID != "" && planVMStatus.ID == vm.ID {
			return planVMStatus
		}
	}
	// Fallback: match by Name when the status VM has no ID
	for _, planVMStatus := range r.Plan.Status.Migration.VMs {
		if planVMStatus.ID == "" && planVMStatus.Name != "" && planVMStatus.Name == vm.Name {
			return planVMStatus
		}
	}

	return nil
}

func (r *Builder) executeTemplate(templateText string, templateData any) (string, error) {
	return templateutil.ExecuteTemplate(templateText, templateData)
}

// TemplateConfig defines configuration for template-based naming
type TemplateConfig struct {
	Template        string
	UseGenerateName bool
	TemplateType    string
}

// setObjectNameFromTemplate sets the Name or GenerateName field on an object's metadata
// based on a template and template data. This refactors the common naming logic.
func (r *Builder) setObjectNameFromTemplate(objectMeta *metav1.ObjectMeta, templateConfig TemplateConfig, templateData any) error {
	if templateConfig.Template == "" {
		return nil
	}

	generatedName, err := r.executeTemplate(templateConfig.Template, templateData)
	if err != nil {
		r.Log.Info("Failed to generate name using template",
			"template", templateConfig.Template,
			"templateType", templateConfig.TemplateType,
			"error", err)
		return err
	}

	if generatedName == "" {
		return nil
	}

	// Validate that template output is a valid k8s label
	errs := k8svalidation.IsDNS1123Label(generatedName)
	if len(errs) > 0 {
		err = errors.New("generated name is not valid")
		r.Log.Info("Generated name is not a valid k8s label",
			"template", templateConfig.Template,
			"templateType", templateConfig.TemplateType,
			"generatedName", generatedName,
			"errors", errs,
			"error", err)
		return err
	}

	if templateConfig.UseGenerateName {
		// Ensure generatedName ends with "-"
		if !strings.HasSuffix(generatedName, "-") {
			generatedName = generatedName + "-"
		}
		objectMeta.GenerateName = generatedName
	} else {
		// Ensure generatedName does not end with "-"
		if strings.HasSuffix(generatedName, "-") {
			generatedName = strings.Trim(generatedName, "-")
		}
		objectMeta.Name = generatedName
	}

	return nil
}

func (r *Builder) setColdMigrationDefaultPVCName(objectMeta *metav1.ObjectMeta, vm *model.VM, diskIndex int, disk vsphere.Disk) error {
	pvcNameTemplate := r.getPVCNameTemplate(vm)
	if pvcNameTemplate == "" {
		pvcNameTemplate = "{{trunc 4 .PlanName}}-{{trunc 4 .VmName}}-disk-{{.DiskIndex}}"
	}

	planVM := r.getPlanVM(vm)
	rootDiskIndex := 0
	if planVM != nil {
		rootDiskIndex = utils.GetBootDiskNumber(planVM.RootDisk)
	}

	templateData := api.VSpherePVCNameTemplateData{
		VmName:         r.getPlanVMSafeName(vm),
		PlanName:       r.Plan.Name,
		DiskIndex:      diskIndex,
		RootDiskIndex:  rootDiskIndex,
		Shared:         disk.Shared,
		FileName:       extractDiskFileName(baseVolume(disk.File, false)),
		WinDriveLetter: disk.WinDriveLetter,
	}

	templateConfig := TemplateConfig{
		Template:        pvcNameTemplate,
		UseGenerateName: r.Plan.Spec.PVCNameTemplateUseGenerateName,
		TemplateType:    "PVC",
	}

	return r.setObjectNameFromTemplate(objectMeta, templateConfig, &templateData)

}

// setPVCNameFromTemplate sets PVC name/generateName using the PVC template
func (r *Builder) setPVCNameFromTemplate(objectMeta *metav1.ObjectMeta, vm *model.VM, diskIndex int, disk vsphere.Disk) error {
	pvcNameTemplate := r.getPVCNameTemplate(vm)
	if pvcNameTemplate == "" {
		return nil
	}

	// Get the VM root disk index
	planVM := r.getPlanVM(vm)
	rootDiskIndex := 0
	if planVM != nil {
		rootDiskIndex = utils.GetBootDiskNumber(planVM.RootDisk)
	}

	isWarm := r.Plan.IsWarm()

	// Get plan VM status
	planVMStatus := r.getPlanVMStatus(vm)

	// Resolve names with safe fallbacks
	vmName := vm.Name
	targetVmName := ""
	if planVMStatus != nil && planVMStatus.NewName != "" {
		targetVmName = planVMStatus.NewName
	} else {
		// Best-effort DNS1123-safe fallback
		targetVmName = utils.ChangeVmName(vmName)
	}

	// Create template data
	templateData := api.VSpherePVCNameTemplateData{
		VmName:         vmName,
		TargetVmName:   targetVmName,
		PlanName:       r.Plan.Name,
		DiskIndex:      diskIndex,
		RootDiskIndex:  rootDiskIndex,
		Shared:         disk.Shared,
		FileName:       extractDiskFileName(baseVolume(disk.File, isWarm)),
		WinDriveLetter: disk.WinDriveLetter,
	}

	templateConfig := TemplateConfig{
		Template:        pvcNameTemplate,
		UseGenerateName: r.Plan.Spec.PVCNameTemplateUseGenerateName,
		TemplateType:    "PVC",
	}

	return r.setObjectNameFromTemplate(objectMeta, templateConfig, &templateData)
}

// setVolumeNameFromTemplate generates volume name using volume template
func (r *Builder) setVolumeNameFromTemplate(vm *model.VM, pvcName string, volumeIndex int) (string, error) {
	volumeNameTemplate := r.getVolumeNameTemplate(vm)
	if volumeNameTemplate == "" {
		return "", nil
	}

	// Create template data
	templateData := api.VolumeNameTemplateData{
		PVCName:     pvcName,
		VolumeIndex: volumeIndex,
	}

	return r.executeTemplate(volumeNameTemplate, &templateData)
}

// setNetworkNameFromTemplate generates network name using network template
func (r *Builder) setNetworkNameFromTemplate(vm *model.VM, mapped *api.NetworkPair, networkIndex int) (string, error) {
	networkNameTemplate := r.getNetworkNameTemplate(vm)
	if networkNameTemplate == "" {
		return "", nil
	}

	// Create template data
	templateData := api.NetworkNameTemplateData{
		NetworkName:      mapped.Destination.Name,
		NetworkNamespace: mapped.Destination.Namespace,
		NetworkType:      mapped.Destination.Type,
		NetworkIndex:     networkIndex,
	}

	return r.executeTemplate(networkNameTemplate, &templateData)
}

// GetPVCNameTemplate returns the PVC name template
func (r *Builder) getPVCNameTemplate(vm *model.VM) string {
	// Check VM-level template first
	planVM := r.getPlanVM(vm)
	if planVM != nil && planVM.PVCNameTemplate != "" {
		return planVM.PVCNameTemplate
	}

	// Check Plan-level template
	if r.Plan.Spec.PVCNameTemplate != "" {
		return r.Plan.Spec.PVCNameTemplate
	}

	return ""
}

// getPlanVMSafeName returns a safe name for the VM
// that can be used in the template output
// The name is sanitized to be a valid k8s label
func (r *Builder) getPlanVMSafeName(vm *model.VM) string {
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
	// Check VM-level template first
	planVM := r.getPlanVM(vm)
	if planVM != nil && planVM.VolumeNameTemplate != "" {
		return planVM.VolumeNameTemplate
	}

	// Check Plan-level template
	if r.Plan.Spec.VolumeNameTemplate != "" {
		return r.Plan.Spec.VolumeNameTemplate
	}

	return ""
}

// getNetworkNameTemplate returns the network name template
func (r *Builder) getNetworkNameTemplate(vm *model.VM) string {
	// Check VM-level template first
	planVM := r.getPlanVM(vm)
	if planVM != nil && planVM.NetworkNameTemplate != "" {
		return planVM.NetworkNameTemplate
	}

	// Check Plan-level template
	if r.Plan.Spec.NetworkNameTemplate != "" {
		return r.Plan.Spec.NetworkNameTemplate
	}

	return ""
}

// MergeSecrets merges the storage vendor secret into the migration secret
func (r *Builder) mergeSecrets(migrationSecret, migrationSecretNS, storageVendorSecret, storageVendorSecretNS, diskSecretName string, pvc *core.PersistentVolumeClaim) error {
	baseMigrationSecret := &core.Secret{}
	if err := r.Destination.Get(context.Background(), client.ObjectKey{
		Name:      migrationSecret,
		Namespace: migrationSecretNS}, baseMigrationSecret); err != nil {
		return fmt.Errorf("failed to get base migration secret: %w", err)
	}

	dst := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      diskSecretName,
			Namespace: migrationSecretNS,
		},
		Data: make(map[string][]byte),
	}

	for key, value := range baseMigrationSecret.Data {
		dst.Data[key] = value
	}

	src := &core.Secret{}
	if err := r.Destination.Get(context.Background(), client.ObjectKey{
		Name:      storageVendorSecret,
		Namespace: storageVendorSecretNS},
		src); err != nil {
		return fmt.Errorf("failed to get storage secret: %w", err)
	}

	if dst.Data == nil {
		dst.Data = make(map[string][]byte)
	}
	for key, value := range src.Data {
		if _, exists := dst.Data[key]; exists {
			r.Log.Info(fmt.Sprintf("secret key %s is going to be overriden in secret %s", key, dst.Name))
		}
		dst.Data[key] = value
	}

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
			dst.Data["accessKeyId"] = value
		case "password":
			dst.Data["GOVMOMI_PASSWORD"] = value
			dst.Data["secretKey"] = value
		case "insecureSkipVerify":
			dst.Data["GOVMOMI_INSECURE"] = value
		}
	}

	// Add provider settings to the secret
	if esxiCloneMethod, ok := r.Source.Provider.Spec.Settings[api.ESXiCloneMethod]; ok {
		dst.Data["ESXI_CLONE_METHOD"] = []byte(esxiCloneMethod)
	}

	// Add controller-level settings for host leases (copy offload)
	if settings.Settings.Migration.HostLeaseNamespace != "" {
		dst.Data["HOST_LEASE_NAMESPACE"] = []byte(settings.Settings.Migration.HostLeaseNamespace)
	}
	if settings.Settings.Migration.HostLeaseDurationSeconds != "" {
		dst.Data["HOST_LEASE_DURATION_SECONDS"] = []byte(settings.Settings.Migration.HostLeaseDurationSeconds)
	}

	// Add SSH keys for vSphere providers
	if r.Source.Provider.Type() == api.VSphere {
		err := r.addSSHKeysToSecret(dst)
		if err != nil {
			r.Log.Error(err, "Failed to add SSH keys to secret", "secret", dst.Name)
			// Continue without SSH keys - this will fall back to VIB method or fail gracefully
		}
	}

	if err := controllerutil.SetOwnerReference(pvc, dst, r.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	if err := r.Destination.Create(context.Background(), dst); err != nil {
		return fmt.Errorf("failed to create disk secret: %w", err)
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
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
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

	// Create Role in openshift-mtv namespace for cross-namespace lease access
	mtvRole := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "populator-lease-reader",
			Namespace: "openshift-mtv",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "list", "watch", "create", "update"},
			},
		},
	}
	err = r.Destination.Client.Create(context.TODO(), &mtvRole, &client.CreateOptions{})
	if err != nil && !k8serr.IsAlreadyExists(err) {
		return err
	}

	// Create RoleBinding in openshift-mtv namespace
	mtvBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "populator-lease-reader-binding",
			Namespace: "openshift-mtv",
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
			Name:     "populator-lease-reader",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	updatedMtvBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: mtvBinding.Name, Namespace: mtvBinding.Namespace}}
	_, err = controllerutil.CreateOrPatch(
		context.TODO(),
		r.Destination.Client,
		updatedMtvBinding, func() error {
			if updatedMtvBinding.CreationTimestamp.IsZero() {
				updatedMtvBinding.Subjects = mtvBinding.Subjects
				updatedMtvBinding.RoleRef = mtvBinding.RoleRef
			} else {
				if !slices.Contains(updatedMtvBinding.Subjects, mtvBinding.Subjects[0]) {
					updatedMtvBinding.Subjects = append(updatedMtvBinding.Subjects, mtvBinding.Subjects[0])
				}
			}
			return nil
		})

	if err != nil {
		return err
	}

	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "populator-pv-reader",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumes"},
				Verbs:     []string{"get"},
			},
		},
	}

	err = r.Destination.Client.Create(context.TODO(), &clusterRole, &client.CreateOptions{})
	if err != nil && !k8serr.IsAlreadyExists(err) {
		return err
	}
	crBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "populator-pv-reader-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "populator",
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "populator-pv-reader",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	deploy := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: crBinding.Name}}
	_, err = controllerutil.CreateOrPatch(
		context.TODO(),
		r.Destination.Client,
		deploy, func() error {
			if deploy.CreationTimestamp.IsZero() {
				deploy.Subjects = crBinding.Subjects
				deploy.RoleRef = crBinding.RoleRef
			} else {
				if !slices.Contains(deploy.Subjects, crBinding.Subjects[0]) {
					deploy.Subjects = append(deploy.Subjects, crBinding.Subjects[0])
				}
			}
			return nil
		})
	if err != nil {
		return err
	}

	return nil
}

// addSSHKeysToSecret adds SSH keys from provider controller to the migration secret
func (r *Builder) addSSHKeysToSecret(secret *core.Secret) error {
	// Generate secret names based on provider name
	providerName := r.Source.Provider.Name
	if providerName == "" {
		return fmt.Errorf("provider name is empty")
	}

	privateSecretName, err := util.GenerateSSHPrivateSecretName(providerName)
	if err != nil {
		return fmt.Errorf("error generating private ssh key %v", err)
	}
	publicSecretName, err := util.GenerateSSHPublicSecretName(providerName)
	if err != nil {
		return fmt.Errorf("error generating public ssh key %v", err)
	}
	// Get SSH private key
	privateSecret := &core.Secret{}
	err = r.Get(context.Background(), client.ObjectKey{
		Name:      privateSecretName,
		Namespace: r.Source.Provider.Namespace,
	}, privateSecret)
	if err != nil {
		return fmt.Errorf("failed to get SSH private key secret %s: %w", privateSecretName, err)
	}

	// Get SSH public key
	publicSecret := &core.Secret{}
	err = r.Get(context.Background(), client.ObjectKey{
		Name:      publicSecretName,
		Namespace: r.Source.Provider.Namespace,
	}, publicSecret)
	if err != nil {
		return fmt.Errorf("failed to get SSH public key secret %s: %w", publicSecretName, err)
	}

	// Add SSH keys to the migration secret as base64-encoded environment variables
	if privateKeyData, found := privateSecret.Data["private-key"]; found {
		secret.Data["SSH_PRIVATE_KEY"] = []byte(base64.StdEncoding.EncodeToString(privateKeyData))
	} else {
		return fmt.Errorf("private key not found in secret %s", privateSecretName)
	}

	if publicKeyData, found := publicSecret.Data["public-key"]; found {
		secret.Data["SSH_PUBLIC_KEY"] = []byte(base64.StdEncoding.EncodeToString(publicKeyData))
	} else {
		return fmt.Errorf("public key not found in secret %s", publicSecretName)
	}

	r.Log.Info("SSH keys added to migration secret", "secret", secret.Name)
	return nil
}

func (r *Builder) isPVCExistsInList(pvc *core.PersistentVolumeClaim, pvcList *core.PersistentVolumeClaimList) bool {
	return r.findExistingPVCInList(pvc, pvcList) != nil
}

func (r *Builder) findExistingPVCInList(pvc *core.PersistentVolumeClaim, pvcList *core.PersistentVolumeClaimList) *core.PersistentVolumeClaim {
	pvcIdentifier := r.ResolvePersistentVolumeClaimIdentifier(pvc)
	if pvcIdentifier == "" {
		return nil
	}
	for i := range pvcList.Items {
		item := &pvcList.Items[i]
		if r.ResolvePersistentVolumeClaimIdentifier(item) == pvcIdentifier {
			return item
		}
	}
	return nil
}

func (r *Builder) generatePopulatorSuffix(migrationUID, vmID string, diskKey int32, diskFile string, diskIndex int) string {
	input := fmt.Sprintf("%s-%s-%d-%s-%d", migrationUID, vmID, diskKey, diskFile, diskIndex)
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])[:8]
}

func (r *Builder) ensureXCopyVolumePopulator(vp *api.VSphereXcopyVolumePopulator) error {
	existingPopulator := &api.VSphereXcopyVolumePopulator{}
	err := r.Destination.Client.Get(context.TODO(), client.ObjectKey{
		Namespace: vp.Namespace,
		Name:      vp.Name,
	}, existingPopulator)
	if err != nil {
		if k8serr.IsNotFound(err) {
			r.Log.Info("Creating the populator resource", "VSphereXcopyVolumePopulator", vp.Name, "namespace", vp.Namespace)
			err = r.Destination.Client.Create(context.TODO(), vp, &client.CreateOptions{})
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		r.Log.Info("Populator already exists", "populator", vp.Name, "namespace", vp.Namespace)
	}
	return nil
}

// ConversionPodConfig returns provider-specific configuration for the virt-v2v conversion pod.
// vSphere provider does not require any special configuration.
func (r *Builder) ConversionPodConfig(_ ref.Ref) (*planbase.ConversionPodConfigResult, error) {
	return &planbase.ConversionPodConfigResult{}, nil
}
