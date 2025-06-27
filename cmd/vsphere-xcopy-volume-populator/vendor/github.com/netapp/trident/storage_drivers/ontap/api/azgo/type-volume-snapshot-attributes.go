// Code generated automatically. DO NOT EDIT.
// Copyright 2022 NetApp, Inc. All Rights Reserved.

package azgo

import (
	"encoding/xml"
	log "github.com/sirupsen/logrus"
	"reflect"
)

// VolumeSnapshotAttributesType is a structure to represent a volume-snapshot-attributes ZAPI object
type VolumeSnapshotAttributesType struct {
	XMLName                           xml.Name `xml:"volume-snapshot-attributes"`
	AutoSnapshotsEnabledPtr           *bool    `xml:"auto-snapshots-enabled"`
	SnapdirAccessEnabledPtr           *bool    `xml:"snapdir-access-enabled"`
	SnapshotCloneDependencyEnabledPtr *bool    `xml:"snapshot-clone-dependency-enabled"`
	SnapshotCountPtr                  *int     `xml:"snapshot-count"`
	SnapshotPolicyPtr                 *string  `xml:"snapshot-policy"`
}

// NewVolumeSnapshotAttributesType is a factory method for creating new instances of VolumeSnapshotAttributesType objects
func NewVolumeSnapshotAttributesType() *VolumeSnapshotAttributesType {
	return &VolumeSnapshotAttributesType{}
}

// ToXML converts this object into an xml string representation
func (o *VolumeSnapshotAttributesType) ToXML() (string, error) {
	output, err := xml.MarshalIndent(o, " ", "    ")
	if err != nil {
		log.Errorf("error: %v", err)
	}
	return string(output), err
}

// String returns a string representation of this object's fields and implements the Stringer interface
func (o VolumeSnapshotAttributesType) String() string {
	return ToString(reflect.ValueOf(o))
}

// AutoSnapshotsEnabled is a 'getter' method
func (o *VolumeSnapshotAttributesType) AutoSnapshotsEnabled() bool {
	var r bool
	if o.AutoSnapshotsEnabledPtr == nil {
		return r
	}
	r = *o.AutoSnapshotsEnabledPtr
	return r
}

// SetAutoSnapshotsEnabled is a fluent style 'setter' method that can be chained
func (o *VolumeSnapshotAttributesType) SetAutoSnapshotsEnabled(newValue bool) *VolumeSnapshotAttributesType {
	o.AutoSnapshotsEnabledPtr = &newValue
	return o
}

// SnapdirAccessEnabled is a 'getter' method
func (o *VolumeSnapshotAttributesType) SnapdirAccessEnabled() bool {
	var r bool
	if o.SnapdirAccessEnabledPtr == nil {
		return r
	}
	r = *o.SnapdirAccessEnabledPtr
	return r
}

// SetSnapdirAccessEnabled is a fluent style 'setter' method that can be chained
func (o *VolumeSnapshotAttributesType) SetSnapdirAccessEnabled(newValue bool) *VolumeSnapshotAttributesType {
	o.SnapdirAccessEnabledPtr = &newValue
	return o
}

// SnapshotCloneDependencyEnabled is a 'getter' method
func (o *VolumeSnapshotAttributesType) SnapshotCloneDependencyEnabled() bool {
	var r bool
	if o.SnapshotCloneDependencyEnabledPtr == nil {
		return r
	}
	r = *o.SnapshotCloneDependencyEnabledPtr
	return r
}

// SetSnapshotCloneDependencyEnabled is a fluent style 'setter' method that can be chained
func (o *VolumeSnapshotAttributesType) SetSnapshotCloneDependencyEnabled(newValue bool) *VolumeSnapshotAttributesType {
	o.SnapshotCloneDependencyEnabledPtr = &newValue
	return o
}

// SnapshotCount is a 'getter' method
func (o *VolumeSnapshotAttributesType) SnapshotCount() int {
	var r int
	if o.SnapshotCountPtr == nil {
		return r
	}
	r = *o.SnapshotCountPtr
	return r
}

// SetSnapshotCount is a fluent style 'setter' method that can be chained
func (o *VolumeSnapshotAttributesType) SetSnapshotCount(newValue int) *VolumeSnapshotAttributesType {
	o.SnapshotCountPtr = &newValue
	return o
}

// SnapshotPolicy is a 'getter' method
func (o *VolumeSnapshotAttributesType) SnapshotPolicy() string {
	var r string
	if o.SnapshotPolicyPtr == nil {
		return r
	}
	r = *o.SnapshotPolicyPtr
	return r
}

// SetSnapshotPolicy is a fluent style 'setter' method that can be chained
func (o *VolumeSnapshotAttributesType) SetSnapshotPolicy(newValue string) *VolumeSnapshotAttributesType {
	o.SnapshotPolicyPtr = &newValue
	return o
}
