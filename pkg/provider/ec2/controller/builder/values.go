package builder

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

// extractValues reads the source EC2 instance, resolves PVCs and network mappings,
// and produces a populated VMBuildValues struct ready for template rendering.
// This replaces the scattered logic previously in mapCPU, mapFirmware, mapInput,
// mapDisks, mapNetworks, and mapNodeSelector.
func (r *Builder) extractValues(vmRef ref.Ref, persistentVolumeClaims []*core.PersistentVolumeClaim) (*builder.VMBuildValues, error) {
	awsInstance, err := inventory.GetAWSInstance(r.Source.Inventory, vmRef)
	if err != nil {
		return nil, err
	}

	values := &builder.VMBuildValues{}

	// --- Identity ---
	r.extractIdentity(awsInstance, values)

	// --- Compute ---
	r.extractCompute(awsInstance, values)

	// --- Firmware ---
	r.extractFirmware(awsInstance, values)

	// --- Input ---
	r.extractInput(values)

	// --- Disks ---
	r.extractDisks(awsInstance, persistentVolumeClaims, values)

	// --- Networks ---
	if err := r.extractNetworks(awsInstance, values); err != nil {
		return nil, err
	}

	// --- Scheduling ---
	r.extractNodeSelector(awsInstance, values)

	// --- Features ---
	r.extractFeatures(values)

	// --- Lifecycle ---
	r.extractRunStrategy(vmRef, values)

	// --- Recommendations ---
	r.extractRecommendations(awsInstance, values)

	return values, nil
}

// extractIdentity populates the identity fields from the EC2 instance.
func (r *Builder) extractIdentity(awsInstance *model.InstanceDetails, values *builder.VMBuildValues) {
	values.Name = inventory.GetInstanceName(awsInstance)
	values.ID = inventory.GetInstanceID(awsInstance)

	instanceType := string(awsInstance.InstanceType)
	if instanceType == "" {
		instanceType = "m5.large"
		r.log.Info("InstanceType not found, using default", "vm", values.Name, "default", instanceType)
	}
	values.InstanceType = instanceType

	// Detect OS from platform details
	values.OSType = r.detectOS(awsInstance)

	// Target identity is set later by the orchestrator (TargetName, TargetNamespace)
	values.TargetNamespace = r.Plan.Spec.TargetNamespace
}

// extractCompute populates CPU and memory fields from the EC2 instance type.
func (r *Builder) extractCompute(awsInstance *model.InstanceDetails, values *builder.VMBuildValues) {
	vcpus, memoryMiB := r.mapInstanceType(values.InstanceType)

	// EC2 always uses sockets=1, all cores in one socket
	values.Sockets = 1
	values.Cores = uint32(vcpus)
	values.MemoryMiB = memoryMiB

	// Nested virtualization for bare metal instances
	if r.isMetalInstance(values.InstanceType) {
		values.NestedVirtEnabled = true
		values.CPUFeatures = []builder.CPUFeatureBuildValues{
			{Name: "vmx", Policy: "optional"},
			{Name: "svm", Policy: "optional"},
		}
		r.log.Info("Enabled nested virtualization for metal instance", "instanceType", values.InstanceType)
	}
}

// extractFirmware populates firmware-related fields from the EC2 instance.
func (r *Builder) extractFirmware(awsInstance *model.InstanceDetails, values *builder.VMBuildValues) {
	// Serial number from instance ID
	if awsInstance.InstanceId != nil {
		values.Serial = *awsInstance.InstanceId
	}

	// Determine firmware type from EC2 BootMode
	bootMode := awsInstance.BootMode
	if bootMode == ec2types.BootModeValuesUefi || bootMode == ec2types.BootModeValuesUefiPreferred {
		values.IsUEFI = true
		values.HasSMM = true // SMM is required for UEFI
	}
	// SecureBoot is always false for EC2 (not exposed by AWS API)
	values.SecureBoot = false
}

// extractInput populates input device fields.
func (r *Builder) extractInput(values *builder.VMBuildValues) {
	if r.useCompatibilityMode() {
		values.InputBus = "usb"
	} else {
		values.InputBus = "virtio"
	}
}

// extractDisks resolves PVCs to source volumes and populates disk build values.
func (r *Builder) extractDisks(awsInstance *model.InstanceDetails, persistentVolumeClaims []*core.PersistentVolumeClaim, values *builder.VMBuildValues) {
	pvcByVolumeID := make(map[string]*core.PersistentVolumeClaim)
	for _, pvc := range persistentVolumeClaims {
		if volumeID, ok := pvc.Labels["forklift.konveyor.io/volume-id"]; ok {
			pvcByVolumeID[volumeID] = pvc
		}
	}

	// Select disk bus based on compatibility mode
	bus := "virtio"
	if r.useCompatibilityMode() {
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
			r.log.Info("No PVC found for volume, skipping", "vm", awsInstance.Name, "volumeID", volumeID)
			continue
		}

		diskName := fmt.Sprintf("disk-%d", diskIndex)
		disk := builder.DiskBuildValues{
			Name:    diskName,
			PVCName: pvc.Name,
			Bus:     bus,
		}

		// First disk is the boot device
		if diskIndex == 0 {
			disk.IsBootDisk = true
			disk.BootOrder = 1
		}

		values.Disks = append(values.Disks, disk)
		diskIndex++
	}
}

