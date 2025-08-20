package populator

//go:generate mockgen -destination=mocks/storage_mock_client.go -package=storage_mocks . StorageApi
type StorageApi interface {
	StorageMapper
	StorageResolver
}

type MappingContext map[string]any

type StorageMapper interface {
	// EnsureClonnerIgroup creates or updates an initiator group with the clonnerIqn
	EnsureClonnerIgroup(initiatorGroup string, clonnerIqn []string) (MappingContext, error)
	// Map is responsible to mapping an initiator group to a LUN
	Map(initatorGroup string, targetLUN LUN, context MappingContext) (LUN, error)
	// UnMap is responsible to unmapping an initiator group from a LUN
	UnMap(initatorGroup string, targetLUN LUN, context MappingContext) error
	// CurrentMappedGroups returns the initiator groups the LUN is mapped to
	CurrentMappedGroups(targetLUN LUN, context MappingContext) ([]string, error)
}

type StorageResolver interface {
	ResolvePVToLUN(persistentVolume PersistentVolume) (LUN, error)
}

type SciniAware interface {
	SciniRequired() bool
}
