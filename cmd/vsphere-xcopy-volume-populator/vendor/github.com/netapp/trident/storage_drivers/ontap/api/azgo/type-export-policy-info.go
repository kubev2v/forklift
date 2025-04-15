// Code generated automatically. DO NOT EDIT.
// Copyright 2022 NetApp, Inc. All Rights Reserved.

package azgo

import (
	"encoding/xml"
	log "github.com/sirupsen/logrus"
	"reflect"
)

// ExportPolicyInfoType is a structure to represent a export-policy-info ZAPI object
type ExportPolicyInfoType struct {
	XMLName       xml.Name              `xml:"export-policy-info"`
	PolicyIdPtr   *int                  `xml:"policy-id"`
	PolicyNamePtr *ExportPolicyNameType `xml:"policy-name"`
	VserverPtr    *string               `xml:"vserver"`
}

// NewExportPolicyInfoType is a factory method for creating new instances of ExportPolicyInfoType objects
func NewExportPolicyInfoType() *ExportPolicyInfoType {
	return &ExportPolicyInfoType{}
}

// ToXML converts this object into an xml string representation
func (o *ExportPolicyInfoType) ToXML() (string, error) {
	output, err := xml.MarshalIndent(o, " ", "    ")
	if err != nil {
		log.Errorf("error: %v", err)
	}
	return string(output), err
}

// String returns a string representation of this object's fields and implements the Stringer interface
func (o ExportPolicyInfoType) String() string {
	return ToString(reflect.ValueOf(o))
}

// PolicyId is a 'getter' method
func (o *ExportPolicyInfoType) PolicyId() int {
	var r int
	if o.PolicyIdPtr == nil {
		return r
	}
	r = *o.PolicyIdPtr
	return r
}

// SetPolicyId is a fluent style 'setter' method that can be chained
func (o *ExportPolicyInfoType) SetPolicyId(newValue int) *ExportPolicyInfoType {
	o.PolicyIdPtr = &newValue
	return o
}

// PolicyName is a 'getter' method
func (o *ExportPolicyInfoType) PolicyName() ExportPolicyNameType {
	var r ExportPolicyNameType
	if o.PolicyNamePtr == nil {
		return r
	}
	r = *o.PolicyNamePtr
	return r
}

// SetPolicyName is a fluent style 'setter' method that can be chained
func (o *ExportPolicyInfoType) SetPolicyName(newValue ExportPolicyNameType) *ExportPolicyInfoType {
	o.PolicyNamePtr = &newValue
	return o
}

// Vserver is a 'getter' method
func (o *ExportPolicyInfoType) Vserver() string {
	var r string
	if o.VserverPtr == nil {
		return r
	}
	r = *o.VserverPtr
	return r
}

// SetVserver is a fluent style 'setter' method that can be chained
func (o *ExportPolicyInfoType) SetVserver(newValue string) *ExportPolicyInfoType {
	o.VserverPtr = &newValue
	return o
}
