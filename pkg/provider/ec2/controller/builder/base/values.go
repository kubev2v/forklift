package base

import (
	"fmt"
	"path"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	builder "github.com/kubev2v/forklift/pkg/provider/builder"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/inventory"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	cnv "kubevirt.io/api/core/v1"
)

// ExtractValues reads the source EC2 instance, resolves PVCs and network mappings,
// and produces a populated VMBuildValues struct ready for template rendering.
func (r *Base) ExtractValues(vmRef ref.Ref, persistentVolumeClaims []*core.PersistentVolumeClaim) (*builder.VMBuildValues, error) {
	awsInstance, err := inventory.GetAWSInstance(r.Source.Inventory, vmRef)
	if err != nil {
		return nil, err
	}

	values := &builder.VMBuildValues{}

	r.extractIdentity(awsInstance, values)
	r.extractCompute(awsInstance, values)
	r.extractFirmware(awsInstance, values)
	r.extractInput(values)
	r.extractDisks(awsInstance, persistentVolumeClaims, values)
	if err := r.extractNetworks(awsInstance, values); err != nil {
		return nil, err
	}
	r.extractNodeSelector(awsInstance, values)
	r.extractFeatures(values)
	r.extractRunStrategy(vmRef, values)
	r.extractRecommendations(awsInstance, values)

	return values, nil
}

func (r *Base) extractIdentity(awsInstance *model.InstanceDetails, values *builder.VMBuildValues) {
	values.Name = inventory.GetInstanceName(awsInstance)
	values.ID = inventory.GetInstanceID(awsInstance)

	instanceType := string(awsInstance.InstanceType)
	if instanceType == "" {
		instanceType = "m5.large"
		r.Log.Info("InstanceType not found, using default", "vm", values.Name, "default", instanceType)
	}
	values.InstanceType = instanceType
	values.OSType = r.DetectOS(awsInstance)
	values.TargetNamespace = r.Plan.Spec.TargetNamespace
}

func (r *Base) extractCompute(awsInstance *model.InstanceDetails, values *builder.VMBuildValues) {
	vcpus, memoryMiB := r.MapInstanceType(values.InstanceType)

	values.Sockets = 1
	values.Cores = uint32(vcpus)
	values.MemoryMiB = memoryMiB

	if r.IsMetalInstance(values.InstanceType) {
		values.NestedVirtEnabled = true
		values.CPUFeatures = []builder.CPUFeatureBuildValues{
			{Name: "vmx", Policy: "optional"},
			{Name: "svm", Policy: "optional"},
		}
		r.Log.Info("Enabled nested virtualization for metal instance", "instanceType", values.InstanceType)
	}
}

func (r *Base) extractFirmware(awsInstance *model.InstanceDetails, values *builder.VMBuildValues) {
	if awsInstance.InstanceId != nil {
		values.Serial = *awsInstance.InstanceId
	}

	bootMode := awsInstance.BootMode
	if bootMode == ec2types.BootModeValuesUefi || bootMode == ec2types.BootModeValuesUefiPreferred {
		values.IsUEFI = true
		values.HasSMM = true
	}
	values.SecureBoot = false
}

func (r *Base) extractInput(values *builder.VMBuildValues) {
	if r.UseCompatibilityMode() {
		values.InputBus = "usb"
	} else {
		values.InputBus = "virtio"
	}
}

func (r *Base) extractDisks(awsInstance *model.InstanceDetails, persistentVolumeClaims []*core.PersistentVolumeClaim, values *builder.VMBuildValues) {
	pvcByVolumeID := make(map[string]*core.PersistentVolumeClaim)
	for _, pvc := range persistentVolumeClaims {
		if volumeID, ok := pvc.Labels["forklift.konveyor.io/volume-id"]; ok {
			pvcByVolumeID[volumeID] = pvc
		}
	}

	bus := "virtio"
	if r.UseCompatibilityMode() {
		bus = "sata"
	}

	diskIndex := 0
	for _, dev := range awsInstance.BlockDeviceMappings {
		if dev.Ebs == nil || dev.Ebs.VolumeId == nil {
			continue
		}

		volumeID := *dev.Ebs.VolumeId
		pvc, pvcFound := pvcByVolumeID[volumeID]
		if !pvcFound {
			r.Log.Info("No PVC found for volume, skipping", "vm", awsInstance.Name, "volumeID", volumeID)
			continue
		}

		diskName := fmt.Sprintf("disk-%d", diskIndex)
		disk := builder.DiskBuildValues{
			Name:    diskName,
			PVCName: pvc.Name,
			Bus:     bus,
		}

		if diskIndex == 0 {
			disk.IsBootDisk = true
			disk.BootOrder = 1
		}

		values.Disks = append(values.Disks, disk)
		diskIndex++
	}
}

