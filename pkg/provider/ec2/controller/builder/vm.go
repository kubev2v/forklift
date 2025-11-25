package builder

import (
	"fmt"
	"strings"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	ec2controller "github.com/kubev2v/forklift/pkg/provider/ec2/controller"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	cnv "kubevirt.io/api/core/v1"
)

// VirtualMachine builds a KubeVirt VirtualMachine spec from an EC2 instance.
// Maps EC2 instance type to CPU/memory, attaches PVCs as disks, and configures pod networking.
// The VM is created with RunStrategy=Halted and virtio disk bus.
func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) error {
	instance := &unstructured.Unstructured{}
	instance.SetUnstructuredContent(map[string]interface{}{"kind": "Instance"})
	err := r.Source.Inventory.Find(instance, vmRef)
	if err != nil {
		return err
	}

	awsInstance, err := ec2controller.GetAWSObject(instance)
	if err != nil {
		return err
	}

	uid, _, _ := unstructured.NestedString(awsInstance, "InstanceId")
	if uid == "" {
		uid, _, _ = unstructured.NestedString(awsInstance, "InstanceId")
	}
	name, _, _ := unstructured.NestedString(awsInstance, "name")
	if name == "" {
		name = uid
	}

	instanceType, found, _ := unstructured.NestedString(awsInstance, "InstanceType")
	if !found {
		instanceType, found, _ = unstructured.NestedString(awsInstance, "InstanceType")
	}
	if !found || instanceType == "" {
		instanceType = "m5.large"
		r.log.Info("InstanceType not found, using default", "vm", name, "default", instanceType)
	}

	vcpus, memoryMiB := r.mapInstanceType(instanceType)

	runStrategy := cnv.RunStrategyHalted
	object.RunStrategy = &runStrategy
	object.Template = &cnv.VirtualMachineInstanceTemplateSpec{
		Spec: cnv.VirtualMachineInstanceSpec{
			Domain: cnv.DomainSpec{
				CPU: &cnv.CPU{
					Cores: uint32(vcpus),
				},
				Resources: cnv.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", memoryMiB)),
					},
				},
				Devices: cnv.Devices{
					Disks:      []cnv.Disk{},
					Interfaces: []cnv.Interface{},
				},
				Features: &cnv.Features{
					ACPI: cnv.FeatureState{},
					SMM:  &cnv.FeatureState{Enabled: ptrBool(true)},
				},
			},
			Networks: []cnv.Network{},
			Volumes:  []cnv.Volume{},
		},
	}

	blockDevices, found, _ := unstructured.NestedSlice(awsInstance, "BlockDeviceMappings")

	pvcByVolumeID := make(map[string]*core.PersistentVolumeClaim)
	for _, pvc := range persistentVolumeClaims {
		if volumeID, ok := pvc.Labels["forklift.konveyor.io/volume-id"]; ok {
			pvcByVolumeID[volumeID] = pvc
		}
	}

	if found {
		for i, devIface := range blockDevices {
			dev, ok := devIface.(map[string]interface{})
			if !ok {
				continue
			}

			volumeID, _, _ := unstructured.NestedString(dev, "Ebs", "VolumeId")

			pvc, pvcFound := pvcByVolumeID[volumeID]
			if !pvcFound {
				r.log.Info("No PVC found for volume, skipping", "vm", name, "volumeID", volumeID)
				continue
			}

			diskName := fmt.Sprintf("disk%d", i)

			object.Template.Spec.Domain.Devices.Disks = append(
				object.Template.Spec.Domain.Devices.Disks,
				cnv.Disk{
					Name: diskName,
					DiskDevice: cnv.DiskDevice{
						Disk: &cnv.DiskTarget{
							Bus: "virtio",
						},
					},
				},
			)

			object.Template.Spec.Volumes = append(
				object.Template.Spec.Volumes,
				cnv.Volume{
					Name: diskName,
					VolumeSource: cnv.VolumeSource{
						PersistentVolumeClaim: &cnv.PersistentVolumeClaimVolumeSource{
							PersistentVolumeClaimVolumeSource: core.PersistentVolumeClaimVolumeSource{
								ClaimName: pvc.Name,
							},
						},
					},
				},
			)
		}
	}

	object.Template.Spec.Networks = append(
		object.Template.Spec.Networks,
		cnv.Network{
			Name: "default",
			NetworkSource: cnv.NetworkSource{
				Pod: &cnv.PodNetwork{},
			},
		},
	)

	object.Template.Spec.Domain.Devices.Interfaces = []cnv.Interface{
		{
			Name: "default",
			InterfaceBindingMethod: cnv.InterfaceBindingMethod{
				Masquerade: &cnv.InterfaceMasquerade{},
			},
		},
	}

	return nil
}

