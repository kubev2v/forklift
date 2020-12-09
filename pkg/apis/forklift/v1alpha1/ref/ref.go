package ref

//
// Source reference.
// Either the ID or Name must be specified.
type Ref struct {
	// The object ID.
	// vsphere:
	//   The managed object ID.
	ID string `json:"id,omitempty"`
	// An object Name.
	// vsphere:
	//   A qualified name.
	Name string `json:"name,omitempty"`
	// Type used to qualify the name.
	Type string `json:"type,omitempty"`
}

//
// Determine if the ref either the ID or Name is set.
func (r Ref) NotSet() bool {
	return r.ID == "" && r.Name == ""
}

//
// String representation.
func (r *Ref) String() (s string) {
	if r.ID != "" {
		s = r.ID
	} else {
		s = r.Name
	}
	if r.Type != "" {
		s = "/" + r.Type + s
	}

	return
}
