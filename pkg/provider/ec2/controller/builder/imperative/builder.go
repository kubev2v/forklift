package imperative

import (
	"fmt"
	"path"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	ec2base "github.com/kubev2v/forklift/pkg/provider/ec2/controller/builder/base"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/inventory"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	cnv "kubevirt.io/api/core/v1"
)

// Builder builds KubeVirt VirtualMachineSpec objects from EC2 instances using
// the imperative map* pattern, directly constructing typed KubeVirt API objects.
type Builder struct {
	base *ec2base.Base
}

// New creates an imperative Builder backed by the shared Base.
func New(b *ec2base.Base) *Builder {
	return &Builder{base: b}
}

// VirtualMachine builds a KubeVirt VirtualMachineSpec from an EC2 instance.
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, _ bool) error {
	awsInstance, err := inventory.GetAWSInstance(r.base.Source.Inventory, vmRef)
	if err != nil {
		return err
	}

	if object.Template == nil {
		object.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}

	r.mapRunStrategy(vmRef, object)
	r.mapFirmware(awsInstance, object)
	r.mapFeatures(awsInstance, object)
	if !usesInstanceType {
		r.mapCPU(awsInstance, object)
		r.mapMemory(awsInstance, object)
	}
	r.mapInput(object)
	r.mapDisks(awsInstance, persistentVolumeClaims, object)
	if err := r.mapNetworks(awsInstance, object); err != nil {
		return err
	}
	r.mapNodeSelector(object)

	return nil
}

func (r *Builder) mapRunStrategy(vmRef ref.Ref, object *cnv.VirtualMachineSpec) {
	strategy := r.base.DetermineRunStrategy(vmRef)
	object.RunStrategy = &strategy
}

func (r *Builder) mapCPU(awsInstance *model.InstanceDetails, object *cnv.VirtualMachineSpec) {
	instanceType := r.base.ResolveInstanceType(awsInstance)
	vcpus, _ := r.base.MapInstanceType(instanceType)

	cpu := &cnv.CPU{
		Sockets: 1,
		Cores:   uint32(vcpus),
	}
	if r.base.IsMetalInstance(instanceType) {
		cpu.Features = []cnv.CPUFeature{
			{Name: "vmx", Policy: "optional"},
			{Name: "svm", Policy: "optional"},
		}
	}
	object.Template.Spec.Domain.CPU = cpu
}

func (r *Builder) mapMemory(awsInstance *model.InstanceDetails, object *cnv.VirtualMachineSpec) {
	instanceType := r.base.ResolveInstanceType(awsInstance)
	_, memoryMiB := r.base.MapInstanceType(instanceType)

	memoryBytes := memoryMiB * 1024 * 1024
	object.Template.Spec.Domain.Memory = &cnv.Memory{
		Guest: resource.NewQuantity(memoryBytes, resource.BinarySI),
	}
}

func (r *Builder) mapFirmware(awsInstance *model.InstanceDetails, object *cnv.VirtualMachineSpec) {
	firmware := &cnv.Firmware{}
	if awsInstance.InstanceId != nil {
		firmware.Serial = *awsInstance.InstanceId
	}

	if r.base.IsUEFI(awsInstance) {
		secureBoot := false
		firmware.Bootloader = &cnv.Bootloader{
			EFI: &cnv.EFI{SecureBoot: &secureBoot},
		}
	} else {
		firmware.Bootloader = &cnv.Bootloader{BIOS: &cnv.BIOS{}}
	}
	object.Template.Spec.Domain.Firmware = firmware
}

func (r *Builder) mapFeatures(awsInstance *model.InstanceDetails, object *cnv.VirtualMachineSpec) {
	if r.base.IsUEFI(awsInstance) {
		object.Template.Spec.Domain.Features = &cnv.Features{
			SMM: &cnv.FeatureState{Enabled: ptr.To(true)},
		}
	}
}

func (r *Builder) mapInput(object *cnv.VirtualMachineSpec) {
	bus := cnv.InputBusVirtio
	if r.base.UseCompatibilityMode() {
		bus = cnv.InputBusUSB
	}
	object.Template.Spec.Domain.Devices.Inputs = []cnv.Input{
		{Type: ec2base.Tablet, Name: ec2base.Tablet, Bus: bus},
	}
}

