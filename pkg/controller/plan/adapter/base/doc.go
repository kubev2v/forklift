package base

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Annotations
const (
	// JSON map of original → sanitized label/annotation keys on the destination VM.
	AnnSanitizedMetadata = "forklift.konveyor.io/sanitized-metadata"

	// Used on DataVolume, contains disk source -- e.g. backing file in
	// VMware or disk ID in oVirt.
	AnnDiskSource = "forklift.konveyor.io/disk-source"

	// Used on DataVolume, contains disk mount order.
	AnnDiskIndex = "forklift.konveyor.io/disk-index"

	// Set on a PVC to indicate it requires format conversion
	AnnRequiresConversion = "forklift.konveyor.io/requires-conversion"

	// Set the source format of the PVC for the conversion later
	AnnSourceFormat = "forklift.konveyor.io/source-format"

	// Set the source PVC of the conversion, used on the DV for filtering
	AnnConversionSourcePVC = "forklift.konveyor.io/conversionSourcePVC"

	// Copy method annotation indicates which method was used for volume migration
	AnnCopyMethod = "forklift.konveyor.io/copy-method"

	// Copy offload annotation contains the source disk identifier for offloaded copies
	AnnCopyOffload = "copy-offload"

	// Copy method values
	CopyMethodCsiImport = "csi-import"

	// CDI

	// Causes the importer pod to be retained after import.
	AnnRetainAfterCompletion = "cdi.kubevirt.io/storage.pod.retainAfterCompletion"

	// DV immediate bind to WaitForFirstConsumer storage class
	AnnBindImmediate = "cdi.kubevirt.io/storage.bind.immediate.requested"

	// Add extra vddk configmap, in the Forklift used to pass AIO configuration to the VDDK.
	// Related to https://github.com/kubevirt/containerized-data-importer/pull/3572
	AnnVddkExtraArgs = "cdi.kubevirt.io/storage.pod.vddk.extraargs"

	// CDI import backing file annotation on PVC
	AnnImportBackingFile = "cdi.kubevirt.io/storage.import.backingFile"

	// Source URL, on PVC
	AnnEndpoint = "cdi.kubevirt.io/storage.import.endpoint"

	// Secret name for source credentials, on PVC
	AnnSecret = "cdi.kubevirt.io/storage.import.secretName"

	// VM UUID, on PVC
	AnnUUID = "cdi.kubevirt.io/storage.import.uuid"

	// VDDK-specific thumbprint
	AnnThumbprint = "cdi.kubevirt.io/storage.import.vddk.thumbprint"

	// VDDK image, on PVC
	AnnVddkInitImageURL = "cdi.kubevirt.io/storage.pod.vddk.initimageurl"

	// Importer pod progress phase, on PVC
	AnnPodPhase = "cdi.kubevirt.io/storage.pod.phase"

	// True if the current checkpoint is the one taken for the cutover, on PVC
	AnnFinalCheckpoint = "cdi.kubevirt.io/storage.checkpoint.final"

	// Current checkpoint reference, on PVC
	AnnCurrentCheckpoint = "cdi.kubevirt.io/storage.checkpoint.current"

	// Previous checkpoint reference, on PVC
	AnnPreviousCheckpoint = "cdi.kubevirt.io/storage.checkpoint.previous"

	// Not a whole annotation but a prefix, append a snapshot name to mark that the snapshot was already copied (on PVC)
	AnnCheckpointsCopied = "cdi.kubevirt.io/storage.checkpoint.copied"

	// Allow DataVolume to adopt a PVC, on DataVolume
	AnnAllowClaimAdoption = "cdi.kubevirt.io/allowClaimAdoption"

	// Inform CDI that the DataVolume is already filled up, on DataVolume
	AnnPrePopulated = "cdi.kubevirt.io/storage.prePopulated"

	// Tell CDI which importer to use, on PVC
	AnnSource = "cdi.kubevirt.io/storage.import.source"

	// Name of the current importer pod, on PVC
	AnnImportPod = "cdi.kubevirt.io/storage.import.importPod"

	// In a UDN namespace we can't directly reach the virt-v2v pod unless we specify default opened ports on the pod network.
	AnnOpenDefaultPorts = "k8s.ovn.org/open-default-ports"

	// UDN L2 bridge binding, needed for KubeVirt VMs with UDN
	UdnL2bridge = "l2bridge"

	// Enhancement doc: https://github.com/openshift/enhancements/pull/1793
	// Example: network.kubevirt.io/addresses: '{"iface1": ["192.168.0.1/24", "fd23:3214::123/64"]}'
	AnnStaticUdnIp = "network.kubevirt.io/addresses"

	// Explicitly disable CDI's populator auto-detection to avoid webhook validation errors
	AnnUsePopulator = "cdi.kubevirt.io/storage.usePopulator"

	// Consumer-side disk metadata used by NetApp Shift/Trident integration.
	AnnNfsServer   = "forklift.konveyor.io/nfs-server"
	AnnNfsPath     = "forklift.konveyor.io/nfs-path"
	AnnVmId        = "forklift.konveyor.io/vm-id"
	AnnVmUUID      = "forklift.konveyor.io/vm-uuid"
	AnnNetAppShift = "forklift.konveyor.io/netapp-shift"
)