// extractNetworks resolves network mappings and populates network build values.
func (r *Builder) extractNetworks(awsInstance *model.InstanceDetails, values *builder.VMBuildValues) error {
	hasUDN := r.Plan.DestinationHasUdnNetwork(r.Destination)

	// Select NIC model based on compatibility mode
	nicModel := Virtio
	if r.useCompatibilityMode() {
		nicModel = E1000e
	}

	networkInterfaces := awsInstance.NetworkInterfaces
	if len(networkInterfaces) == 0 {
		// No network interfaces - add default pod network
		r.log.Info("No network interfaces found, using default pod network", "vm", awsInstance.Name)
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

	// Map each network interface using the network mapping
	netMapIn := r.Context.Map.Network.Spec.Map
	networkIndex := 0

	for _, eni := range networkInterfaces {
		// Find mapping for this subnet
		var mapped *api.NetworkPair
		if eni.SubnetId != nil {
			mapped = r.findNetworkMapping(*eni.SubnetId, netMapIn)
		}

		// Skip if destination type is Ignored
		if mapped != nil && mapped.Destination.Type == Ignored {
			continue
		}

		networkName := fmt.Sprintf("net-%d", networkIndex)
		net := builder.NetworkBuildValues{
			Name:   networkName,
			Model:  nicModel,
			HasUDN: hasUDN,
		}

		// Preserve MAC address from source ENI
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

// extractNodeSelector populates the node selector for AZ-based scheduling.
func (r *Builder) extractNodeSelector(awsInstance *model.InstanceDetails, values *builder.VMBuildValues) {
	if r.Plan.Spec.SkipZoneNodeSelector {
		return
	}

	az, err := r.getTargetAZ()
	if err != nil {
		r.log.Info("Could not get target AZ from provider settings, skipping zone node selector",
			"vm", awsInstance.Name,
			"error", err.Error())
		return
	}

	values.NodeSelector = map[string]string{
		TopologyZoneLabel: az,
	}
	r.log.Info("Set AZ-based node selector for VM",
		"vm", awsInstance.Name,
		"targetAZ", az,
		"label", TopologyZoneLabel)
}

// extractFeatures populates VM feature flags.
func (r *Builder) extractFeatures(values *builder.VMBuildValues) {
	// EC2 always enables ACPI for proper shutdown
	values.HasACPI = true
	// HasSMM is already set by extractFirmware when UEFI is detected
}

// extractRunStrategy determines and populates the run strategy for the target VM.
// Mirrors the logic in kubevirt.go's determineRunStrategy(), using the plan's
// TargetPowerState setting and the source VM's power state.
func (r *Builder) extractRunStrategy(vmRef ref.Ref, values *builder.VMBuildValues) {
	values.RunStrategy = string(r.determineRunStrategy(vmRef))
}

// determineRunStrategy determines the appropriate run strategy based on the target
// power state configuration and the source VM's power state.
// This mirrors the orchestrator's logic in kubevirt.go.
func (r *Builder) determineRunStrategy(vmRef ref.Ref) cnv.VirtualMachineRunStrategy {
	// Find the VM status to get per-VM target power state and source power state
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

	// Per-VM setting takes precedence, then plan-level setting
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
		// Default: match the source VM's power state
		if sourceVMPowerState == plan.VMPowerStateOn {
			return cnv.RunStrategyAlways
		}
		return cnv.RunStrategyHalted
	}
}

// extractRecommendations populates the RecommendedInstanceType, RecommendedPreference,
// and RecommendedTemplate fields by querying the cluster for best-matching resources.
// Errors are logged but not returned -- recommendations are best-effort.
func (r *Builder) extractRecommendations(awsInstance *model.InstanceDetails, values *builder.VMBuildValues) {
	ns := r.Plan.Spec.TargetNamespace

	// Recommended instance type: find closest by CPU and memory
	rec, err := builder.FindClosestInstanceType(r.Client, ns, values.Sockets*values.Cores, values.MemoryMiB)
	if err != nil {
		r.log.Info("Could not resolve recommended instance type", "vm", values.Name, "error", err)
	} else {
		values.RecommendedInstanceType = rec
	}

	// Recommended preference: EC2 doesn't have a pre-existing preference name,
	// so we derive one from the detected OS type using the os-info ID.
	osType := r.detectOS(awsInstance)
	if osType != "" {
		rec, err = builder.ResolvePreference(r.Client, ns, osType)
		if err != nil {
			r.log.Info("Could not resolve recommended preference", "vm", values.Name, "os", osType, "error", err)
		} else {
			values.RecommendedPreference = rec
		}
	}

	// Recommended OpenShift template: match by OS/workload/flavor labels
	templateLabels := r.buildTemplateLabels(osType)
	rec, err = builder.ResolveTemplate(r.Client, templateLabels)
	if err != nil {
		r.log.Info("Could not resolve recommended template", "vm", values.Name, "error", err)
	} else {
		values.RecommendedTemplate = rec
	}
}

// buildTemplateLabels returns OpenShift template matching labels for the EC2 instance.
func (r *Builder) buildTemplateLabels(osType string) map[string]string {
	os := osType
	if os == "" {
		os = DefaultLinux
	}

	labels := make(map[string]string)
	labels[fmt.Sprintf(TemplateOSLabel, os)] = "true"
	labels[TemplateWorkloadLabel] = "true"
	labels[TemplateFlavorLabel] = "true"

	return labels
}
