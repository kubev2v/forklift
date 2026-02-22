package ovirt

import (
	"context"
	"encoding/pem"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/base"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	utils "github.com/kubev2v/forklift/pkg/controller/plan/util"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ovirt"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	util "github.com/kubev2v/forklift/pkg/lib/util"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BIOS types
const (
	ClusterDefault = "cluster_default"
	Q35Ovmf        = "q35_ovmf"
	Q35SecureBoot  = "q35_secure_boot"
)

// Bus types
const (
	VirtioScsi = "virtio_scsi"
	Virtio     = "virtio"
	Sata       = "sata"
	Scsi       = "scsi"
	IDE        = "ide"
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
)

// Map of ovirt guest ids to osinfo ids.
var osMap = map[string]string{
	"rhel_6_10_plus_ppc64": "rhel6.10",
	"rhel_6_ppc64":         "rhel6.10",
	"rhel_6":               "rhel6.10",
	"rhel_6x64":            "rhel6.10",
	"rhel_6_9_plus_ppc64":  "rhel6.9",
	"rhel_7_ppc64":         "rhel7.7",
	"rhel_7_s390x":         "rhel7.7",
	"rhel_7x64":            "rhel7.7",
	"rhel_8x64":            "rhel8.1",
	"rhel_9x64":            "rhel9.1",
	"sles_11_ppc64":        "opensuse15.0",
	"sles_11":              "opensuse15.0",
	"sles_12_s390x":        "opensuse15.0",
	"ubuntu_12_04":         "ubuntu18.04",
	"ubuntu_12_10":         "ubuntu18.04",
	"ubuntu_13_04":         "ubuntu18.04",
	"ubuntu_13_10":         "ubuntu18.04",
	"ubuntu_14_04_ppc64":   "ubuntu18.04",
	"ubuntu_14_04":         "ubuntu18.04",
	"ubuntu_16_04_s390x":   "ubuntu18.04",
	"windows_10":           "win10",
	"windows_10x64":        "win10",
	"windows_2003":         "win10",
	"windows_2003x64":      "win10",
	"windows_2008R2x64":    "win2k8",
	"windows_2008":         "win2k8",
	"windows_2008x64":      "win2k8",
	"windows_2012R2x64":    "win2k12r2",
	"windows_2012x64":      "win2k12r2",
	"windows_2016x64":      "win2k16",
	"windows_2019x64":      "win2k19",
	"windows_2022":         "win2k22",
	"windows_7":            "win10",
	"windows_7x64":         "win10",
	"windows_8":            "win10",
	"windows_8x64":         "win10",
	"windows_xp":           "win10",
	"windows_11":           "win11",
}

// oVirt builder.
type Builder struct {
	*plancontext.Context
}

// Create DataVolume certificate configmap.
func (r *Builder) ConfigMap(_ ref.Ref, in *core.Secret, object *core.ConfigMap) (err error) {
	// For CNV 4.21+, ConfigMap is only needed in secure mode (when insecureSkipVerify is not used).
	// For CNV < 4.21, ConfigMap is required even in insecure mode as a fallback,
	// since the InsecureSkipVerify field is not supported.
	if cacert, exists := in.Data["cacert"]; exists && len(cacert) > 0 {
		object.BinaryData["ca.pem"] = cacert
	} else {
		// If no CA cert provided, try to fetch it from the oVirt engine.
		// This is needed for CNV < 4.21 when insecure mode is enabled but
		// InsecureSkipVerify field is not supported.
		cacert, err = r.fetchOVirtCACert()
		if err != nil {
			r.Log.Error(err, "Failed to fetch CA certificate from oVirt engine")
			// Don't return error - let migration proceed and fail with clear error if cert is actually needed
			err = nil
		} else if len(cacert) > 0 {
			object.BinaryData["ca.pem"] = cacert
		}
	}
	return
}

// Fetches the CA certificate from the oVirt engine URL.
// This is needed for CNV < 4.21 in insecure mode as a fallback when InsecureSkipVerify field is not supported.
func (r *Builder) fetchOVirtCACert() (cert []byte, err error) {
	engineURL := r.Source.Provider.Spec.URL
	parsedURL, err := url.Parse(engineURL)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to parse oVirt engine URL", "url", engineURL)
	}

	tempSecret := &core.Secret{
		Data: map[string][]byte{
			"insecureSkipVerify": []byte("true"),
		},
	}

	cacert, err := util.GetTlsCertificate(parsedURL, tempSecret)
	if err != nil {
		return nil, liberr.Wrap(err, "failed to fetch certificate from oVirt engine")
	}
	if cacert == nil {
		return nil, liberr.New("no certificate returned from oVirt engine")
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cacert.Raw,
	})

	return certPEM, nil
}

