package mapped

//
// Mapped source.
type SourceObject struct {
	// The object identifier.
	// For:
	//   - vsphere: The managed object ID.
	ID string `json:"id"`
}