var VolumePopulatorNotSupportedError = liberr.New("provider does not support volume populators")

// ConversionPodConfigResult contains provider-specific configuration for the virt-v2v conversion pod.
// All fields are optional - nil means no provider-specific configuration for that aspect.
type ConversionPodConfigResult struct {
	// NodeSelector specifies provider-required node selection constraints.
	// These are merged with (but can be overridden by) Plan.Spec.ConvertorNodeSelector.
	NodeSelector map[string]string

	// Labels specifies provider-specific labels to add to the conversion pod.
	// These are merged with (but can be overridden by) Plan.Spec.ConvertorLabels.
	Labels map[string]string

	// Annotations specifies provider-specific annotations to add to the conversion pod.
	Annotations map[string]string
}

// Adapter API.
// Constructs provider-specific implementations
// of the Builder, Client, and Validator.
type Adapter interface {
	// Construct builder.
	Builder(ctx *plancontext.Context) (Builder, error)
	// Construct VM client.
	Client(ctx *plancontext.Context) (Client, error)
	// Construct validator.
	Validator(ctx *plancontext.Context) (Validator, error)
	// Construct DestinationClient.
	DestinationClient(ctx *plancontext.Context) (DestinationClient, error)
	// Ensurer
	Ensurer(ctx *plancontext.Context) (ensure Ensurer, err error)
}