func (r *Builder) PodEnvironment(_ ref.Ref, _ *core.Secret) (env []core.EnvVar, err error) {
	return
}

// Build the DataVolume credential secret.
func (r *Builder) Secret(_ ref.Ref, in, object *core.Secret) (err error) {
	object.StringData = map[string]string{
		"accessKeyId": string(in.Data["user"]),
		"secretKey":   string(in.Data["password"]),
	}
	return
}

// Create DataVolume specs for the VM.
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *core.ConfigMap) (dvs []cdi.DataVolume, err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	url := r.Source.Provider.Spec.URL

	dsMapIn := r.Context.Map.Storage.Spec.Map
	for i := range dsMapIn {
		mapped := &dsMapIn[i]
		ref := mapped.Source
		sd := &model.StorageDomain{}
		if err = r.Source.Inventory.Find(sd, ref); err != nil {
			return
		}
		for _, da := range vm.DiskAttachments {
			if da.Disk.StorageType == "image" && da.Disk.StorageDomain == sd.ID {
				storageClass := mapped.Destination.StorageClass
				size := da.Disk.ProvisionedSize
				if da.Disk.ActualSize > size {
					size = da.Disk.ActualSize
				}

				insecure := base.GetInsecureSkipVerifyFlag(r.Source.Secret)

				imageioSource := &cdi.DataVolumeSourceImageIO{
					URL:       url,
					DiskID:    da.Disk.ID,
					SecretRef: secret.Name,
				}

				if insecure && settings.Settings.InsecureSkipVerifySupported {
					// CNV 4.21+: Use CDI's insecureSkipVerify field to skip TLS verification
					imageioSource.InsecureSkipVerify = &insecure
				} else {
					// CNV < 4.21 or secure mode: Use ConfigMap with CA cert
					// For older CNV versions with insecure flag, fall back to using CA cert
					// since InsecureSkipVerify field is not supported
					imageioSource.CertConfigMap = configMap.Name
				}
				dvSpec := cdi.DataVolumeSpec{
					Source: &cdi.DataVolumeSource{
						Imageio: imageioSource,
					},
					Storage: &cdi.StorageSpec{
						Resources: core.VolumeResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: *resource.NewQuantity(size, resource.BinarySI),
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
				dv.ObjectMeta.Annotations[planbase.AnnDiskSource] = da.Disk.ID
				dvs = append(dvs, *dv)
			}
		}
	}

	return
}

// Create the destination Kubevirt VM.
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) (err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	if object.Template == nil {
		object.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}
	r.mapDisks(vm, persistentVolumeClaims, object)
	r.mapFirmware(vm, &vm.Cluster, object)
	if !usesInstanceType {
		r.mapCPU(vm, object)
		r.mapMemory(vm, object)
	}
	r.mapClock(vm, object)
	r.mapInput(object)
	r.mapTpm(vm, object)
	err = r.mapNetworks(vm, object)
	if err != nil {
		return
	}

	return
}

func (r *Builder) mapNetworks(vm *model.Workload, object *cnv.VirtualMachineSpec) (err error) {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

	numNetworks := 0
	hasUDN := r.Plan.DestinationHasUdnNetwork(r.Destination)
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
		needed := []model.XNIC{}
		for _, nic := range vm.NICs {
			if nic.Profile.Network == network.ID {
				needed = append(needed, nic)
			}
		}
		if len(needed) == 0 {
			continue
		}
		for _, nic := range needed {
			networkName := fmt.Sprintf("net-%v", numNetworks)
			numNetworks++
			kNetwork := cnv.Network{
				Name: networkName,
			}
			kInterface := cnv.Interface{
				Name:  networkName,
				Model: nic.Interface,
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
				} else {
					kInterface.Masquerade = &cnv.InterfaceMasquerade{}
				}
			case Multus:
				kNetwork.Multus = &cnv.MultusNetwork{
					NetworkName: path.Join(mapped.Destination.Namespace, mapped.Destination.Name),
				}
				if nic.Profile.PassThrough {
					kInterface.SRIOV = &cnv.InterfaceSRIOV{}
				} else {
					kInterface.Bridge = &cnv.InterfaceBridge{}
				}
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

func (r *Builder) mapClock(vm *model.Workload, object *cnv.VirtualMachineSpec) {
	if object.Template.Spec.Domain.Clock == nil {
		object.Template.Spec.Domain.Clock = &cnv.Clock{}
	}

	timezone := cnv.ClockOffsetTimezone(vm.Timezone)
	object.Template.Spec.Domain.Clock.Timezone = &timezone
}

func (r *Builder) mapMemory(vm *model.Workload, object *cnv.VirtualMachineSpec) {
	reservation := resource.NewQuantity(vm.Memory, resource.BinarySI)
	object.Template.Spec.Domain.Memory = &cnv.Memory{Guest: reservation}
}

func (r *Builder) mapCPU(vm *model.Workload, object *cnv.VirtualMachineSpec) {
	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: uint32(vm.CpuSockets),
		Cores:   uint32(vm.CpuCores),
		Threads: uint32(vm.CpuThreads),
	}

	if vm.CpuPinningPolicy == model.Dedicated {
		object.Template.Spec.Domain.CPU.DedicatedCPUPlacement = true
	}
	if vm.CustomCpuModel != "" {
		r.setCpuFlags(vm.CustomCpuModel, object)
	} else if r.Plan.Spec.PreserveClusterCPUModel {
		r.setCpuFlags(r.getClusterCpu(vm), object)
	}
}

func (r *Builder) getClusterCpu(vm *model.Workload) string {
	var cpuAndFlags string
	cpus := strings.Split(vm.ServerCpu.SystemOptionValue[0].Value, ";")
	for _, values := range cpus {
		if strings.Contains(values, vm.Cluster.CPU.Type) {
			cpuAndFlags = strings.Split(values, ":")[3]
			break
		}
	}
	return cpuAndFlags
}

func (r *Builder) setCpuFlags(fullCpu string, object *cnv.VirtualMachineSpec) {
	cpuTypeAndFlags := strings.Split(fullCpu, ",")
	object.Template.Spec.Domain.CPU.Model = cpuTypeAndFlags[0]
	for _, val := range cpuTypeAndFlags[1:] {
		if flag, found := strings.CutPrefix(val, "+"); found {
			object.Template.Spec.Domain.CPU.Features = append(object.Template.Spec.Domain.CPU.Features, cnv.CPUFeature{Name: flag, Policy: "require"})
		} else if flag, found = strings.CutPrefix(val, "-"); found {
			object.Template.Spec.Domain.CPU.Features = append(object.Template.Spec.Domain.CPU.Features, cnv.CPUFeature{Name: flag, Policy: "disable"})
		} else {
			object.Template.Spec.Domain.CPU.Features = append(object.Template.Spec.Domain.CPU.Features, cnv.CPUFeature{Name: flag})
		}
	}
}

func (r *Builder) mapFirmware(vm *model.Workload, cluster *model.Cluster, object *cnv.VirtualMachineSpec) {
	biosType := vm.BIOS
	if biosType == ClusterDefault {
		biosType = cluster.BiosType
	}
	serial := vm.SerialNumber
	if serial == "" {
		serial = vm.ID
	}
	firmware := &cnv.Firmware{
		Serial: serial,
		UUID:   types.UID(vm.ID),
	}
	switch biosType {
	case Q35Ovmf, Q35SecureBoot:
		// We disable secure boot even if it was enabled on the source because the guest OS won't
		// be able to boot without getting the NVRAM data. So we start the VM without secure boot
		// to ease the procedure users need to do in order to make the guest OS to boot.
		secureBootEnabled := false
		firmware.Bootloader = &cnv.Bootloader{
			EFI: &cnv.EFI{
				SecureBoot: &secureBootEnabled,
			}}
	default:
		firmware.Bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	}
	object.Template.Spec.Domain.Firmware = firmware
}

func (r *Builder) mapDisks(vm *model.Workload, persistentVolumeClaims []*core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk

	pvcMap := make(map[string]*core.PersistentVolumeClaim)
	for i := range persistentVolumeClaims {
		pvc := persistentVolumeClaims[i]
		pvcMap[r.ResolvePersistentVolumeClaimIdentifier(pvc)] = pvc
	}

	for _, da := range vm.DiskAttachments {
		claimName := pvcMap[da.Disk.ID].Name
		volumeName := da.Disk.ID
		var bus string
		switch da.Interface {
		case VirtioScsi:
			bus = Scsi
		case Sata, IDE:
			bus = Sata
		default:
			bus = Virtio
		}
		var disk cnv.Disk
		if da.Disk.Disk.StorageType == "lun" {
			claimName = volumeName
			disk = cnv.Disk{
				Name: volumeName,
				DiskDevice: cnv.DiskDevice{
					LUN: &cnv.LunTarget{
						Bus: cnv.DiskBus(bus),
					},
				},
			}
		} else {
			disk = cnv.Disk{
				Name: volumeName,
				DiskDevice: cnv.DiskDevice{
					Disk: &cnv.DiskTarget{
						Bus: cnv.DiskBus(bus),
					},
				},
				Serial: da.Disk.ID,
			}
		}
		volume := cnv.Volume{
			Name: volumeName,
			VolumeSource: cnv.VolumeSource{
				PersistentVolumeClaim: &cnv.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: core.PersistentVolumeClaimVolumeSource{
						ClaimName: claimName,
					},
				},
			},
		}
		if da.DiskAttachment.Bootable {
			var bootOrder uint = 1
			disk.BootOrder = &bootOrder
		}

		kVolumes = append(kVolumes, volume)
		kDisks = append(kDisks, disk)
	}
	object.Template.Spec.Volumes = kVolumes
	object.Template.Spec.Domain.Devices.Disks = kDisks
}

func (r *Builder) mapTpm(vm *model.Workload, object *cnv.VirtualMachineSpec) {
	if vm.OSType == "windows_2022" || vm.OSType == "windows_11" {
		persistData := true
		object.Template.Spec.Domain.Devices.TPM = &cnv.TPMDevice{Persistent: &persistData}
	}
}

// Build tasks.
func (r *Builder) Tasks(vmRef ref.Ref) (list []*plan.Task, err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
	}
	for _, da := range vm.DiskAttachments {
		// We don't add a task for LUNs because we don't copy their content but rather assume we can connect to
		// the LUNs that are used in the source environment also from the target environment.
		if da.Disk.StorageType != "lun" {
			mB := da.Disk.ProvisionedSize / 0x100000
			list = append(
				list,
				&plan.Task{
					Name: da.Disk.ID,
					Progress: libitr.Progress{
						Total: mB,
					},
					Annotations: map[string]string{
						"unit": "MB",
					},
				})
		}
	}

	return
}

