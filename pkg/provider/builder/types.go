package builder

// VMBuildValues contains all values extracted from the source VM,
// resolved with runtime state (PVCs, network mappings), ready for template rendering.
// Designed as a provider-agnostic superset; each provider populates the fields it supports.
type VMBuildValues struct {
	// --- Identity: Source ---

	// Name is the source VM name from the provider.
	// Also used as the display name annotation (AnnDisplayName) when the VM is renamed.
	// Providers: vSphere (model.VM.Name), oVirt (model.VM.Name),
	//   OpenStack (model.VM.Name), OVA (model.VM.Name), EC2 (instance Name tag)
	Name string

	// ID is the unique identifier of the source VM.
	// Unified field -- each provider populates it with its native identifier:
	//   vSphere: vm.UUID
	//   oVirt: vm.ID
	//   OpenStack: vm.ID
	//   OVA: vm.UUID
	//   EC2: InstanceId (e.g. "i-0abc123")
	// Also used as the tracking annotation (AnnOriginalID) when the VM is renamed.
	ID string

	// InstanceType is the cloud instance type string (provider metadata, informational).
	// Useful in templates for display annotations or conditional logic.
	// Providers: EC2 (e.g. "m5.large")
	// Not applicable: vSphere, oVirt, OpenStack, OVA (empty string)
	InstanceType string

	// OSType is the guest operating system identifier.
	// Unified field -- each provider populates it with its native OS identifier:
	//   vSphere: vm.GuestID (e.g. "rhel8_64Guest")
	//   oVirt: vm.OSType (e.g. "rhel_8x64", "windows_2022")
	//   OpenStack: from image os_type / os_distro
	//   OVA: vm.OsType
	//   EC2: detected from PlatformDetails (e.g. "rhel8.1", "win10")
	OSType string

	// --- Identity: Target ---

	// TargetName is the DNS1123-safe name for the target VM.
	// If the source name is DNS1123-incompatible, this is the adjusted name.
	// Populated by: orchestrator (kubevirt.go getNewVMName), all providers.
	TargetName string

	// TargetNamespace is the Kubernetes namespace where the target VM will be created.
	// Populated by: Plan.Spec.TargetNamespace, all providers.
	TargetNamespace string

	// NOTE: IsRenamed is NOT in this struct. The orchestrator handles rename annotations
	// (AnnDisplayName=Name, AnnOriginalID=ID) outside the template.

	// --- Lifecycle ---

	// RunStrategy is the resolved KubeVirt run strategy for the target VM.
	// Determined from the plan's TargetPowerState setting and the source VM's power state:
	//   - TargetPowerState "on"  → "Always"
	//   - TargetPowerState "off" → "Halted"
	//   - TargetPowerState "auto" or unset → matches source VM power state
	// Providers: all (resolved during extraction from plan config + source power state)
	RunStrategy string

	// --- Compute: CPU ---

	// Sockets is the number of CPU sockets.
	// Providers: vSphere (CpuCount / CoresPerSocket), oVirt (CpuSockets),
	//   OpenStack (hw_cpu_sockets or Flavor.VCPUs), OVA (CpuCount / CoresPerSocket),
	//   EC2 (always 1)
	Sockets uint32

	// Cores is the number of cores per socket.
	// Providers: vSphere (CoresPerSocket), oVirt (CpuCores),
	//   OpenStack (hw_cpu_cores, default 1), OVA (CoresPerSocket),
	//   EC2 (derived from instance type size)
	Cores uint32

	// Threads is the number of threads per core.
	// Providers: oVirt (CpuThreads), OpenStack (hw_cpu_threads, default 1)
	// Not applicable: vSphere, OVA, EC2 (set to 0, meaning "not specified")
	Threads uint32

	// HasDedicatedCPU indicates whether the VM requires dedicated (pinned) CPU placement.
	// Providers: oVirt (CpuPinningPolicy == "dedicated"),
	//   OpenStack (hw_cpu_policy == "dedicated")
	// Not applicable: vSphere, OVA, EC2
	HasDedicatedCPU bool

	// IsolateEmulatorThread indicates the emulator thread should be isolated.
	// Providers: OpenStack (flavor hw:emulator_threads_policy == "isolate")
	// Not applicable: vSphere, oVirt, OVA, EC2
	IsolateEmulatorThread bool

	// CPUModel is the explicit CPU model name (e.g. "Skylake-Server").
	// Providers: oVirt (CustomCpuModel, or cluster CPU when PreserveClusterCPUModel)
	// Not applicable: vSphere, OpenStack, OVA, EC2
	CPUModel string

	// CPUFeatures lists CPU feature flags to require/disable.
	// Providers: vSphere (vmx/svm when NestedVirtEnabled),
	//   oVirt (parsed from CustomCpuModel or cluster CPU flags),
	//   EC2 (vmx/svm when NestedVirtEnabled i.e. .metal instances)
	// Not applicable: OpenStack, OVA
	CPUFeatures []CPUFeatureBuildValues

	// --- Compute: Memory ---

	// MemoryMiB is the memory allocation in mebibytes.
	// Providers: vSphere (MemoryMB), oVirt (Memory / 1048576),
	//   OpenStack (Flavor.RAM), OVA (MemoryMB),
	//   EC2 (derived from instance type size)
	MemoryMiB int64

	// --- Firmware ---

	// IsUEFI indicates UEFI firmware (vs BIOS).
	// Providers: vSphere (Firmware == "efi"), oVirt (BIOS in [Q35Ovmf, Q35SecureBoot]),
	//   OpenStack (hw_firmware_type == "uefi"), OVA (Firmware == "efi"),
	//   EC2 (BootMode in [uefi, uefi-preferred])
	IsUEFI bool

	// SecureBoot indicates whether secure boot is enabled on the source.
	// Providers: vSphere (vm.SecureBoot), OVA (vm.SecureBoot)
	// oVirt: always set to false for migration (even if Q35SecureBoot)
	// OpenStack: always false for migration
	// EC2: always false (not exposed by AWS API)
	SecureBoot bool

	// Serial is the system serial number for the VM firmware.
	// Providers: vSphere (UUID or VMware-format serial), oVirt (SerialNumber or ID),
	//   EC2 (InstanceId)
	// Not set: OpenStack, OVA
	Serial string

	// FirmwareUUID is the firmware UUID (distinct from serial).
	// Providers: oVirt (vm.ID as types.UID)
	// Not applicable: vSphere, OpenStack, OVA, EC2
	FirmwareUUID string

	// --- Clock ---

	// Timezone is the VM/host timezone for the guest clock.
	// Providers: vSphere (Host.Timezone), oVirt (vm.Timezone)
	// Not applicable: OpenStack, OVA, EC2
	Timezone string

	// --- TPM ---

	// TPMEnabled indicates a virtual TPM should be attached (persistent).
	// Providers: vSphere (vm.TpmEnabled), oVirt (derived: windows_2022 or windows_11)
	// Not applicable: OpenStack, OVA, EC2
	TPMEnabled bool

	// TPMExplicitDisable indicates TPM should be explicitly disabled.
	// vSphere sets TPM.Enabled=false when TpmEnabled is false.
	// Other providers simply omit TPM when not needed.
	TPMExplicitDisable bool

	// --- Devices: Input ---

	// InputBus is the bus type for the tablet input device.
	// Empty string means no tablet input (e.g., OpenStack when hw_pointer_model is absent).
	// Providers: vSphere ("virtio" or "usb" in compat mode),
	//   oVirt (always "virtio"), OpenStack ("usb" when hw_pointer_model present, "" otherwise),
	//   OVA (always "virtio"), EC2 ("virtio" or "usb" in compat mode)
	// Template usage: {{ if .InputBus }} to conditionally add tablet.
	InputBus string

	// --- Devices: Video ---

	// AutoattachGraphicsDevice controls whether a graphics device is auto-attached.
	// Providers: OpenStack (hw_video_model; false when "none")
	// Not applicable: vSphere, oVirt, OVA, EC2 (default KubeVirt behavior)
	AutoattachGraphicsDevice *bool

	// --- Devices: RNG ---

	// HasRNG indicates a hardware random number generator should be attached.
	// Providers: OpenStack (hw_rng_model + flavor hw_rng:allowed)
	// Not applicable: vSphere, oVirt, OVA, EC2
	HasRNG bool

	// --- Devices: Disks ---

	// Disks contains the resolved disk list (PVC names matched to source volumes).
	// Providers: all
	Disks []DiskBuildValues

	// --- Devices: Networks ---

	// Networks contains the resolved network list (mappings applied, MAC addresses).
	// Providers: all
	Networks []NetworkBuildValues

	// --- Features ---

	// HasACPI indicates ACPI feature should be enabled.
	// Providers: EC2 (always true for both BIOS and UEFI)
	// vSphere, oVirt, OpenStack, OVA: not explicitly set (KubeVirt default)
	HasACPI bool

	// HasSMM indicates SMM (System Management Mode) feature should be enabled.
	// Required for UEFI secure boot.
	// Providers: vSphere (when SecureBoot), EC2 (when UEFI),
	//   OVA (when SecureBoot)
	// oVirt, OpenStack: not set
	HasSMM bool

	// --- Scheduling ---

	// NodeSelector is a map of label key=value for node scheduling.
	// Provider-specific node selectors resolved during extraction:
	//   EC2: topology.kubernetes.io/zone from target-az
	// Not applicable: vSphere, oVirt, OpenStack, OVA
	// NOTE: Plan-level TargetNodeSelector/TargetAffinity/TargetLabels are handled
	// by the orchestrator after template rendering, not in the values struct.
	NodeSelector map[string]string

	// --- Annotations ---

	// Annotations are extra annotations to add to the VM template metadata.
	// Provider-specific annotations resolved during extraction
	// (e.g., vSphere AnnStaticUdnIp for static IPs -- resolved based on PreserveStaticIPs flag).
	// NOTE: Orchestrator annotations (workload, rename) are handled outside the template.
	Annotations map[string]string

	// --- Labels ---

	// Labels are extra labels to add to the VM template metadata.
	// Provider-specific labels (e.g., from TemplateLabels for OS/workload/flavor matching).
	// NOTE: Plan-level labels (TargetLabels, guestConverted, app) are handled by the orchestrator.
	Labels map[string]string

	// NOTE: The following flags are NOT in this struct -- they are consumed elsewhere:
	//   CompatibilityMode  → resolved during extraction into per-device bus types
	//   UsesInstanceType   → drives template selection (variant without CPU/memory)
	//   IsRenamed          → orchestrator handles rename annotations
	//   PlanLabels/PlanNodeSelector/PlanAffinity → orchestrator post-processing
	//   GuestConverted     → orchestrator label
	//   UseVMwareSerialFormat → resolved during extraction into Serial value
	//   PreserveStaticIPs  → resolved during extraction into NetworkBuildValues.StaticIPs
	//
	// NOTE: RunStrategy IS in this struct -- resolved during extraction from
	//   TargetPowerState + source VM power state so the template can render it.
	//   The orchestrator may still override it as a safety net.

	// --- Provider-Specific: Common ---

	// NestedVirtEnabled indicates the source VM had nested virtualization enabled.
	// When true, CPUFeatures should include vmx/svm with "optional" policy.
	// Providers: vSphere (vm.NestedHVEnabled), EC2 (instance type ends with ".metal")
	// Not applicable: oVirt, OpenStack, OVA
	NestedVirtEnabled bool

	// --- Provider-Specific: vSphere ---

	// DiskEnableUuid controls whether disk serial numbers are set (SCSI only).
	// Providers: vSphere (vm.DiskEnableUuid)
	// NOTE: UseVMwareSerialFormat and PreserveStaticIPs are NOT here --
	// they are consumed during extraction to produce resolved Serial and StaticIPs values.
	DiskEnableUuid bool

	// --- Provider-Specific: oVirt ---

	// HasUsbEnabled indicates USB controller should be attached.
	// Providers: oVirt (vm.UsbEnabled)
	HasUsbEnabled bool

	// --- Provider-Specific: OpenStack ---

	// VifMultiQueueEnabled enables multi-queue for network interfaces.
	// Providers: OpenStack (hw_vif_multiqueue_enabled)
	VifMultiQueueEnabled bool

	// --- Recommended KubeVirt Resources ---
	// These fields are pre-resolved by each provider using provider-specific matching logic.
	// Template authors can use them (e.g., to set spec.instancetype or spec.preference)
	// or ignore them and hardcode resources directly.

	// RecommendedInstanceType is the best-matching KubeVirt VirtualMachineInstancetype
	// for the source VM's compute profile (CPU, memory).
	// Each provider implements its own matching strategy.
	// Empty if no suitable instancetype is found.
	RecommendedInstanceType RecommendedResource

	// RecommendedPreference is the best-matching KubeVirt VirtualMachinePreference
	// for the source VM's OS and device profile.
	// Each provider implements its own matching strategy.
	// Empty if no suitable preference is found.
	RecommendedPreference RecommendedResource

	// RecommendedTemplate is the best-matching OpenShift Template for the source VM's
	// OS, workload type, and flavor (compute size).
	// Empty if no suitable template is found (e.g., not running on OpenShift).
	RecommendedTemplate RecommendedResource
}

