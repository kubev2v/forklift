package base

import (
	"fmt"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
)

type Model = libmodel.Model
type ListOptions = libmodel.ListOptions

//
// An object reference.
type Ref struct {
	// The kind (type) of the referenced.
	Kind string `json:"kind"`
	// The ID of object referenced.
	ID string `json:"id"`
}

//
// Invalid reference.
type InvalidRefError struct {
	Ref
}

func (r InvalidRefError) Error() string {
	return fmt.Sprintf("Reference %#v not valid.", r.Ref)
}

//
// Invalid kind.
type InvalidKindError struct {
	Object interface{}
}

func (r InvalidKindError) Error() string {
	return fmt.Sprintf("Kind %#v not valid.", r.Object)
}