func (r *Builder) PreferenceName(vmRef ref.Ref, configMap *core.ConfigMap) (name string, err error) {
	vm := &model.Workload{}
	if err = r.Source.Inventory.Find(vm, vmRef); err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	name, ok := configMap.Data[vm.OSType]
	if !ok {
		err = liberr.Wrap(err, "vm", vmRef.String())
	}
	return
}

func (r *Builder) ConfigMaps(vmRef ref.Ref) (list []core.ConfigMap, err error) {
	return nil, nil
}

func (r *Builder) Secrets(vmRef ref.Ref) (list []core.Secret, err error) {
	return nil, nil
}

func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	os, ok := osMap[vm.OSType]
	if !ok {
		if strings.Contains(vm.OSType, "linux") || strings.Contains(vm.OSType, "rhel") {
			os = DefaultLinux
		} else if strings.Contains(vm.OSType, "win") {
			os = DefaultWindows
		} else {
			os = Unknown
		}
	}

	labels = make(map[string]string)
	labels[fmt.Sprintf(TemplateOSLabel, os)] = "true"
	labels[TemplateWorkloadLabel] = "true"
	labels[TemplateFlavorLabel] = "true"

	return
}

// Return a stable identifier for a DataVolume.
func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return dv.ObjectMeta.Annotations[planbase.AnnDiskSource]
}

