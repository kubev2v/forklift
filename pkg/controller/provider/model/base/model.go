package base

import (
	"fmt"

	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

type Model = libmodel.Model
type ListOptions = libmodel.ListOptions

func init() {
	libmodel.DefaultDetail = 1
}

const (
	MaxDetail = libmodel.MaxDetail
)

// An object reference.
type Ref struct {
	// The kind (type) of the referenced.
	Kind string `json:"kind"`
	// The ID of object referenced.
	ID string `json:"id"`
}

// Invalid reference.
type InvalidRefError struct {
	Ref
}

func (r InvalidRefError) Error() string {
	return fmt.Sprintf("Reference %#v not valid.", r.Ref)
}

// Invalid kind.
type InvalidKindError struct {
	Object interface{}
}

func (r InvalidKindError) Error() string {
	return fmt.Sprintf("Kind %#v not valid.", r.Object)
}

// VM concerns.
type Concern struct {
	Id         string `json:"id"`
	Label      string `json:"label"`
	Category   string `json:"category"`
	Assessment string `json:"assessment"`
}
