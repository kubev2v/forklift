package populator

import (
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
)

const (
	// CleanupXcopyInitiatorGroup is the key to signal cleanup of the initiator group.
	CleanupXcopyInitiatorGroup = "cleanupXcopyInitiatorGroup"
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