// Return a stable identifier for a PersistentDataVolume.
func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	return pvc.Annotations[planbase.AnnDiskSource]
}

// Create PVs specs for the VM LUNs.
func (r *Builder) LunPersistentVolumes(vmRef ref.Ref) (pvs []core.PersistentVolume, err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	for _, da := range vm.DiskAttachments {
		if da.Disk.StorageType == "lun" {
			volMode := core.PersistentVolumeBlock
			logicalUnit := da.Disk.Lun.LogicalUnits.LogicalUnit[0]

			var pvSource core.PersistentVolumeSource
			if logicalUnit.Address != "" {
				pvSource = core.PersistentVolumeSource{
					ISCSI: &core.ISCSIPersistentVolumeSource{
						TargetPortal: logicalUnit.Address + ":" + logicalUnit.Port,
						IQN:          logicalUnit.Target,
						Lun:          logicalUnit.LunMapping,
						ReadOnly:     false,
					},
				}
			} else {
				pvSource = core.PersistentVolumeSource{
					FC: &core.FCVolumeSource{
						WWIDs:    []string{logicalUnit.LunID},
						ReadOnly: false,
					},
				}
			}

			pvSpec := core.PersistentVolume{
				ObjectMeta: meta.ObjectMeta{
					Name:      da.Disk.ID,
					Namespace: r.Plan.Spec.TargetNamespace,
					Annotations: map[string]string{
						planbase.AnnDiskSource: da.Disk.ID,
						"lun":                  "true",
					},
					Labels: r.Labeler.VMLabelsWithExtra(vmRef, map[string]string{
						"volume": fmt.Sprintf("%v-%v", vm.Name, da.ID),
					}),
				},
				Spec: core.PersistentVolumeSpec{
					PersistentVolumeSource: pvSource,
					Capacity: core.ResourceList{
						core.ResourceStorage: *resource.NewQuantity(logicalUnit.Size, resource.BinarySI),
					},
					AccessModes: []core.PersistentVolumeAccessMode{
						core.ReadWriteMany,
					},
					VolumeMode: &volMode,
				},
			}
			pvs = append(pvs, pvSpec)
		}
	}
	return
}

