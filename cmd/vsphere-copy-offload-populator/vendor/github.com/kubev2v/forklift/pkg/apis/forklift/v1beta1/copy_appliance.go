package v1beta1

import (
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const CopyApplianceFinalizer = "forklift/copy-appliance"

// CopyApplianceAnnotation is set on the appliance VM notes in vSphere so
// inventory can ignore it (e.g. shared-disk counting) and operators can
// recognize it.
const CopyApplianceAnnotation = "Forklift Copy Appliance"

// Label keys and values applied to CopyAppliance CRs. LabelMigration /
// LabelVM match plan conversion-context keys.
const (
	LabelApp       = "app"
	LabelSubapp    = "subapp"
	LabelProvider  = "provider"
	LabelMigration = "migration"
	LabelVM        = "vmID"
	AppForklift    = "forklift"

	SubappAppliance = "copy-appliance"
	SubappCheck     = "copy-appliance-check"
)

// LabelCopyApplianceSetup marks setup pods owned by a CopyAppliance (value is
// the CopyAppliance name, which is in the pod's own namespace).
const LabelCopyApplianceSetup = "forklift.konveyor.io/copy-appliance-setup"

// Phases of the CopyAppliance deploy, export, and teardown itineraries.
// Written to Status.Phase; terminal values are DeployCompleted, DeployFailed,
// Released, TeardownCompleted, and TeardownFailed.
//
// PhaseWaitForPowerOff is defined with the plan migration phases in doc.go
// (same string value) and is reused here.
const (
	PhaseDeployFailed        = "DeployFailed"
	PhaseCloneVM             = "CloneVM"
	PhaseWaitForClone        = "WaitForClone"
	PhaseWaitForNetwork      = "WaitForNetwork"
	PhaseSetupAppliance      = "SetupAppliance"
	PhaseWaitForExports      = "WaitForExports"
	PhaseReleased            = "Released"
	PhaseReleaseDisks        = "ReleaseDisks"
	PhaseWaitForReleaseDisks = "WaitForReleaseDisks"
	PhaseAttachDisks         = "AttachDisks"
	PhaseWaitForAttachDisks  = "WaitForAttachDisks"
	PhaseRestartOrchestrator = "RestartOrchestrator"
	PhasePowerOff            = "PowerOff"
	PhaseDetachDisks         = "DetachDisks"
	PhaseWaitForDetachDisks  = "WaitForDetachDisks"
	PhaseDestroyVM           = "DestroyVM"
	PhaseWaitForDestroyVM    = "WaitForDestroyVM"
	PhaseDeployCompleted     = "DeployCompleted"
	PhaseTeardownCompleted   = "TeardownCompleted"
	PhaseTeardownFailed      = "TeardownFailed"
)

// Phases an older controller could have left in Status.Phase. Loading the image
// and installing the supervisor ran here, one phase each; both now run in a
// setup pod under PhaseSetupAppliance. The deploy itinerary no longer contains
// these, and an appliance found on either is sent to PhaseSetupAppliance.
const (
	PhaseConfigure = "Configure"
	PhaseLoadImage = "LoadImage"
)

// CopyAppliance specification.
//
// The appliance VM is a clone of Template, named after the CopyAppliance
// itself, and that name is how an operator recognizes the VM in the vSphere
// inventory. The controller finds it again by the moRef recorded in the status.
type CopyApplianceSpec struct {
	// Source provider in which the appliance VM is created.
	Provider core.ObjectReference `json:"provider" ref:"Provider"`
	// Secret holding everything the controller reaches the appliance with.
	// Name is required; Namespace defaults to the CopyAppliance namespace.
	//
	// The SSH private key is read from the "private-key" data key; the matching
	// public key is expected to be installed in the appliance image already.
	//
	// The mutual-TLS material the appliance serves its exports with is read from
	// the data keys named for the files the appliance expects (see
	// pkg/nbd-container/announce). The controller installs the CA and the
	// server half on the appliance and keeps the client half to query the
	// exports with.
	//
	// The server certificate must be issued for the logical name "nbd-server"
	// rather than for an address. The appliance is cloned on demand and its
	// address is not known when the certificate is issued, so the client
	// verifies the name instead of where it reached it.
	Secret core.ObjectReference `json:"secret" ref:"Secret"`
	// Fully qualified pull spec for the container image loaded into the
	// appliance's podman store. A setup pod reads the image and streams it to
	// the appliance over SSH, so the appliance needs no registry access of its
	// own. The pod carries no credential, so the image has to be readable
	// without one.
	// +kubebuilder:validation:MinLength=1
	ContainerImage string `json:"containerImage"`
	// Datacenter in which the appliance VM is created.
	// +optional
	Datacenter string `json:"datacenter,omitempty"`
	// Datastore that holds the appliance VM home directory.
	Datastore string `json:"datastore"`
	// Resource pool in which the appliance VM is created.
	// +kubebuilder:validation:MinLength=1
	ResourcePool string `json:"resourcePool"`
	// Inventory folder in which the appliance VM is created.
	Folder string `json:"folder"`
	// Disks to attach to the appliance VM for export. Each entry names an
	// existing VMDK and carries the VMware identifiers needed to correlate
	// guest exports with source inventory. Empty means disks should be
	// detached (release); non-empty means they should be attached and
	// exporting. Changing the set while terminal re-enters the export
	// itinerary.
	// Capped so that the root disk plus the attached disks fit within the
	// four SCSI controllers vSphere permits per VM (4 x 15 addressable
	// units = 60 disks).
	// +kubebuilder:validation:MaxItems=59
	// +optional
	AttachDisks []AttachedDisk `json:"attachDisks,omitempty"`
	// Inventory path of the VM template the appliance is cloned from. The
	// template supplies the root disk, so it must support the controller its
	// disks are attached to, and the one network the appliance is reached on,
	// which the clone inherits as-is.
	// +kubebuilder:validation:MinLength=1
	Template string `json:"template"`
}

// AttachedDisk is an existing VMDK to attach to the copy appliance.
type AttachedDisk struct {
	// Datastore path of the VMDK (e.g. "[datastore13] some-vm/disk-0.vmdk").
	// +kubebuilder:validation:Pattern=`^\[[^\]]+\]\s*.+\.vmdk$`
	VMDKPath string `json:"vmdkPath"`
	// VMware virtual device key from inventory.
	// +optional
	DiskKey int32 `json:"diskKey,omitempty"`
	// backing.Uuid from inventory. Used to match guest exports.
	// +optional
	Serial string `json:"serial,omitempty"`
	// Disk capacity in bytes. Used as a secondary match key when serial is empty.
	// +optional
	Capacity int64 `json:"capacity,omitempty"`
}

// ApplianceAddress is an address the appliance VM's guest reports on its
// network adapter. The template gives the appliance one network, and an adapter
// can hold more than one address on it.
type ApplianceAddress struct {
	// Name of the portgroup the guest reports the adapter is attached to.
	Network string `json:"network"`
	// MAC address of the adapter.
	MAC string `json:"mac"`
	// IP address.
	IP string `json:"ip"`
}

// ApplianceExport is one disk the appliance publishes over NBD.
type ApplianceExport struct {
	// Stable identifier the appliance's guest resolved for the disk. It falls
	// back to the device path when the guest can report nothing better.
	WWID string `json:"wwid"`
	// Port on the appliance the export is served on.
	Port int32 `json:"port"`
	// Device node the export reads, as the appliance's guest sees it.
	Device string `json:"device"`
	// VMware virtual device key of the attached source disk.
	// +optional
	DiskKey int32 `json:"diskKey,omitempty"`
	// Datastore path of the attached source VMDK.
	// +optional
	VMDKPath string `json:"vmdkPath,omitempty"`
	// backing.Uuid of the attached source disk from inventory.
	// +optional
	SourceSerial string `json:"sourceSerial,omitempty"`
}

// CopyAppliance status.
type CopyApplianceStatus struct {
	// Conditions.
	libcnd.Conditions `json:",inline"`
	// The managed object reference ID of the created appliance VM.
	// +optional
	MoRef string `json:"moRef,omitempty"`
	// The addresses the appliance VM's guest reports on its network, one entry
	// per address, in the order the guest reports them. Empty until the guest
	// has booted far enough to answer.
	// +optional
	Addresses []ApplianceAddress `json:"addresses,omitempty"`
	// The instance UUID of the vCenter the appliance VM was created in. A
	// managed object reference is only unique within one vCenter, so MoRef
	// must not be trusted when this does not match the connected instance.
	// +optional
	VCenterInstanceUUID string `json:"vcenterInstanceUUID,omitempty"`
	// The digest-tagged reference the container image is loaded under in the
	// appliance's podman store. The orchestrator unit is rendered to run it.
	// +optional
	ExporterImage string `json:"exporterImage,omitempty"`
	// The disk exports the appliance publishes, one per attached disk. Empty
	// until the appliance is serving all of them.
	// +optional
	Exports []ApplianceExport `json:"exports,omitempty"`
	// Current deploy, export, or teardown phase. Terminal: DeployCompleted
	// (exporting), DeployFailed, Released (detached), TeardownCompleted,
	// TeardownFailed.
	// +optional
	Phase string `json:"phase,omitempty"`
	// The managed object reference ID of the vSphere task the current phase is
	// waiting on. Empty when the phase has nothing outstanding.
	// +optional
	TaskRef string `json:"taskRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CopyAppliance is the Schema for API which causes copy appliances to be created
// in source hypervisors to facilitate disk transfer.
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
type CopyAppliance struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            CopyApplianceSpec   `json:"spec,omitempty"`
	Status          CopyApplianceStatus `json:"status,omitempty"`
}

// Address returns the address to reach the appliance at, and whether the guest
// has reported one yet. The appliance answers on any of its addresses, so the
// first one will do.
func (r *CopyAppliance) Address() (address string, ok bool) {
	if len(r.Status.Addresses) == 0 {
		return
	}
	address = r.Status.Addresses[0].IP
	ok = true
	return
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CopyApplianceList contains a list of CopyAppliances
type CopyApplianceList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []CopyAppliance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CopyAppliance{}, &CopyApplianceList{})
}