// RecommendedResource represents a pre-resolved KubeVirt resource recommendation.
// The provider populates this during value extraction; the template can reference it.
type RecommendedResource struct {
	// Name is the resource name (e.g., "u1.medium", "rhel.9", "rhel8-server-medium").
	// Empty string means no recommendation was found.
	Name string

	// Kind distinguishes namespaced vs cluster-scoped resources.
	// For instancetype: "VirtualMachineInstancetype" or "VirtualMachineClusterInstancetype".
	// For preference: "VirtualMachinePreference" or "VirtualMachineClusterPreference".
	// For template: always "Template".
	// Empty when Name is empty.
	Kind string
}

// CPUFeatureBuildValues represents a CPU feature flag.
type CPUFeatureBuildValues struct {
	// Name is the CPU feature name (e.g. "vmx", "svm", "avx2").
	Name string
	// Policy is the feature policy: "require", "optional", "disable", or "" (default).
	// Providers: vSphere ("optional" for vmx/svm),
	//   oVirt ("require" for +flag, "disable" for -flag),
	//   EC2 ("optional" for vmx/svm on metal)
	Policy string
}

// DiskBuildValues contains resolved disk information for template rendering.
type DiskBuildValues struct {
	// Name is the volume/disk name in the KubeVirt VM (e.g. "disk-0", "vol-0").
	// Providers: all (EC2: "disk-N", vSphere: "vol-N" or from template,
	//   oVirt: disk.ID, OpenStack: "vol-{diskSourceID}", OVA: "vol-N")
	Name string

	// PVCName is the name of the PersistentVolumeClaim backing this disk.
	// Providers: all (resolved by matching source disk ID to PVC labels/annotations)
	PVCName string

	// Bus is the disk bus type.
	// Providers: vSphere (from source disk.Bus; virtio/sata/scsi per bus type + compat mode),
	//   oVirt (from DiskAttachment.Interface: virtio_scsi→scsi, sata, ide→sata, else virtio),
	//   OpenStack (from hw_disk_bus; ide→sata, default virtio),
	//   OVA (always virtio), EC2 (virtio or sata in compat mode)
	Bus string

	// IsBootDisk indicates this disk should have BootOrder=1.
	// Providers: all (vSphere: from RootDisk plan field, oVirt: DiskAttachment.Bootable,
	//   OpenStack: bootable volume/image, OVA: first disk, EC2: first disk)
	IsBootDisk bool

	// IsShared indicates the disk is shared between multiple VMs.
	// Providers: vSphere (disk.Shared → Shareable=true, Cache=None)
	// Not applicable: oVirt, OpenStack, OVA, EC2
	IsShared bool

	// Serial is the disk serial number.
	// Providers: vSphere (disk.Serial, only when DiskEnableUuid && bus==SCSI)
	// Not applicable: oVirt, OpenStack, OVA, EC2
	Serial string

	// IsCDROM indicates this volume is a CD-ROM (not a disk).
	// Providers: OpenStack (image DiskFormat == "iso")
	// Not applicable: vSphere, oVirt, OVA, EC2
	IsCDROM bool

	// IsLUN indicates the disk is a LUN (raw block device, not copied).
	// Providers: oVirt (DiskAttachment.Disk.ActualSize == 0 for FC/iSCSI LUNs)
	// Not applicable: vSphere, OpenStack, OVA, EC2
	IsLUN bool

	// LUNBus is the bus type for LUN disks (can differ from regular disk bus).
	// Providers: oVirt
	LUNBus string

	// BootOrder is the explicit boot order value (0 means not set, 1 = primary).
	// Providers: all (typically 1 for boot disk, 0 for others)
	BootOrder uint
}