// Create PVCs specs for the VM LUNs.
func (r *Builder) LunPersistentVolumeClaims(vmRef ref.Ref) (pvcs []core.PersistentVolumeClaim, err error) {
	vm := &model.Workload{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}
	for _, da := range vm.DiskAttachments {
		if da.Disk.StorageType == "lun" {
			sc := ""
			volMode := core.PersistentVolumeBlock
			pvcSpec := core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      da.Disk.ID,
					Namespace: r.Plan.Spec.TargetNamespace,
					Annotations: map[string]string{
						planbase.AnnDiskSource: da.Disk.ID,
						"lun":                  "true",
					},
					Labels: r.Labeler.VMLabels(vmRef),
				},
				Spec: core.PersistentVolumeClaimSpec{
					AccessModes: []core.PersistentVolumeAccessMode{
						core.ReadWriteMany,
					},
					Selector: &meta.LabelSelector{
						MatchLabels: map[string]string{
							"volume": fmt.Sprintf("%v-%v", vm.Name, da.ID),
						},
					},
					StorageClassName: &sc,
					VolumeMode:       &volMode,
					Resources: core.VolumeResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceStorage: *resource.NewQuantity(da.Disk.Lun.LogicalUnits.LogicalUnit[0].Size, resource.BinarySI),
						},
					},
				},
			}
			pvcs = append(pvcs, pvcSpec)
		}
	}
	return
}

func (r *Builder) SupportsVolumePopulators() bool {
	return !r.Context.Plan.IsWarm() && r.Context.Plan.Provider.Destination.IsHost()
}

