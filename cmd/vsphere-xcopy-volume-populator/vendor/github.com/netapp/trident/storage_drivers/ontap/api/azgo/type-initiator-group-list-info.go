// Code generated automatically. DO NOT EDIT.
// Copyright 2022 NetApp, Inc. All Rights Reserved.

package azgo

import (
	"encoding/xml"
	log "github.com/sirupsen/logrus"
	"reflect"
)

// InitiatorGroupListInfoType is a structure to represent a initiator-group-list-info ZAPI object
type InitiatorGroupListInfoType struct {
	XMLName               xml.Name `xml:"initiator-group-list-info"`
	InitiatorGroupNamePtr *string  `xml:"initiator-group-name"`
}

// NewInitiatorGroupListInfoType is a factory method for creating new instances of InitiatorGroupListInfoType objects
func NewInitiatorGroupListInfoType() *InitiatorGroupListInfoType {
	return &InitiatorGroupListInfoType{}
}

// ToXML converts this object into an xml string representation
func (o *InitiatorGroupListInfoType) ToXML() (string, error) {
	output, err := xml.MarshalIndent(o, " ", "    ")
	if err != nil {
		log.Errorf("error: %v", err)
	}
	return string(output), err
}

// String returns a string representation of this object's fields and implements the Stringer interface
func (o InitiatorGroupListInfoType) String() string {
	return ToString(reflect.ValueOf(o))
}

// InitiatorGroupName is a 'getter' method
func (o *InitiatorGroupListInfoType) InitiatorGroupName() string {
	var r string
	if o.InitiatorGroupNamePtr == nil {
		return r
	}
	r = *o.InitiatorGroupNamePtr
	return r
}

// SetInitiatorGroupName is a fluent style 'setter' method that can be chained
func (o *InitiatorGroupListInfoType) SetInitiatorGroupName(newValue string) *InitiatorGroupListInfoType {
	o.InitiatorGroupNamePtr = &newValue
	return o
}