// Builder API.
// Builds/updates objects as needed with provider
// specific constructs.
type Builder interface {
	// Build secret.
	Secret(vmRef ref.Ref, in, object *core.Secret) error
	// Build DataVolume config map.
	ConfigMap(vmRef ref.Ref, secret *core.Secret, object *core.ConfigMap) error
	// Build the Kubevirt VirtualMachine spec.
	VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) error
	// Build DataVolumes.
	DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *core.ConfigMap) (dvs []cdi.DataVolume, err error)
	// Build tasks.
	Tasks(vmRef ref.Ref) ([]*planapi.Task, error)
	// Build template labels.
	TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error)
	// Return a stable identifier for a DataVolume.
	ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string
	// Return a stable identifier for a PersistentDataVolume
	ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string
	// Conversion Pod environment
	PodEnvironment(vmRef ref.Ref, sourceSecret *core.Secret) (env []core.EnvVar, err error)
	// Build LUN PVs.
	LunPersistentVolumes(vmRef ref.Ref) (pvs []core.PersistentVolume, err error)
	// Build LUN PVCs.
	LunPersistentVolumeClaims(vmRef ref.Ref) (pvcs []core.PersistentVolumeClaim, err error)
	// check whether the builder supports Volume Populators
	SupportsVolumePopulators() bool
	// Build populator volumes
	PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) ([]*core.PersistentVolumeClaim, error)
	// Transferred bytes
	PopulatorTransferredBytes(persistentVolumeClaim *core.PersistentVolumeClaim) (transferredBytes int64, err error)
	// Storage offload metadata for the populator copy (e.g. "xcopyUsed"), surfaced as task annotations.
	PopulatorOffloadInfo(pvc *core.PersistentVolumeClaim) (info map[string]string, err error)
	// Set the populator PVC labels
	SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) (err error)
	// Get the populator task name associated to a PVC
	GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error)
	// Get the virtual machine preference name
	PreferenceName(vmRef ref.Ref, configMap *core.ConfigMap) (name string, err error)
	// Build VM ConfigMaps
	ConfigMaps(vmRef ref.Ref) (list []core.ConfigMap, err error)
	// Build VM Secrets
	Secrets(vmRef ref.Ref) (list []core.Secret, err error)
	// ConversionPodConfig returns provider-specific configuration for the virt-v2v conversion pod.
	// Returns an empty struct if no provider-specific configuration is needed.
	// The returned config is merged with user settings from Plan.Spec (user settings take precedence).
	ConversionPodConfig(vmRef ref.Ref) (*ConversionPodConfigResult, error)
	// NetAppShiftPVCs builds PVCs for disks mapped to NetApp Shift StorageClasses.
	// Returns nil for non-vSphere providers or when no Shift mappings exist.
	NetAppShiftPVCs(vmRef ref.Ref, labels map[string]string) ([]core.PersistentVolumeClaim, error)
	// CsiImportPVCs builds PVCs for disks that have CsiVolumeImport configured.
	CsiImportPVCs(vmRef ref.Ref, pvcLabels map[string]string) ([]core.PersistentVolumeClaim, error)
	// SourceVMLabelsAndAnnotations returns provider-specific labels and annotations
	// derived from source VM metadata (e.g. vSphere tags and custom attributes).
	SourceVMLabelsAndAnnotations(vmRef ref.Ref, tagMapping *api.TagMapping) (labels map[string]string, annotations map[string]string, sanitizationReport map[string]string, err error)
}