func (r *Builder) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) (pvcs []*core.PersistentVolumeClaim, err error) {
	workload := &model.Workload{}
	err = r.Source.Inventory.Find(workload, vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	var sdToStorageClass map[string]string
	for _, diskAttachment := range workload.DiskAttachments {
		if diskAttachment.Disk.StorageType == "lun" {
			continue
		}
		_, err = r.getVolumePopulator(diskAttachment.DiskAttachment.ID)
		if err != nil {
			if !k8serr.IsNotFound(err) {
				err = liberr.Wrap(err)
				return
			}
			var populatorName string
			populatorName, err = r.createVolumePopulatorCR(diskAttachment, secretName, vmRef.ID)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
			var pvc *core.PersistentVolumeClaim
			if sdToStorageClass == nil {
				if sdToStorageClass, err = r.mapStorageDomainToStorageClass(); err != nil {
					return
				}
			}
			storageClassName := sdToStorageClass[diskAttachment.Disk.StorageDomain]
			pvc, err = r.persistentVolumeClaimWithSourceRef(diskAttachment, storageClassName, populatorName, annotations, vmRef.ID)
			if err != nil {
				if !k8serr.IsAlreadyExists(err) {
					err = liberr.Wrap(err, "disk attachment", diskAttachment.DiskAttachment.ID, "storage class", storageClassName, "populator", populatorName)
					return
				}
				err = nil
				continue
			}
			pvcs = append(pvcs, pvc)
		}
	}
	return
}

func (r *Builder) mapStorageDomainToStorageClass() (map[string]string, error) {
	sdToStorageClass := make(map[string]string)
	for _, mapped := range r.Context.Map.Storage.Spec.Map {
		sd := &model.StorageDomain{}
		if err := r.Source.Inventory.Find(sd, mapped.Source); err != nil {
			return nil, liberr.Wrap(err)
		}
		sdToStorageClass[sd.ID] = mapped.Destination.StorageClass
	}
	return sdToStorageClass, nil
}

// Get the OvirtVolumePopulator CustomResource based on the disk ID.
func (r *Builder) getVolumePopulator(diskID string) (populatorCr api.OvirtVolumePopulator, err error) {
	list := api.OvirtVolumePopulatorList{}
	err = r.Destination.Client.List(context.TODO(), &list, &client.ListOptions{
		Namespace: r.Plan.Spec.TargetNamespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"migration": string(r.Migration.UID),
			"diskID":    diskID,
		}),
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if len(list.Items) == 0 {
		err = k8serr.NewNotFound(api.SchemeGroupVersion.WithResource("OvirtVolumePopulator").GroupResource(), diskID)
		return
	}
	if len(list.Items) > 1 {
		err = liberr.New("Multiple OvirtVolumePopulator CRs found for the same disk", "diskID", diskID)
		return
	}

	populatorCr = list.Items[0]

	return
}

func (r *Builder) createVolumePopulatorCR(diskAttachment model.XDiskAttachment, secretName, vmId string) (name string, err error) {
	providerURL, err := url.Parse(r.Source.Provider.Spec.URL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	engineURL := url.URL{
		Scheme: providerURL.Scheme,
		Host:   providerURL.Host,
	}
	populatorCR := &api.OvirtVolumePopulator{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", diskAttachment.DiskAttachment.ID),
			Namespace:    r.Plan.Spec.TargetNamespace,
			Labels: r.Labeler.VMLabelsWithExtra(ref.Ref{ID: vmId}, map[string]string{
				"diskID": diskAttachment.Disk.ID,
			}),
		},
		Spec: api.OvirtVolumePopulatorSpec{
			EngineURL:        engineURL.String(),
			EngineSecretName: secretName,
			DiskID:           diskAttachment.Disk.ID,
			TransferNetwork:  r.Plan.Spec.TransferNetwork,
		},
	}
	err = r.Context.Client.Create(context.TODO(), populatorCR, &client.CreateOptions{})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	name = populatorCR.Name
	return
}

func (r *Builder) getDefaultVolumeAndAccessMode(storageClassName string) ([]core.PersistentVolumeAccessMode, *core.PersistentVolumeMode, error) {
	var filesystemMode = core.PersistentVolumeFilesystem
	storageProfile := &cdi.StorageProfile{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: storageClassName}, storageProfile)
	if err != nil {
		return nil, nil, liberr.Wrap(err)
	}

	if len(storageProfile.Status.ClaimPropertySets) > 0 &&
		len(storageProfile.Status.ClaimPropertySets[0].AccessModes) > 0 {
		accessModes := storageProfile.Status.ClaimPropertySets[0].AccessModes
		volumeMode := storageProfile.Status.ClaimPropertySets[0].VolumeMode
		if volumeMode == nil {
			// volumeMode is an optional API parameter. Filesystem is the default mode used when volumeMode parameter is omitted.
			volumeMode = &filesystemMode
		}
		return accessModes, volumeMode, nil
	}
	// no accessMode configured on storageProfile
	return nil, nil, liberr.New("no accessMode defined on StorageProfile for StorageClass", "storageName", storageClassName)
}

