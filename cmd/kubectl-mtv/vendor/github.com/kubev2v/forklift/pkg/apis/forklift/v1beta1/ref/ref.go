package ref

import "fmt"

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
	// The VM Namespace
	// Only relevant for an openshift source.
	Namespace string `json:"namespace,omitempty"`
	// Type used to qualify the name.
	Type string `json:"type,omitempty"`
}

// Determine if the ref either the ID or Name is set.
func (r Ref) NotSet() bool {
	return r.ID == "" && r.Name == "" && r.Type == ""
}

// String representation.
func (r *Ref) String() (s string) {
	if r.Type != "" {
		s = fmt.Sprintf(
			"(%s)",
			r.Type)
	}
	s = fmt.Sprintf(
		"%s id:%s name:'%s' ",
		s,
		r.ID,
		r.Name)

	return
}

// Collection of Refs.
type Refs struct {
	List []Ref `json:"references,omitempty"`
}

// Determine whether the list of refs contains a given ref.
func (r *Refs) Find(ref Ref) (found bool) {
	for _, r := range r.List {
		if r.ID == ref.ID {
			found = true
			break
		}
	}
	return
}
