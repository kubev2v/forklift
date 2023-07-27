package base

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	core "k8s.io/api/core/v1"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// Annotations
const (
	// Used on DataVolume, contains disk source -- e.g. backing file in
	// VMware or disk ID in oVirt.
	AnnDiskSource = "forklift.konveyor.io/disk-source"
)

var VolumePopulatorNotSupportedError = liberr.New("provider does not support volume populators")

// Adapter API.
// Constructs provider-specific implementations
// of the Builder, Client, and Validator.
type Adapter interface {
	// Construct builder.
	Builder(ctx *plancontext.Context) (Builder, error)
	// Construct VM client.
	Client(ctx *plancontext.Context) (Client, error)
	// Construct validator.
	Validator(plan *api.Plan) (Validator, error)
	// Construct DestinationClient.
	DestinationClient(ctx *plancontext.Context) (DestinationClient, error)
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
	VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []core.PersistentVolumeClaim) error
	// Build DataVolumes.
	DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume) (dvs []cdi.DataVolume, err error)
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
	PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) (pvcNames []string, err error)
	// Transferred bytes
	PopulatorTransferredBytes(persistentVolumeClaim *core.PersistentVolumeClaim) (transferredBytes int64, err error)
	// Set the populator PVC labels
	SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []core.PersistentVolumeClaim) (err error)
	// Get the populator task name associated to a PVC
	GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (taskName string, err error)
}

// Client API.
// Performs provider-specific actions on the source VM.
type Client interface {
	// Power on the source VM.
	PowerOn(vmRef ref.Ref) error
	// Power off the source VM.
	PowerOff(vmRef ref.Ref) error
	// Return the source VM's power state.
	PowerState(vmRef ref.Ref) (string, error)
	// Return whether the source VM is powered off.
	PoweredOff(vmRef ref.Ref) (bool, error)
	// Create a snapshot of the source VM.
	CreateSnapshot(vmRef ref.Ref) (string, error)
	// Remove all warm migration snapshots.
	RemoveSnapshots(vmRef ref.Ref, precopies []planapi.Precopy) error
	// Check if a snapshot is ready to transfer.
	CheckSnapshotReady(vmRef ref.Ref, snapshot string) (ready bool, err error)
	// Set DataVolume checkpoints.
	SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool) (err error)
	// Close connections to the provider API.
	Close()
	// Finalize migrations
	Finalize(vms []*planapi.VMStatus, planName string)
	// Detach disks that are attached to the target VM without being cloned (e.g., LUNs).
	DetachDisks(vmRef ref.Ref) error
	// Actions on source env needed before running the populator pods
	PreTransferActions(vmRef ref.Ref) (ready bool, err error)
}

// Validator API.
// Performs provider-specific validation.
type Validator interface {
	// Validate that a VM's disk backing storage has been mapped.
	StorageMapped(vmRef ref.Ref) (bool, error)
	// Validate that a VM's networks have been mapped.
	NetworksMapped(vmRef ref.Ref) (bool, error)
	// Validate that a VM's Host isn't in maintenance mode.
	MaintenanceMode(vmRef ref.Ref) (bool, error)
	// Validate whether warm migration is supported from this provider type.
	WarmMigration() bool
	// Validate that no more than one of a VM's networks is mapped to the pod network.
	PodNetwork(vmRef ref.Ref) (bool, error)
}

// DestinationClient API.
// Performs provider-specific actions on the Destination cluster
type DestinationClient interface {
	// Deletes Populator Data Source
	DeletePopulatorDataSource(vm *planapi.VMStatus) error
	// Set the VolumePopulator CustomResource Ownership.
	SetPopulatorCrOwnership() error
}
