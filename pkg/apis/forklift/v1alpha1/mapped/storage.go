package mapped

import (
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
	core "k8s.io/api/core/v1"
)

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
	// Volume mode.
	// +kubebuilder:validation:Enum=Filesystem;Block
	VolumeMode core.PersistentVolumeMode `json:"volumeMode,omitempty"`
	// Access mode.
	// +kubebuilder:validation:Enum=ReadWriteOnce;ReadWriteMany;ReadOnlyMany
	AccessMode core.PersistentVolumeAccessMode `json:"accessMode,omitempty"`
}