// NetworkBuildValues contains resolved network information for template rendering.
type NetworkBuildValues struct {
	// Name is the network interface name in the KubeVirt VM (e.g. "net-0").
	// Providers: all
	Name string

	// Type is the destination network type: "pod" or "multus".
	// Providers: all (resolved from NetworkMap)
	Type string

	// MultusName is the full "namespace/name" reference for Multus networks.
	// Providers: all (empty when Type=="pod")
	MultusName string

	// MACAddress is the MAC address to preserve from the source NIC.
	// Providers: vSphere (nic.MAC), oVirt (nic.MAC), OpenStack (from Addresses),
	//   OVA (nic.MAC), EC2 (eni.MacAddress)
	// Note: may be cleared when HasUDN && !UdnSupportsMac
	MACAddress string

	// Model is the NIC model/driver type.
	// Providers: vSphere ("virtio" or "e1000e" in compat mode),
	//   oVirt (from nic.Interface, e.g. "virtio", "e1000", "rtl8139"),
	//   OpenStack (from hw_vif_model, default "virtio"),
	//   OVA ("virtio"), EC2 ("virtio" or "e1000e" in compat mode)
	Model string

	// BindingMethod is the network binding method: "masquerade", "bridge", "sriov", "l2bridge".
	// Providers: all (derived from network type + UDN status + passthrough)
	BindingMethod string

	// IsSRIOV indicates the interface uses SR-IOV passthrough.
	// Providers: oVirt (nic.Profile.PassThrough)
	// Not applicable: vSphere, OpenStack, OVA, EC2
	IsSRIOV bool

	// HasUDN indicates the destination namespace has User Defined Networks.
	// Providers: all (cluster-level detection)
	HasUDN bool

	// IsUDNPod indicates this is a pod network in a UDN namespace (uses l2bridge binding).
	// Providers: all (when HasUDN && Type=="pod")
	IsUDNPod bool

	// StaticIPs contains static IP addresses to preserve (for UDN static IP support).
	// Providers: vSphere (from GuestNetworks when PreserveStaticIPs && UDN)
	// Not applicable: oVirt, OpenStack, OVA, EC2
	StaticIPs []string
}
