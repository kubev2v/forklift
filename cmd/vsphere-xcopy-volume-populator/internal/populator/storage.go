package populator

import (
	"errors"
)

const (
	// CleanupXcopyInitiatorGroup is the key to signal cleanup of the initiator group.
	CleanupXcopyInitiatorGroup = "cleanupXcopyInitiatorGroup"
)

//go:generate go run go.uber.org/mock/mockgen -destination=mocks/storage_mock_client.go -package=storage_mocks . StorageApi
type StorageApi interface {
	StorageMapper
	StorageResolver
	AdapterIdHandler
}
type AdapterIdHandler interface {
	GetAdaptersID() ([]string, error)
	AddAdapterID(adapterID string)
}

type MappingContext map[string]any

type StorageMapper interface {
	// EnsureClonnerIgroup creates or updates an initiator group with the clonnerIqn
	EnsureClonnerIgroup(initiatorGroup string, adapterIds []string) (MappingContext, error)
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

type AdapterIdHandlerImpl struct {
	adaptersID []string
}

func (a *AdapterIdHandlerImpl) GetAdaptersID() ([]string, error) {
	if len(a.adaptersID) == 0 {
		return nil, errors.New("adapters ID are not set")
	}
	return a.adaptersID, nil
}

func (a *AdapterIdHandlerImpl) AddAdapterID(adapterID string) {
	if len(a.adaptersID) == 0 {
		a.adaptersID = make([]string, 0)
	}
	a.adaptersID = append(a.adaptersID, adapterID)
}
