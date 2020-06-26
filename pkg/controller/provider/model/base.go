package model

import (
	"encoding/json"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/controller/pkg/logging"
)

// Errors
var NotFound = libmodel.NotFound
var Conflict = libmodel.Conflict

const (
	Assign = "assign"
)

func init() {
	logger := logging.WithName("")
	logger.Reset()
	Log = &logger
}

//
// Extended model.
type Model libmodel.Model

//
// Base VMWare model.
type Base struct {
	// Primary key (digest).
	PK string `sql:"pk"`
	// Provider
	ID string `sql:"key,unique(a)"`
	// Name
	Name string `sql:""`
	// Parent
	Parent string `sql:"index(a)"`
	// The raw json-encoded object.
	Object string `sql:""`
}

//
// Get the PK.
func (m *Base) Pk() string {
	return m.PK
}

//
// Set the primary key.
func (m *Base) SetPk() {
	m.PK = m.ID
}

func (m *Base) String() string {
	return m.ID
}

func (m *Base) Labels() libmodel.Labels {
	return nil
}

func (m *Base) Equals(other libmodel.Model) bool {
	if vm, cast := other.(*VM); cast {
		return m.ID == vm.ID
	}

	return false
}

//
// Encode the parent.
// Set the `Parent` field using the ref.
func (m *Base) EncodeParent(r Ref) {
	m.Parent = r.Encode()
}

//
// Decode parent and get the Ref.
func (m *Base) DecodeParent() Ref {
	r := Ref{}
	r.With(m.Parent)
	return r
}

//
// Encode the object.
// Set the `Object` field using the ref.
func (m *Base) EncodeObject(object Object) {
	m.Object = object.Encode()
}

//
// Decode the `Object` ref.
func (m *Base) DecodeObject() Object {
	r := Object{}
	r.With(m.Object)
	return r
}

//
// Encoded `Object` content.
type Object map[string]interface{}

//
// Encode the object.
func (r *Object) Encode() string {
	j, _ := json.Marshal(r)
	return string(j)
}

//
// Unmarshal the json `j` into self.
func (r *Object) With(s string) {
	json.Unmarshal([]byte(s), r)
}

//
// An object reference.
type Ref struct {
	// The kind (type) of the referenced.
	Kind string
	// The ID of object referenced.
	ID string
}

//
// Encode the ref.
func (r *Ref) Encode() string {
	j, _ := json.Marshal(r)
	return string(j)
}

//
// Unmarshal the json `j` into self.
func (r *Ref) With(j string) {
	json.Unmarshal([]byte(j), r)
}

//
// List of `Ref`.
type RefList []Ref

//
// Encode the list.
func (r *RefList) Encode() string {
	j, _ := json.Marshal(r)
	return string(j)
}

//
// Unmarshal the json `j` into self.
func (r *RefList) With(j string) {
	json.Unmarshal([]byte(j), r)
}
