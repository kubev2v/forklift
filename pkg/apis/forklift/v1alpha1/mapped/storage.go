package mapped

import "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"

//
// Mapped storage.
type StoragePair struct {
	// Source storage.
	Source ref.Ref `json:"source"`
	// Destination storage.
	Destination DestinationStorage `json:"destination"`
}

//
// Mapped storage destination.
type DestinationStorage struct {
	// A storage class.
	StorageClass string `json:"storageClass"`
}
