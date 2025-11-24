package dynamic

import (
	"fmt"
	"math"
	"path"
	"strconv"
	"strings"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/inventory/unstructured"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// Firmware types
const (
	BIOS = "bios"
	UEFI = "uefi"
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
	Unknown = "unknown"
)

// DynamicBuilder is a generic builder that works with ANY dynamic provider.
// It uses schema definitions from the Provider CRD and unstructured data access.
type Builder struct {
	*plancontext.Context
	schema *ProviderSchema
}

// NewBuilder creates a new dynamic builder with the given schema.
func NewBuilder(ctx *plancontext.Context, schema *ProviderSchema) *Builder {
	return &Builder{
		Context: ctx,
		schema:  schema,
	}
}

// ConfigMap creates DataVolume certificate configmap.
// No-op for dynamic providers.
func (r *Builder) ConfigMap(_ ref.Ref, _ *core.Secret, _ *core.ConfigMap) (err error) {
	return
}

// PodEnvironment builds environment variables for the conversion pod.
func (r *Builder) PodEnvironment(vmRef ref.Ref, sourceSecret *core.Secret) (env []core.EnvVar, err error) {
	// Get VM using unstructured
	vm := &unstructured.Unstructured{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	// Extract name
	namePath, _ := r.schema.VM.GetField("name")
	name, _ := vm.GetString(namePath)

	// Extract OVA path if available
	ovaPath := ""
	if ovaPathField, found := r.schema.VM.GetField("path"); found {
		ovaPath, _ = vm.GetString(ovaPathField)
	}

	env = append(
		env,
		core.EnvVar{
			Name:  "V2V_vmName",
			Value: name,
		},
		core.EnvVar{
			Name:  "V2V_diskPath",
			Value: getDiskSourcePath(ovaPath),
		},
		core.EnvVar{
			Name:  "V2V_source",
			Value: string(r.Source.Provider.Type()),
		})

	return
}

// Secret builds the DataVolume credential secret.
func (r *Builder) Secret(vmRef ref.Ref, in, object *core.Secret) (err error) {
	return
}

// DataVolumes creates DataVolume specs for the VM disks.
//
// For dynamic providers, this method calls the provider's datavolume-source endpoint
// to get the CDI DataVolumeSource configuration for each disk.
//
// Behavior based on RequiresConversion flag:
//   - RequiresConversion = true:  Creates INPUT DataVolumes with PVC source
//     Controller will create OUTPUT blank DataVolumes separately
//   - RequiresConversion = false: Creates OUTPUT DataVolumes with PVC source
//     No additional DataVolumes needed
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *core.ConfigMap) (dvs []cdi.DataVolume, err error) {
	// Get VM using unstructured
	vm := &unstructured.Unstructured{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	// Get VM ID
	vmIDPath, _ := r.schema.VM.GetField("id")
	vmID, _ := vm.GetString(vmIDPath)

	// Get disks from VM
	disksPath, _ := r.schema.VM.GetField("disks")
	disksData, found := vm.GetSlice(disksPath)
	if !found {
		return
	}

	// Create datavolume client
	dvClient := NewDataVolumeClient(r.Source.Provider)

	// Get storage mappings
	storageMapIn := r.Context.Map.Storage.Spec.Map
	for i := range storageMapIn {
		mapped := &storageMapIn[i]
		ref := mapped.Source

		// Find storage in inventory
		storage := &unstructured.Unstructured{}
		fErr := r.Source.Inventory.Find(storage, ref)
		if fErr != nil {
			err = fErr
			return
		}

		storageIDPath, _ := r.schema.Storage.GetField("id")
		storageID, _ := storage.GetString(storageIDPath)

		// Match disks to this storage
		for diskIndex, diskData := range disksData {
			disk := &unstructured.Unstructured{}
			disk.Object = diskData.(map[string]interface{})

			diskIDPath, _ := r.schema.Disk.GetField("id")
			diskID, _ := disk.GetString(diskIDPath)

			if diskID == storageID {
				var dv *cdi.DataVolume
				dv, err = r.mapDataVolume(vmID, diskID, diskIndex, mapped.Destination, dvTemplate, dvClient)
				if err != nil {
					return
				}
				dvs = append(dvs, *dv)
			}
		}
	}

	return
}

func (r *Builder) mapDataVolume(vmID, diskID string, diskIndex int, destination v1beta1.DestinationStorage, dvTemplate *cdi.DataVolume, dvClient *DataVolumeClient) (dv *cdi.DataVolume, err error) {
	// Call provider's datavolume-source endpoint to get CDI source
	sourceResponse, err := dvClient.GetDataVolumeSource(
		vmID,
		diskID,
		r.Plan.Spec.TargetNamespace,
		destination.StorageClass)
	if err != nil {
		err = liberr.Wrap(err, "failed to get datavolume source from provider",
			"vmID", vmID,
			"diskID", diskID)
		return
	}

	// Parse size from response
	diskSize, err := resource.ParseQuantity(sourceResponse.Size)
	if err != nil {
		err = liberr.Wrap(err, "failed to parse disk size", "size", sourceResponse.Size)
		return
	}

	storageClass := destination.StorageClass

	// Use the CDI source from provider response
	dvSource := sourceResponse.Source

	dvSpec := cdi.DataVolumeSpec{
		Source: &dvSource,
		Storage: &cdi.StorageSpec{
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: diskSize,
				},
			},
			StorageClassName: &storageClass,
		},
	}

	// Set access mode and volume mode if specified
	if destination.AccessMode != "" {
		dvSpec.Storage.AccessModes = []core.PersistentVolumeAccessMode{destination.AccessMode}
	}
	if destination.VolumeMode != "" {
		dvSpec.Storage.VolumeMode = &destination.VolumeMode
	}

	dv = dvTemplate.DeepCopy()
	dv.Spec = dvSpec

	// Add annotations
	if dv.ObjectMeta.Annotations == nil {
		dv.ObjectMeta.Annotations = make(map[string]string)
	}

	// Preserve disk index for matching PVCs to VM disks
	dv.ObjectMeta.Annotations[planbase.AnnDiskIndex] = fmt.Sprintf("%d", diskIndex)
	dv.ObjectMeta.Annotations[planbase.AnnDiskSource] = diskID

	// Mark if this is an input DataVolume (for conversion)
	if r.Source.Provider.RequiresConversion() {
		// This is an INPUT DataVolume - CDI will populate it from the PVC source
		// The controller will create a matching OUTPUT blank DataVolume
		dv.ObjectMeta.Annotations["forklift.konveyor.io/datavolume-role"] = "input"
		dv.ObjectMeta.Name = dv.ObjectMeta.Name + "-input"
	} else {
		// This is an OUTPUT DataVolume - CDI will clone directly from source to target
		dv.ObjectMeta.Annotations["forklift.konveyor.io/datavolume-role"] = "output"
	}

	return
}

