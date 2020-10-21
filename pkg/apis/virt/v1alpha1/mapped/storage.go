package mapped

//
// Mapped storage.
type StoragePair struct {
	// Source storage.
	Source SourceObject `json:"source"`
	// Destination storage.
	Destination DestinationStorage `json:"destination"`
}

//
// Mapped storage destination.
type DestinationStorage struct {
	// A storage class.
	StorageClass string `json:"storageClass"`
}