func (r *Builder) mapDisks(awsInstance *model.InstanceDetails, persistentVolumeClaims []*core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	pvcByVolumeID := make(map[string]*core.PersistentVolumeClaim)
	for _, pvc := range persistentVolumeClaims {
		if volumeID, ok := pvc.Labels["forklift.konveyor.io/volume-id"]; ok {
			pvcByVolumeID[volumeID] = pvc
		}
	}

	bus := cnv.DiskBusVirtio
	if r.base.UseCompatibilityMode() {
		bus = cnv.DiskBusSATA
	}

	var kDisks []cnv.Disk
	var kVolumes []cnv.Volume
	diskIndex := 0

	for _, dev := range awsInstance.BlockDeviceMappings {
		if dev.Ebs == nil || dev.Ebs.VolumeId == nil {
			continue
		}
		volumeID := *dev.Ebs.VolumeId
		pvc, found := pvcByVolumeID[volumeID]
		if !found {
			r.base.Log.Info("No PVC found for volume, skipping", "volumeID", volumeID)
			continue
		}

		diskName := fmt.Sprintf("disk-%d", diskIndex)
		kDisk := cnv.Disk{
			Name: diskName,
			DiskDevice: cnv.DiskDevice{
				Disk: &cnv.DiskTarget{Bus: bus},
			},
		}
		if diskIndex == 0 {
			kDisk.BootOrder = ptr.To(uint(1))
		}

		kVolume := cnv.Volume{
			Name: diskName,
			VolumeSource: cnv.VolumeSource{
				PersistentVolumeClaim: &cnv.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: core.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			},
		}

		kDisks = append(kDisks, kDisk)
		kVolumes = append(kVolumes, kVolume)
		diskIndex++
	}

	object.Template.Spec.Domain.Devices.Disks = kDisks
	object.Template.Spec.Volumes = kVolumes
}

func (r *Builder) mapNetworks(awsInstance *model.InstanceDetails, object *cnv.VirtualMachineSpec) error {
	hasUDN := r.base.Plan.DestinationHasUdnNetwork(r.base.Destination)
	nicModel := ec2base.Virtio
	if r.base.UseCompatibilityMode() {
		nicModel = ec2base.E1000e
	}

	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

	if len(awsInstance.NetworkInterfaces) == 0 {
		kNetwork := cnv.Network{Name: "default"}
		kNetwork.Pod = &cnv.PodNetwork{}
		kInterface := cnv.Interface{Name: "default", Model: nicModel}
		if hasUDN {
			kInterface.Binding = &cnv.PluginBinding{Name: planbase.UdnL2bridge}
		} else {
			kInterface.Masquerade = &cnv.InterfaceMasquerade{}
		}
		kNetworks = append(kNetworks, kNetwork)
		kInterfaces = append(kInterfaces, kInterface)
	} else {
		networkIndex := 0
		for _, eni := range awsInstance.NetworkInterfaces {
			mapped := r.base.FindMappingForSubnet(eni.SubnetId)
			if mapped != nil && mapped.Destination.Type == ec2base.Ignored {
				continue
			}

			networkName := fmt.Sprintf("net-%d", networkIndex)
			kNetwork := cnv.Network{Name: networkName}
			kInterface := cnv.Interface{Name: networkName, Model: nicModel}

			if eni.MacAddress != nil && *eni.MacAddress != "" {
				if !hasUDN || settings.Settings.UdnSupportsMac {
					kInterface.MacAddress = *eni.MacAddress
				}
			}

			switch {
			case mapped == nil || mapped.Destination.Type == ec2base.Pod:
				kNetwork.Pod = &cnv.PodNetwork{}
				if hasUDN {
					kInterface.Binding = &cnv.PluginBinding{Name: planbase.UdnL2bridge}
				} else {
					kInterface.Masquerade = &cnv.InterfaceMasquerade{}
				}
			case mapped.Destination.Type == ec2base.Multus:
				kNetwork.Multus = &cnv.MultusNetwork{
					NetworkName: path.Join(mapped.Destination.Namespace, mapped.Destination.Name),
				}
				kInterface.Bridge = &cnv.InterfaceBridge{}
			}

			kNetworks = append(kNetworks, kNetwork)
			kInterfaces = append(kInterfaces, kInterface)
			networkIndex++
		}
	}

	object.Template.Spec.Networks = kNetworks
	object.Template.Spec.Domain.Devices.Interfaces = kInterfaces
	return nil
}

func (r *Builder) mapNodeSelector(object *cnv.VirtualMachineSpec) {
	if r.base.Plan.Spec.SkipZoneNodeSelector {
		return
	}
	az, err := r.base.GetTargetAZ()
	if err != nil {
		r.base.Log.Info("Could not get target AZ, skipping zone node selector", "error", err.Error())
		return
	}
	object.Template.Spec.NodeSelector = map[string]string{
		ec2base.TopologyZoneLabel: az,
	}
}