// Client API.
// Performs provider-specific actions on the source VM.
type Client interface {
	// Power on the source VM.
	PowerOn(vmRef ref.Ref) error
	// Power off the source VM.
	PowerOff(vmRef ref.Ref) error
	// Return the source VM's power state.
	PowerState(vmRef ref.Ref) (planapi.VMPowerState, error)
	// Return whether the source VM is powered off.
	PoweredOff(vmRef ref.Ref) (bool, error)
	// Create a snapshot of the source VM.
	CreateSnapshot(vmRef ref.Ref, hostsFunc util.HostsFunc) (snapshotId string, creationTaskId string, err error)
	// Remove a snapshot.
	RemoveSnapshot(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (removeTaskId string, err error)
	// Check if a snapshot is ready to transfer.
	CheckSnapshotReady(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (ready bool, snapshotId string, err error)
	// Check if a snapshot is removed.
	CheckSnapshotRemove(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (ready bool, err error)
	// Set DataVolume checkpoints.
	SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool, hostsFunc util.HostsFunc) (err error)
	// Close connections to the provider API.
	Close()
	// Finalize migrations
	Finalize(vms []*planapi.VMStatus, planName string)
	// Detach disks that are attached to the target VM without being cloned (e.g., LUNs).
	DetachDisks(vmRef ref.Ref) error
	// Actions on source env needed before running the populator pods
	PreTransferActions(vmRef ref.Ref) (ready bool, err error)
	// Get disk deltas for a VM snapshot.
	GetSnapshotDeltas(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (map[string]string, error)
}

// Validator API.
// Performs provider-specific validation.
type Validator interface {
	// Validate that a VM's disk backing storage has been mapped.
	StorageMapped(vmRef ref.Ref) (bool, error)
	// Validate that a VM's direct LUN/FC has the required details (oVirt only)
	DirectStorage(vmRef ref.Ref) (bool, error)
	// Validate that a VM's networks have been mapped.
	NetworksMapped(vmRef ref.Ref) (bool, error)
	// Validate that a VM's Host isn't in maintenance mode.
	MaintenanceMode(vmRef ref.Ref) (bool, error)
	// Validate whether warm migration is supported from this provider type.
	WarmMigration() bool
	// Validate whether the migration type is supported by this provider.
	MigrationType() bool
	// Return one source-network ref per VM NIC.
	NICNetworkRefs(vmRef ref.Ref) ([]ref.Ref, error)
	// Validate that we have information about static IPs for every virtual NIC
	StaticIPs(vmRef ref.Ref) (bool, error)
	// Validate if the UDN subnet matches the VM IP
	UdnStaticIPs(vmRef ref.Ref, client client.Client) (ok bool, err error)
	// Validate the shared disk, returns msg and category as the errors depends on the provider implementations
	SharedDisks(vmRef ref.Ref, client client.Client) (ok bool, msg string, category string, err error)
	// Validate that the vm has the change tracking enabled
	ChangeTrackingEnabled(vmRef ref.Ref) (bool, error)
	// Validate that VM has no pre-existing snapshots for warm migration
	HasSnapshot(vmRef ref.Ref) (ok bool, msg string, category string, err error)
	// Validate that the VM power state is compatible with the migration type.
	PowerState(vmRef ref.Ref) (bool, error)
	// Validate that the VM is inherently compatible with the migration type.
	VMMigrationType(vmRef ref.Ref) (bool, error)
	// Validate that the VM disks have valid sizes (> 0).
	InvalidDiskSizes(vmRef ref.Ref) ([]string, error)
	// Validate that the VM MAC addresses don't conflict with existing destination VMs.
	MacConflicts(vmRef ref.Ref) ([]MacConflict, error)
	// Validate that the PVC name template is valid
	PVCNameTemplate(vmRef ref.Ref, pvcNameTemplate string) (bool, error)
	// Validate guest tools installation and status (e.g., VMware Tools, VirtIO drivers).
	GuestToolsInstalled(vmRef ref.Ref) (ok bool, err error)
	// Validate that VM does not need to collapse any snapshots into a single base file
	ConsolidationNeeded(vmRef ref.Ref) (needed bool, err error)
	// ValidateCalicoNADs validates every Calico-referencing NAD in the
	// plan's network map. Issues are NAD-scoped (network/IPPool config);
	// the returned cache is consumed by CalicoVMIssues.
	ValidateCalicoNADs(client client.Client) (CalicoValidationResult, error)
	// CalicoVMIssues returns per-VM Calico issues (IP membership in subnet
	// / IPPool). Reads only from the cache produced by ValidateCalicoNADs;
	// VMs whose mapped NAD failed plan-level validation are skipped here
	// — their failure is already reported via CalicoNetworkInvalid.
	CalicoVMIssues(vmRef ref.Ref, cache *CalicoValidationCache) ([]CalicoIssue, error)
	// ValidateCalicoPrimary validates the (at most one) calico-flagged
	// NetworkMap entry — a type: pod destination carrying the calico
	// field. Returns plan-level issues (CRD presence, UDN conflict,
	// Network/VLAN/IPPool resolution) plus a cache consumed by
	// CalicoPrimaryIssues. On non-vSphere providers, returns a single
	// CalicoIssuePrimaryProviderUnsupported issue when any calico-flagged
	// entry is present.
	//
	// Precondition: Plan.Referenced.Map.Network is populated by the
	// dispatcher before this is called. With a nil NetworkMap, returns an
	// empty result and a non-nil cache with Primary == nil.
	ValidateCalicoPrimary(client client.Client) (CalicoPrimaryValidationResult, error)
	// CalicoPrimaryIssues returns per-VM issues for the calico-flagged
	// primary NIC (IP membership in IPPool / VLAN subnet, gated on
	// PreserveStaticIPs). Reads only from the cache produced by
	// ValidateCalicoPrimary; when the cache is nil or Primary is nil (plan
	// failed, or no calico-flagged entry exists), returns nil.
	//
	// When IP preservation is on but the VM has no findable IPv4 IPs
	// (IPv6-only or no GuestNetworks reported), no per-VM issue is emitted
	// and the builder emits no ipAddrs annotation — preservation is
	// best-effort.
	CalicoPrimaryIssues(vmRef ref.Ref, cache *CalicoPrimaryValidationCache) ([]CalicoPrimaryIssue, error)
}

// CalicoIssueKind enumerates the Calico Network failure modes.
type CalicoIssueKind string

const (
	// CalicoIssueNADUnreadable indicates the destination NAD could not be
	// fetched or parsed (NotFound, malformed JSON, transient API error). The
	// NAD's Calico configuration is unknowable until this is resolved.
	CalicoIssueNADUnreadable CalicoIssueKind = "NADUnreadable"
	// CalicoIssueNetworkNotFound no Network CR existed.
	CalicoIssueNetworkNotFound CalicoIssueKind = "NetworkNotFound"
	// CalicoIssueNetworkCRDAbsent the destination cluster does not have the
	// projectcalico.org/v3 Network CRD installed (the Calico install is too
	// old or does not ship the Network resource).
	CalicoIssueNetworkCRDAbsent CalicoIssueKind = "NetworkCRDAbsent"
	// CalicoIssueVRFVlanIgnored the NAD names a VLAN but the referenced
	// Network is a VRF (routed L3) network.
	CalicoIssueVRFVlanIgnored CalicoIssueKind = "VRFVlanIgnored"
	// CalicoIssueVRFNodeScoped every spec.vrf.hostConfig entry on the
	// referenced VRF Network carries a nodeSelector (an empty selector is
	// the canonical all-nodes form, so this means a node subset) and the
	// plan sets neither targetNodeSelector nor targetAffinity. A VM
	// scheduled onto an uncovered node fails to start, so with placement
	// uncontrolled the migration outcome is nondeterministic. Critical.
	CalicoIssueVRFNodeScoped CalicoIssueKind = "VRFNodeScoped"
	// CalicoIssueVRFPlacementUnverified as VRFNodeScoped, but the plan
	// constrains VM placement (targetNodeSelector/targetAffinity).
	// Warn-level: Forklift does not evaluate Calico's selector grammar,
	// so it cannot verify the plan's placement keeps VMs on nodes the
	// network's selectors cover.
	CalicoIssueVRFPlacementUnverified CalicoIssueKind = "VRFPlacementUnverified"
	// CalicoIssueVRFRouteTableReserved a hostConfig entry on the referenced
	// VRF Network claims kernel route table 253, 254 or 255 — the kernel's
	// own default/main/local tables, which a VRF must never take over.
	CalicoIssueVRFRouteTableReserved CalicoIssueKind = "VRFRouteTableReserved"
	// CalicoIssueVRFRouteTableConflict a routeTableIndex on the referenced
	// VRF Network is provably claimed elsewhere: another VRF Network uses
	// it with at least one of the two entries unscoped (an all-nodes entry
	// overlaps any node set), or it falls inside an explicit
	// FelixConfiguration routeTableRanges range. Calico documents such
	// conflicts as able to cause network outages.
	CalicoIssueVRFRouteTableConflict CalicoIssueKind = "VRFRouteTableConflict"
	// CalicoIssueVRFRouteTablePossibleConflict another VRF Network claims
	// the same routeTableIndex, but both entries carry nodeSelectors, so
	// whether they land on the same node depends on which nodes the
	// selectors match — unprovable without selector evaluation. Warn-level.
	CalicoIssueVRFRouteTablePossibleConflict CalicoIssueKind = "VRFRouteTablePossibleConflict"
	// CalicoIssueVRFDataplaneNotNftables a NAD references a VRF network but
	// the destination Calico install is not running the nftables dataplane.
	// VRF networking is supported only on nftables — BPF and iptables
	// installs cannot honour it — so FelixConfiguration "default" must have
	// nftablesMode Enabled and bpfEnabled off. nftablesMode "Auto" is also
	// reported: the mode Felix actually picks cannot be verified from here.
	// Emitted once per plan.
	CalicoIssueVRFDataplaneNotNftables CalicoIssueKind = "VRFDataplaneNotNftables"
	// CalicoIssueVRFPoolNotPinned the NAD references a VRF network but its
	// IPAM config pins no ipv4_pools, and the plan does not preserve static
	// IPs. Calico's IPAM is VRF-unaware; the documented convention pins the
	// VRF's dedicated IPPool via ipam.ipv4_pools in the NAD. Without the
	// pin, a freshly assigned address comes from whichever pool IPAM
	// selects — likely one the VRF's tenant fabric cannot route back to.
	// Warn-level, per NAD (the pin is a NAD property, not a Network one).
	// Deliberately not emitted when the plan preserves static IPs: the
	// addresses are then explicit via ipAddrs and already validated against
	// pools per-VM; the risk exists only for freshly-assigned addresses.
	CalicoIssueVRFPoolNotPinned CalicoIssueKind = "VRFPoolNotPinned"
	// CalicoIssueVRFNoBGPPeer no BGPPeer with spec.network naming the
	// referenced VRF Network exists. VRF networks ship with
	// inClusterMode: Local only — Felix programs a node's own pod routes;
	// routes to pods on other nodes exist only when such a BGPPeer
	// distributes them. Without one, VMs placed on different nodes silently
	// cannot reach each other inside the VRF. Warn-level, once per
	// referenced Network. Also emitted when the BGPPeer kind is unknown to
	// the apiserver: no peer can be bound in that state either.
	CalicoIssueVRFNoBGPPeer CalicoIssueKind = "VRFNoBGPPeer"
	// CalicoIssueVRFNoHostInterfaces a spec.vrf.hostConfig entry on the
	// referenced VRF Network names no hostInterfaces. Pods (and thus VMs)
	// on the nodes that entry matches have no path off their node in the
	// VRF. Warn-level, once per referenced Network.
	CalicoIssueVRFNoHostInterfaces CalicoIssueKind = "VRFNoHostInterfaces"
	// CalicoIssueNetworkHasNoL2Bridge Network CR existed but had no L2Bridge field spec'd.
	CalicoIssueNetworkHasNoL2Bridge CalicoIssueKind = "NetworkHasNoL2Bridge"
	// CalicoIssueNetworkHasNoVLANs Network CR's L2Bridge had an empty vlans list (no VLAN to select).
	CalicoIssueNetworkHasNoVLANs CalicoIssueKind = "NetworkHasNoVLANs"
	// CalicoIssueVLANNotInNetwork NIC's NAD entry's VLAN was not present in the referenced Network CR.
	CalicoIssueVLANNotInNetwork CalicoIssueKind = "VLANNotInNetwork"
	// CalicoIssueVLANRequired the NAD references a Calico Network but names
	// no VLAN. A VLAN must be stated explicitly whenever a Network is
	// selected; Forklift does not auto-select, not even for a single-VLAN
	// Network.
	CalicoIssueVLANRequired CalicoIssueKind = "VLANRequired"
	// CalicoIssueVLANHasNoIPPool no IPPool existed satisfying the VLAN subnet's requirements.
	CalicoIssueVLANHasNoIPPool CalicoIssueKind = "VLANHasNoIPPool"
	// CalicoIssueIPNotInSubnet NIC's IP was not in any Network.spec.l2Bridge.vlans[].subnets[].cidr.
	CalicoIssueIPNotInSubnet CalicoIssueKind = "IPNotInSubnet"
	// CalicoIssueIPNotInIPPool NIC's IP was not in any Calico IPPool.
	CalicoIssueIPNotInIPPool CalicoIssueKind = "IPNotInIPPool"
	// CalicoIssueTooManyIPs the NIC carries more than one IPv4 address.
	// Calico's ipAddrs annotation accepts at most one IPv4 per interface,
	// so preservation cannot represent this NIC — CNI ADD would fail and
	// the pod would never start.
	CalicoIssueTooManyIPs CalicoIssueKind = "TooManyIPs"
	// CalicoIssueNADMissingNetwork the NAD requests the Calico CNI but does
	// not name a projectcalico.org Network resource (no "network" field).
	// This is Calico's legacy L3 IPAM mode; identity preservation (MAC + IP)
	// will not be applied for NICs mapped to this NAD. Warn-level.
	CalicoIssueNADMissingNetwork CalicoIssueKind = "NADMissingNetwork"
	// CalicoIssueDataplaneNotBPF a NAD resolved to an l2Bridge network but
	// the destination Calico install is not running the BPF dataplane
	// (FelixConfiguration "default" has bpfEnabled false or unset). L2
	// networks require the BPF dataplane. Emitted once per plan.
	CalicoIssueDataplaneNotBPF CalicoIssueKind = "DataplaneNotBPF"

	// Calico-primary IssueKinds. Used by Validator.ValidateCalicoPrimary
	// and Validator.CalicoPrimaryIssues for the calico-flagged NetworkMap
	// path (type: pod destinations carrying the calico field). The Primary
	// prefix disambiguates from the NAD-path kinds above.

	// CalicoIssuePrimaryProviderUnsupported indicates a NetworkMap with a
	// calico-flagged entry is being consumed by a provider that does not
	// support the feature in this release (any non-vSphere provider).
	CalicoIssuePrimaryProviderUnsupported CalicoIssueKind = "PrimaryProviderUnsupported"
	// CalicoIssuePrimaryUnsupported indicates the destination cluster does
	// not have Calico installed (projectcalico.org/v3 IPPool CRD absent).
	// Network CRD absence with IPPool present is reported as the more
	// specific CalicoIssuePrimaryNetworkCRDAbsent.
	CalicoIssuePrimaryUnsupported CalicoIssueKind = "PrimaryUnsupported"
	// CalicoIssuePrimaryNetworkCRDAbsent the destination cluster has Calico
	// (IPPool CRD present) but the projectcalico.org/v3 Network CRD is not
	// installed. The user requested L2 attach via calico.network, but the
	// install does not ship the Network resource. The non-L2 case (calico.network
	// == "") continues to work in this state and is not blocked.
	CalicoIssuePrimaryNetworkCRDAbsent CalicoIssueKind = "PrimaryNetworkCRDAbsent"
	// CalicoIssuePrimaryConflictsWithUDN indicates the destination namespace
	// is labelled for a UDN primary network — incompatible with the calico
	// field.
	CalicoIssuePrimaryConflictsWithUDN CalicoIssueKind = "PrimaryConflictsWithUDN"
	// CalicoIssuePrimaryNetworkNotFound the named projectcalico.org/v3
	// Network CR does not exist on the destination cluster.
	CalicoIssuePrimaryNetworkNotFound CalicoIssueKind = "PrimaryNetworkNotFound"
	// CalicoIssuePrimaryNetworkTypeUnsupported the named Network CR is not
	// an l2Bridge network (e.g. a VRF network). Only l2Bridge networks can
	// back the primary NIC; a VRF network attaches via a multus NAD instead.
	CalicoIssuePrimaryNetworkTypeUnsupported CalicoIssueKind = "PrimaryNetworkTypeUnsupported"
	// CalicoIssuePrimaryNetworkHasNoL2Bridge the named Network CR has no
	// l2Bridge spec — incompatible with L2 attach.
	CalicoIssuePrimaryNetworkHasNoL2Bridge CalicoIssueKind = "PrimaryNetworkHasNoL2Bridge"
	// CalicoIssuePrimaryNetworkHasNoVLANs the named Network CR's
	// l2Bridge.vlans is empty.
	CalicoIssuePrimaryNetworkHasNoVLANs CalicoIssueKind = "PrimaryNetworkHasNoVLANs"
	// CalicoIssuePrimaryVLANRequired calico.network is set but calico.vlan
	// is not. A VLAN must be stated explicitly whenever a Network is
	// selected.
	CalicoIssuePrimaryVLANRequired CalicoIssueKind = "PrimaryVLANRequired"
	// CalicoIssuePrimaryVLANNotInNetwork the user-specified calico.vlan does
	// not match any vlan.id in the named Network CR.
	CalicoIssuePrimaryVLANNotInNetwork CalicoIssueKind = "PrimaryVLANNotInNetwork"
	// CalicoIssuePrimaryNoEligibleIPPool no IPPool covers either the L3
	// allocation (non-L2 case) or the matched VLAN's subnet (L2-attach case).
	CalicoIssuePrimaryNoEligibleIPPool CalicoIssueKind = "PrimaryNoEligibleIPPool"
	// CalicoIssuePrimaryIPNotInSubnet (L2-attach case only) the source NIC IP
	// falls outside the matched VLAN's subnets.
	CalicoIssuePrimaryIPNotInSubnet CalicoIssueKind = "PrimaryIPNotInSubnet"
	// CalicoIssuePrimaryTooManyIPs the calico-mapped NIC carries more than
	// one IPv4 address. Calico's ipAddrs annotation accepts at most one
	// IPv4 per interface, so preservation cannot represent this NIC — CNI
	// ADD would fail and the pod would never start.
	CalicoIssuePrimaryTooManyIPs CalicoIssueKind = "PrimaryTooManyIPs"
	// CalicoIssuePrimaryFieldsMisplaced the calico block is set on a
	// non-pod entry, or calico.vlan is set without calico.network, or more
	// than one calico-flagged entry exists in the map. The Network field
	// on the issue carries the offending value for message disambiguation.
	CalicoIssuePrimaryFieldsMisplaced CalicoIssueKind = "PrimaryFieldsMisplaced"
	// CalicoIssuePrimaryStaticIPsNotPreserved indicates a calico-flagged
	// NetworkMap entry exists while Plan.Spec.PreserveStaticIPs is false.
	// Bridge binding is still enabled (Calico requires it for L2 attach),
	// so DHCP-configured guests will pick up the Calico-assigned IP via
	// the veth. Static-IP-configured guests would have a diver. Warn-class — informational only.
	CalicoIssuePrimaryStaticIPsNotPreserved CalicoIssueKind = "PrimaryStaticIPsNotPreserved"
	// CalicoIssuePrimaryDataplaneNotBPF calico.network resolved to an
	// l2Bridge network but the destination Calico install is not running
	// the BPF dataplane (FelixConfiguration "default" has bpfEnabled false
	// or unset). L2 networks require the BPF dataplane.
	CalicoIssuePrimaryDataplaneNotBPF CalicoIssueKind = "PrimaryDataplaneNotBPF"
)

// CalicoIssue represents a per-VM Calico Network validation failure: the
// VM's NIC IP does not fit the destination's Calico Network VLAN subnet or
// IPPool. NAD-level issues (NetworkNotFound, NetworkHasNoL2Bridge, etc.)
// are surfaced via CalicoNADIssue, not this type.
type CalicoIssue struct {
	Kind CalicoIssueKind
	// Network is the Calico Network CR name reference.
	Network string
	// VLAN is the resolved l2Bridge.vlans[].vlan.id (zero when the NAD's
	// Network is a VRF network — VRF networks carry no VLANs).
	VLAN uint16
	// IP is the source VM IP.
	IP string
}

// DestinationClient API.
// Performs provider-specific actions on the Destination cluster
type DestinationClient interface {
	// Deletes Populator Data Source
	DeletePopulatorDataSource(vm *planapi.VMStatus) error
	// Set the VolumePopulator CustomResource Ownership.
	SetPopulatorCrOwnership() error
}

// Ensurer API
// Ensures creates or check that the resources are present
type Ensurer interface {
	// SharedConfigMaps ensures that shared ConfigMap with VM are present
	SharedConfigMaps(vm *planapi.VMStatus, configMaps []core.ConfigMap) (err error)
	// SharedSecrets ensures that shared Secret with VM are present
	SharedSecrets(vm *planapi.VMStatus, secrets []core.Secret) (err error)
	// PersistentVolumeClaims ensures that PVCs are present on the destination.
	PersistentVolumeClaims(vm *planapi.VMStatus, pvcs []core.PersistentVolumeClaim) (err error)
}