// mapInstanceType extracts the size suffix from an EC2 instance type and returns corresponding resources.
// Parses "family.size" format (e.g., "m5.large") and looks up vCPU/memory from instanceSizeSpecs.
// Defaults to 2 vCPUs and 4096 MiB if parsing fails.
func (r *Builder) mapInstanceType(instanceType string) (vcpus int32, memoryMiB int64) {
	vcpus = 2
	memoryMiB = 4096

	if len(instanceType) > 0 {
		parts := strings.Split(instanceType, ".")
		if len(parts) > 1 {
			size := parts[len(parts)-1]
			vcpus, memoryMiB = r.mapInstanceSize(size)
		}
	}

	r.log.V(1).Info("Mapped instance type", "type", instanceType, "vcpus", vcpus, "memoryMiB", memoryMiB)
	return
}

// instanceSizeSpec defines the resource allocation (CPU and memory) for an EC2 instance size.
//
// EC2 instance types follow a naming pattern: family.size (e.g., m5.large, t3.xlarge).
// This struct stores the vCPU count and memory allocation for each size suffix.
type instanceSizeSpec struct {
	// vcpus is the number of virtual CPUs allocated to this instance size.
	// Maps to the KubeVirt VirtualMachine's CPU cores specification.
	vcpus int32

	// memoryMiB is the amount of memory in mebibytes (MiB) allocated to this instance size.
	// Maps to the KubeVirt VirtualMachine's memory request specification.
	memoryMiB int64
}

// instanceSizeSpecs maps EC2 instance size suffixes to CPU/memory for KubeVirt resource requests.
// Instance types: <family>.<size> (e.g., t3.medium, m5.xlarge). Memory in MiB (1024-based).
var instanceSizeSpecs = map[string]instanceSizeSpec{
	"nano":     {1, 512},      // 1 vCPU, 512 MiB (0.5 GiB) - minimal instances
	"micro":    {1, 1024},     // 1 vCPU, 1 GiB - t2.micro, t3.micro
	"small":    {1, 2048},     // 1 vCPU, 2 GiB - t2.small, t3.small
	"medium":   {2, 4096},     // 2 vCPU, 4 GiB - t3.medium, m5.medium
	"large":    {2, 8192},     // 2 vCPU, 8 GiB - t3.large, m5.large
	"xlarge":   {4, 16384},    // 4 vCPU, 16 GiB - m5.xlarge, c5.xlarge
	"2xlarge":  {8, 32768},    // 8 vCPU, 32 GiB - m5.2xlarge
	"4xlarge":  {16, 65536},   // 16 vCPU, 64 GiB - m5.4xlarge
	"8xlarge":  {32, 131072},  // 32 vCPU, 128 GiB - m5.8xlarge
	"12xlarge": {48, 196608},  // 48 vCPU, 192 GiB - m5.12xlarge
	"16xlarge": {64, 262144},  // 64 vCPU, 256 GiB - m5.16xlarge
	"24xlarge": {96, 393216},  // 96 vCPU, 384 GiB - m5.24xlarge
	"32xlarge": {128, 524288}, // 128 vCPU, 512 GiB - largest instances like m5.32xlarge
}

// mapInstanceSize looks up CPU and memory allocation for an EC2 instance size suffix.
// Returns values from instanceSizeSpecs map or defaults to 2 vCPUs/4096 MiB for unknown sizes.
func (r *Builder) mapInstanceSize(size string) (vcpus int32, memoryMiB int64) {
	if spec, ok := instanceSizeSpecs[size]; ok {
		return spec.vcpus, spec.memoryMiB
	}

	r.log.Info("Unknown instance size, using default", "size", size)
	return 2, 4096
}

// TemplateLabels returns an error as KubeVirt templates are not used for EC2 migrations.
// EC2 instances are mapped directly to VirtualMachine specs without templates.
func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	err = liberr.New("templates are not used by this provider")
	return
}
