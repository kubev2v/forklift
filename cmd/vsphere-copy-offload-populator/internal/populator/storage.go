package populator

import (
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware"
)

//go:generate go run go.uber.org/mock/mockgen -destination=mocks/storage_mock_client.go -package=mocks . StorageApi
type StorageApi interface {
	VMDKCapable
}

// StorageResolver resolves a PersistentVolume to LUN details
// This interface is embedded by VVolCapable, RDMCapable, and VMDKCapable
type StorageResolver interface {
	// ResolvePVToLUN resolves PersistentVolume to LUN details
	ResolvePVToLUN(persistentVolume PersistentVolume) (LUN, error)
}

// VVolCapable defines storage that can perform VVol operations
type VVolCapable interface {
	StorageResolver
	// VvolCopy performs a direct copy operation using vSphere API to discover source volume
	VvolCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume PersistentVolume, progress chan<- uint64) error
}

// RDMCapable defines storage that can perform RDM operations
type RDMCapable interface {
	StorageResolver
	// RDMCopy performs a copy operation for RDM-backed disks
	RDMCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume PersistentVolume, progress chan<- uint64) error
}

// StorageMapper handles initiator group mapping for VMDK/Xcopy operations
type StorageMapper interface {
	// EnsureClonnerIgroup creates or updates an initiator group with the clonnerIqn
	EnsureClonnerIgroup(initiatorGroup string, clonnerIqn []string) (MappingContext, error)
	// MapTarget maps the LUN to the clonner group (internalized).
	MapTarget(targetLUN LUN, context MappingContext) (LUN, error)
	// UnmapTarget unmaps the LUN from the clonner group (internalized).
	UnmapTarget(targetLUN LUN, context MappingContext) error
	// Map is responsible for mapping an initiator group to a LUN
	Map(initatorGroup string, targetLUN LUN, context MappingContext) (LUN, error)
	// UnMap is responsible for unmapping an initiator group from a LUN
	UnMap(initatorGroup string, targetLUN LUN, context MappingContext) error
	// CurrentMappedGroups returns the initiator groups the LUN is mapped to
	CurrentMappedGroups(targetLUN LUN, context MappingContext) ([]string, error)
}

// VMDKCapable defines storage that can perform VMDK/Xcopy operations (DEFAULT fallback)
// This is the required interface - all storage implementations must support this
type VMDKCapable interface {
	StorageMapper
	StorageResolver
}

// MappingContext holds context information for mapping operations
type MappingContext map[string]any

// SciniAware indicates that a storage requires scini module (PowerFlex)
type SciniAware interface {
	SciniRequired() bool
}

// ProtocolConflictResolver is an optional interface for storage backends that need to
// temporarily remove NVMe-connected hosts before the iSCSI/FC xcopy mapping can succeed.
//
// SCSI XCOPY requires SCSI, so the ESXi host performing xcopy always connects via iSCSI or
// FC.  Pure FlashArray (and similar arrays) reject adding an iSCSI/FC host connection to a
// volume that already has an NVMe-oF host connected.  When the target PVC was provisioned
// on OpenShift via NVMe-oF/TCP by the CSI driver, the volume is pre-connected to one or
// more NVMe hosts that must be temporarily removed before xcopy can proceed.
//
// Implementations must:
//   - Record the evicted host names inside MappingContext so RestoreConflictingConnections
//     can restore them unconditionally after xcopy finishes (success or failure).
//   - Be idempotent: if there are no NVMe connections the call is a no-op.
type ProtocolConflictResolver interface {
	// EvictConflictingConnections disconnects any NVMe-connected hosts from targetLUN
	// so that the iSCSI/FC xcopy initiator can be mapped.  Evicted host names are stored
	// in mappingContext for later restoration.
	EvictConflictingConnections(targetLUN LUN, mappingContext MappingContext) error

	// RestoreConflictingConnections reconnects any hosts that were evicted by
	// EvictConflictingConnections.  Should be called in a defer after xcopy completes,
	// whether or not xcopy succeeded.
	RestoreConflictingConnections(targetLUN LUN, mappingContext MappingContext) error
}

// StorageArrayInfo holds metadata about the storage array, retrieved from the API at connection time.
type StorageArrayInfo struct {
	// Vendor is the storage array vendor (e.g. "IBM", "Dell", "NetApp").
	Vendor string
	// Product is the vendor's product name (e.g. "FlashSystem", "PowerMax", "ONTAP").
	Product string
	// Model is the specific model of the storage array, retrieved from the API. May be empty.
	Model string
	// Version is the software/firmware version of the storage array, retrieved from the API. May be empty.
	Version string
}

// StorageArrayInfoProvider is an optional interface that storage implementations can implement
// to provide metadata about the storage array for metric labels.
type StorageArrayInfoProvider interface {
	GetStorageArrayInfo() StorageArrayInfo
}