// Build a PersistentVolumeClaim with DataSourceRef for VolumePopulator
func (r *Builder) persistentVolumeClaimWithSourceRef(diskAttachment model.XDiskAttachment,
	storageClassName string,
	populatorName string,
	annotations map[string]string,
	vmID string) (pvc *core.PersistentVolumeClaim, err error) {
	diskSize := diskAttachment.Disk.ProvisionedSize
	var accessModes []core.PersistentVolumeAccessMode
	var volumeMode *core.PersistentVolumeMode
	accessModes, volumeMode, err = r.getDefaultVolumeAndAccessMode(storageClassName)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	// We add 10% overhead because of the fsOverhead in CDI, around 5% to ext4 and 5% for root partition.
	// This value is configurable using `FILESYSTEM_OVERHEAD`
	// Encrypted Ceph RBD makes the pod see less space, this possible overhead needs to be taken into account.
	// For Block the value is configurable using `BLOCK_OVERHEAD`
	diskSize = utils.CalculateSpaceWithOverhead(diskSize, volumeMode)

	annotations[planbase.AnnDiskSource] = diskAttachment.ID

	pvc = &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", diskAttachment.DiskAttachment.ID),
			Namespace:    r.Plan.Spec.TargetNamespace,
			Annotations:  annotations,
			Labels: r.Labeler.VMLabelsWithExtra(ref.Ref{ID: vmID}, map[string]string{
				"diskID": diskAttachment.Disk.ID,
			}),
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: core.VolumeResourceRequirements{
				Requests: map[core.ResourceName]resource.Quantity{
					core.ResourceStorage: *resource.NewQuantity(diskSize, resource.BinarySI)},
			},
			StorageClassName: &storageClassName,
			VolumeMode:       volumeMode,
			DataSourceRef: &core.TypedObjectReference{
				APIGroup: &api.SchemeGroupVersion.Group,
				Kind:     api.OvirtVolumePopulatorKind,
				Name:     populatorName,
			},
		},
	}

	err = r.Client.Create(context.TODO(), pvc, &client.CreateOptions{})
	return
}

func (r *Builder) PopulatorTransferredBytes(pvc *core.PersistentVolumeClaim) (transferredBytes int64, err error) {
	if _, ok := pvc.Annotations["lun"]; ok {
		// skip LUNs
		return
	}

	diskID := pvc.Annotations[planbase.AnnDiskSource]

	populatorCr, err := r.getVolumePopulator(diskID)
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

// Sets the OvirtVolumePopulator CRs with VM ID and migration ID into the labels.
func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) (err error) {
	ovirtVm := &model.Workload{}
	err = r.Source.Inventory.Find(ovirtVm, vmRef)
	if err != nil {
		return
	}
	var diskIds []string
	for _, da := range ovirtVm.DiskAttachments {
		diskIds = append(diskIds, da.Disk.ID)
	}
	if len(diskIds) != len(pvcs) {
		// To be sure we have every disk based on what already migrated and what's not.
		// e.g when initializing the plan and the PVC has not been created yet (but the populator CR is) or when the disks that are attached to the source VM change.
		for _, pvc := range pvcs {
			diskIds = append(diskIds, pvc.Spec.DataSource.Name)
		}
	}
	migrationID := r.ActiveMigrationUID()
	for _, id := range diskIds {
		populatorCr, err := r.getVolumePopulator(id)
		if err != nil {
			continue
		}
		err = r.setOvirtPopulatorLabels(populatorCr, vmRef.ID, migrationID)
		if err != nil {
			r.Log.Error(err, "Couldn't update the Populator Custom Resource labels.",
				"vmID", vmRef.ID, "migrationID", migrationID, "OvirtVolumePopulator", populatorCr.Name)
			continue
		}
	}
	return
}

func (r *Builder) setOvirtPopulatorLabels(populatorCr api.OvirtVolumePopulator, vmId, migrationId string) (err error) {
	populatorCrCopy := populatorCr.DeepCopy()
	if populatorCr.Labels == nil {
		populatorCr.Labels = make(map[string]string)
	}
	populatorCr.Labels["vmID"] = vmId
	populatorCr.Labels["migration"] = migrationId
	patch := client.MergeFrom(populatorCrCopy)
	err = r.Destination.Client.Patch(context.TODO(), &populatorCr, patch)
	return
}

func (r *Builder) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error) {
	taskName = pvc.Annotations[planbase.AnnDiskSource]
	return
}

// ConversionPodConfig returns provider-specific configuration for the virt-v2v conversion pod.
// oVirt provider does not require any special configuration.
func (r *Builder) ConversionPodConfig(_ ref.Ref) (*planbase.ConversionPodConfigResult, error) {
	return &planbase.ConversionPodConfigResult{}, nil
}