func (r *Base) extractNetworks(awsInstance *model.InstanceDetails, values *builder.VMBuildValues) error {
	hasUDN := r.Plan.DestinationHasUdnNetwork(r.Destination)

	nicModel := Virtio
	if r.UseCompatibilityMode() {
		nicModel = E1000e
	}

	networkInterfaces := awsInstance.NetworkInterfaces
	if len(networkInterfaces) == 0 {
		r.Log.Info("No network interfaces found, using default pod network", "vm", awsInstance.Name)
		net := builder.NetworkBuildValues{
			Name:   "default",
			Type:   Pod,
			Model:  nicModel,
			HasUDN: hasUDN,
		}
		if hasUDN {
			net.BindingMethod = planbase.UdnL2bridge
			net.IsUDNPod = true
		} else {
			net.BindingMethod = "masquerade"
		}
		values.Networks = append(values.Networks, net)
		return nil
	}

	netMapIn := r.Context.Map.Network.Spec.Map
	networkIndex := 0

	for _, eni := range networkInterfaces {
		var mapped *api.NetworkPair
		if eni.SubnetId != nil {
			mapped = r.FindNetworkMapping(*eni.SubnetId, netMapIn)
		}

		if mapped != nil && mapped.Destination.Type == Ignored {
			continue
		}

		networkName := fmt.Sprintf("net-%d", networkIndex)
		net := builder.NetworkBuildValues{
			Name:   networkName,
			Model:  nicModel,
			HasUDN: hasUDN,
		}

		if eni.MacAddress != nil && *eni.MacAddress != "" {
			if !hasUDN || settings.Settings.UdnSupportsMac {
				net.MACAddress = *eni.MacAddress
			}
		}

		if mapped == nil || mapped.Destination.Type == Pod {
			net.Type = Pod
			if hasUDN {
				net.BindingMethod = planbase.UdnL2bridge
				net.IsUDNPod = true
			} else {
				net.BindingMethod = "masquerade"
			}
		} else if mapped.Destination.Type == Multus {
			net.Type = Multus
			net.MultusName = path.Join(mapped.Destination.Namespace, mapped.Destination.Name)
			net.BindingMethod = "bridge"
		}

		values.Networks = append(values.Networks, net)
		networkIndex++
	}

	return nil
}

func (r *Base) extractNodeSelector(awsInstance *model.InstanceDetails, values *builder.VMBuildValues) {
	if r.Plan.Spec.SkipZoneNodeSelector {
		return
	}

	az, err := r.GetTargetAZ()
	if err != nil {
		r.Log.Info("Could not get target AZ from provider settings, skipping zone node selector",
			"vm", awsInstance.Name,
			"error", err.Error())
		return
	}

	values.NodeSelector = map[string]string{
		TopologyZoneLabel: az,
	}
	r.Log.Info("Set AZ-based node selector for VM",
		"vm", awsInstance.Name,
		"targetAZ", az,
		"label", TopologyZoneLabel)
}

func (r *Base) extractFeatures(values *builder.VMBuildValues) {
	values.HasACPI = true
}

func (r *Base) extractRunStrategy(vmRef ref.Ref, values *builder.VMBuildValues) {
	values.RunStrategy = string(r.DetermineRunStrategy(vmRef))
}

// DetermineRunStrategy determines the appropriate run strategy based on the target
// power state configuration and the source VM's power state.
func (r *Base) DetermineRunStrategy(vmRef ref.Ref) cnv.VirtualMachineRunStrategy {
	var vmTargetPowerState plan.TargetPowerState
	var sourceVMPowerState plan.VMPowerState

	if r.Migration != nil {
		for _, vmStatus := range r.Migration.Status.VMs {
			if vmStatus.ID == vmRef.ID {
				vmTargetPowerState = vmStatus.TargetPowerState
				sourceVMPowerState = vmStatus.RestorePowerState
				break
			}
		}
	}

	targetPowerState := vmTargetPowerState
	if targetPowerState == "" {
		targetPowerState = r.Plan.Spec.TargetPowerState
	}

	switch targetPowerState {
	case plan.TargetPowerStateOn:
		return cnv.RunStrategyAlways
	case plan.TargetPowerStateOff:
		return cnv.RunStrategyHalted
	default:
		if sourceVMPowerState == plan.VMPowerStateOn {
			return cnv.RunStrategyAlways
		}
		return cnv.RunStrategyHalted
	}
}

func (r *Base) extractRecommendations(awsInstance *model.InstanceDetails, values *builder.VMBuildValues) {
	ns := r.Plan.Spec.TargetNamespace

	rec, err := builder.FindClosestInstanceType(r.Client, ns, values.Sockets*values.Cores, values.MemoryMiB)
	if err != nil {
		r.Log.Info("Could not resolve recommended instance type", "vm", values.Name, "error", err)
	} else {
		values.RecommendedInstanceType = rec
	}

	osType := r.DetectOS(awsInstance)
	if osType != "" {
		rec, err = builder.ResolvePreference(r.Client, ns, osType)
		if err != nil {
			r.Log.Info("Could not resolve recommended preference", "vm", values.Name, "os", osType, "error", err)
		} else {
			values.RecommendedPreference = rec
		}
	}

	templateLabels := r.BuildTemplateLabels(osType)
	rec, err = builder.ResolveTemplate(r.Client, templateLabels)
	if err != nil {
		r.Log.Info("Could not resolve recommended template", "vm", values.Name, "error", err)
	} else {
		values.RecommendedTemplate = rec
	}
}