// VirtualMachine creates the destination Kubevirt VM using the generic schema-based builder.
//
// GENERIC BUILDER: This builder is used when the provider does NOT declare custom builder support
// in its DynamicProviderServer.Spec.Features.SupportsCustomBuilder field.
//
// The controller checks the feature flag:
// - SupportsCustomBuilder = true  → Uses provider's POST /vms/{id}/build-spec API
// - SupportsCustomBuilder = false → Uses this generic builder (schema-based)
//
// GENERIC APPROACH:
// - Uses schema definitions from Provider CRD
// - Accesses inventory data via unstructured interface
// - Provides baseline VM spec building for any dynamic provider
// - Basic OS detection, firmware, CPU, memory, network, and disk mapping
//
// CUSTOM BUILDER BENEFITS (when implemented by provider):
// - Uses native metadata (e.g., OVF for OVA)
// - Better OS and firmware detection
// - Provider-specific optimizations
// - Can use guest tools data
// - See: pkg/controller/plan/adapter/dynamic/providerclient.go
//
// This generic implementation allows providers to start simple and add custom
// builder support later for optimization.
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) (err error) {
	// Get VM using unstructured
	vm := &unstructured.Unstructured{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	if object.Template == nil {
		object.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}

	r.mapDisks(vm, persistentVolumeClaims, object)
	r.mapFirmware(vm, vmRef, object)
	r.mapInput(object)

	if !usesInstanceType {
		r.mapCPU(vm, object)
		err = r.mapMemory(vm, object)
		if err != nil {
			return
		}
	}

	err = r.mapNetworks(vm, object)
	if err != nil {
		return
	}

	return
}

// mapNetworks handles network mapping for the generic builder fallback.
func (r *Builder) mapNetworks(vm *unstructured.Unstructured, object *cnv.VirtualMachineSpec) (err error) {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

	numNetworks := 0
	hasUDN := r.Plan.DestinationHasUdnNetwork(r.Destination)
	netMapIn := r.Context.Map.Network.Spec.Map

	// Get NICs from VM
	nicsPath, _ := r.schema.VM.GetField("nics")
	nicsData, found := vm.GetSlice(nicsPath)
	if !found {
		nicsData = []interface{}{}
	}

	for i := range netMapIn {
		mapped := &netMapIn[i]

		// Skip network mappings with destination type 'Ignored'
		if mapped.Destination.Type == Ignored {
			continue
		}

		ref := mapped.Source

		// Find network in inventory
		network := &unstructured.Unstructured{}
		fErr := r.Source.Inventory.Find(network, ref)
		if fErr != nil {
			err = fErr
			return
		}

		networkNamePath, _ := r.schema.Network.GetField("name")
		networkName, _ := network.GetString(networkNamePath)

		// Find NICs that use this network
		needed := []map[string]interface{}{}
		for _, nicData := range nicsData {
			nic := &unstructured.Unstructured{}
			nic.Object = nicData.(map[string]interface{})
			nicNetworkPath, _ := r.schema.NIC.GetField("network")
			nicNetwork, _ := nic.GetString(nicNetworkPath)

			if nicNetwork == networkName {
				needed = append(needed, nicData.(map[string]interface{}))
			}
		}

		if len(needed) == 0 {
			continue
		}

		for _, nicData := range needed {
			nic := &unstructured.Unstructured{}
			nic.Object = nicData
			networkName := fmt.Sprintf("net-%v", numNetworks)
			numNetworks++

			kNetwork := cnv.Network{
				Name: networkName,
			}
			kInterface := cnv.Interface{
				Name:  networkName,
				Model: Virtio,
			}

			// Set MAC address if available
			if macPath, found := r.schema.NIC.GetField("mac"); found {
				if mac, found := nic.GetString(macPath); found {
					if !hasUDN || settings.Settings.UdnSupportsMac {
						kInterface.MacAddress = mac
					}
				}
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

// mapInput handles input device mapping for the generic builder fallback.
func (r *Builder) mapInput(object *cnv.VirtualMachineSpec) {
	tablet := cnv.Input{
		Type: Tablet,
		Name: Tablet,
		Bus:  Virtio,
	}
	object.Template.Spec.Domain.Devices.Inputs = []cnv.Input{tablet}
}

// mapMemory handles memory configuration for the generic builder fallback.
func (r *Builder) mapMemory(vm *unstructured.Unstructured, object *cnv.VirtualMachineSpec) error {
	memoryMBPath, _ := r.schema.VM.GetField("memoryMB")
	memoryMB, found := vm.GetInt(memoryMBPath)
	if !found {
		memoryMB = 2048 // Default to 2GB
	}

	memoryUnitsPath, _ := r.schema.VM.GetField("memoryUnits")
	memoryUnits, _ := vm.GetString(memoryUnitsPath)

	var memoryBytes int64
	memoryBytes, err := getResourceCapacity(int64(memoryMB), memoryUnits)
	if err != nil {
		return err
	}

	reservation := resource.NewQuantity(memoryBytes, resource.BinarySI)
	object.Template.Spec.Domain.Memory = &cnv.Memory{Guest: reservation}
	return nil
}

// mapCPU handles CPU configuration for the generic builder fallback.
func (r *Builder) mapCPU(vm *unstructured.Unstructured, object *cnv.VirtualMachineSpec) {
	cpuCountPath, _ := r.schema.VM.GetField("cpuCount")
	cpuCount, found := vm.GetInt(cpuCountPath)
	if !found {
		cpuCount = 1
	}

	coresPerSocketPath, _ := r.schema.VM.GetField("coresPerSocket")
	coresPerSocket, found := vm.GetInt(coresPerSocketPath)
	if !found || coresPerSocket == 0 {
		coresPerSocket = 1
	}

	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: uint32(cpuCount / coresPerSocket),
		Cores:   uint32(coresPerSocket),
	}
}

// mapFirmware handles firmware configuration for the generic builder fallback.
func (r *Builder) mapFirmware(vm *unstructured.Unstructured, vmRef ref.Ref, object *cnv.VirtualMachineSpec) {
	var virtV2VFirmware string

	// Try to get firmware from VM
	firmwarePath, _ := r.schema.VM.GetField("firmware")
	firmware, found := vm.GetString(firmwarePath)

	if !found || firmware == "" {
		// Check migration status for firmware info
		for _, vmConf := range r.Migration.Status.VMs {
			if vmConf.ID == vmRef.ID {
				virtV2VFirmware = vmConf.Firmware
				break
			}
		}
		if virtV2VFirmware == "" {
			r.Log.Info("failed to match the vm firmware", "vmRef ID", vmRef.ID)
		}
	} else {
		virtV2VFirmware = firmware
	}

	// Get secure boot if available
	secureBootPath, _ := r.schema.VM.GetField("secureBoot")
	secureBoot, _ := vm.GetBool(secureBootPath)

	// Get UUID for firmware serial
	uuidPath, _ := r.schema.VM.GetField("uuid")
	uuid, _ := vm.GetString(uuidPath)

	firmwareObj := &cnv.Firmware{
		Serial: uuid,
	}

	switch virtV2VFirmware {
	case BIOS:
		firmwareObj.Bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	default:
		// For UEFI firmware
		firmwareObj.Bootloader = &cnv.Bootloader{
			EFI: &cnv.EFI{
				SecureBoot: &secureBoot,
			}}
		if secureBoot {
			object.Template.Spec.Domain.Features = &cnv.Features{
				SMM: &cnv.FeatureState{
					Enabled: &secureBoot,
				},
			}
		}
	}
	object.Template.Spec.Domain.Firmware = firmwareObj
}

// mapDisks handles disk mapping for the generic builder fallback.
func (r *Builder) mapDisks(vm *unstructured.Unstructured, persistentVolumeClaims []*core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	var kVolumes []cnv.Volume
	var kDisks []cnv.Disk

	// Get disks from VM
	disksPath, _ := r.schema.VM.GetField("disks")
	disksData, found := vm.GetSlice(disksPath)
	if !found {
		return
	}

	// Build PVC map
	pvcMap := make(map[string]*core.PersistentVolumeClaim)
	for i := range persistentVolumeClaims {
		pvc := persistentVolumeClaims[i]
		if source, ok := pvc.Annotations[planbase.AnnDiskSource]; ok {
			pvcMap[source] = pvc
		}
	}

	for i, diskData := range disksData {
		disk := &unstructured.Unstructured{}
		disk.Object = diskData.(map[string]interface{})
		diskPath := getDiskFullPath(disk, r.schema)
		pvc := pvcMap[diskPath]

		volumeName := fmt.Sprintf("vol-%v", i)
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
					Bus: Virtio,
				},
			},
		}
		kVolumes = append(kVolumes, volume)
		kDisks = append(kDisks, kubevirtDisk)
	}

	object.Template.Spec.Volumes = kVolumes
	object.Template.Spec.Domain.Devices.Disks = kDisks
}

// Tasks builds migration tasks for progress tracking.
func (r *Builder) Tasks(vmRef ref.Ref) (list []*plan.Task, err error) {
	// Get VM using unstructured
	vm := &unstructured.Unstructured{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	// Get disks
	disksPath, _ := r.schema.VM.GetField("disks")
	disksData, found := vm.GetSlice(disksPath)
	if !found {
		return
	}

	for _, diskData := range disksData {
		disk := &unstructured.Unstructured{}
		disk.Object = diskData.(map[string]interface{})

		capacityPath, _ := r.schema.Disk.GetField("capacity")
		capacity, _ := disk.GetInt64(capacityPath)

		mB := capacity / 0x100000
		list = append(
			list,
			&plan.Task{
				Name: getDiskFullPath(disk, r.schema),
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
	// Dynamic providers don't use preferences yet
	err = liberr.New("preferences are not used by dynamic providers")
	return
}

func (r *Builder) ConfigMaps(vmRef ref.Ref) (list []core.ConfigMap, err error) {
	return nil, nil
}

func (r *Builder) Secrets(vmRef ref.Ref) (list []core.Secret, err error) {
	return nil, nil
}

func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	// Get VM using unstructured
	vm := &unstructured.Unstructured{}
	err = r.Source.Inventory.Find(vm, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return
	}

	os := Unknown

	labels = make(map[string]string)
	labels[fmt.Sprintf(TemplateOSLabel, os)] = "true"
	labels[TemplateWorkloadLabel] = "true"
	labels[TemplateFlavorLabel] = "true"

	return
}

func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return dv.ObjectMeta.Annotations[planbase.AnnDiskSource]
}

func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	return ""
}

// LunPersistentVolumes builds LUN PVs.
func (r *Builder) LunPersistentVolumes(vmRef ref.Ref) (pvs []core.PersistentVolume, err error) {
	return
}

// LunPersistentVolumeClaims builds LUN PVCs.
func (r *Builder) LunPersistentVolumeClaims(vmRef ref.Ref) (pvcs []core.PersistentVolumeClaim, err error) {
	return
}

func (r *Builder) SupportsVolumePopulators() bool {
	return false
}

func (r *Builder) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) (pvcs []*core.PersistentVolumeClaim, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) PrePopulateActions(c planbase.Client, vmRef ref.Ref) (ready bool, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) PopulatorTransferredBytes(persistentVolumeClaim *core.PersistentVolumeClaim) (transferredBytes int64, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) (err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

func (r *Builder) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error) {
	err = planbase.VolumePopulatorNotSupportedError
	return
}

// Helper functions

func getDiskFullPath(disk *unstructured.Unstructured, schema *ProviderSchema) string {
	filePathField, _ := schema.Disk.GetField("filePath")
	filePath, _ := disk.GetString(filePathField)

	nameField, _ := schema.Disk.GetField("name")
	name, _ := disk.GetString(nameField)

	return filePath + "::" + name
}

func getDiskSourcePath(filePath string) string {
	if strings.HasSuffix(filePath, ".ova") {
		return filePath
	}
	return strings.TrimSuffix(filePath, "/")
}

func getResourceCapacity(capacity int64, units string) (int64, error) {
	if units == "" {
		return capacity, nil
	}

	if strings.ToLower(units) == "megabytes" {
		return capacity * (1 << 20), nil
	}

	items := strings.Split(units, "*")
	for i := range items {
		item := strings.TrimSpace(items[i])
		if i == 0 && len(item) > 0 && item != "byte" {
			return 0, fmt.Errorf("units '%s' are invalid, only 'byte' is supported", units)
		}
		if i == 0 {
			continue
		}
		num, err := strconv.Atoi(item)
		if err == nil {
			capacity = capacity * int64(num)
			continue
		}
		nums := strings.Split(item, "^")
		if len(nums) != 2 {
			return 0, fmt.Errorf("units '%s' are invalid, item is invalid: %s", units, item)
		}
		base, err := strconv.Atoi(nums[0])
		if err != nil {
			return 0, fmt.Errorf("units '%s' are invalid, base component is invalid: %s", units, item)
		}
		pow, err := strconv.Atoi(nums[1])
		if err != nil {
			return 0, fmt.Errorf("units '%s' are invalid, pow component is invalid: %s", units, item)
		}
		capacity = capacity * int64(math.Pow(float64(base), float64(pow)))
	}
	return capacity, nil
}
