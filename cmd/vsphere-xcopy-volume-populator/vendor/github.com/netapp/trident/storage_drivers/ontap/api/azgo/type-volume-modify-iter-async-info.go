// Code generated automatically. DO NOT EDIT.
// Copyright 2022 NetApp, Inc. All Rights Reserved.

package azgo

import (
	"encoding/xml"
	log "github.com/sirupsen/logrus"
	"reflect"
)

// VolumeModifyIterAsyncInfoType is a structure to represent a volume-modify-iter-async-info ZAPI object
type VolumeModifyIterAsyncInfoType struct {
	XMLName         xml.Name                                `xml:"volume-modify-iter-async-info"`
	ErrorCodePtr    *int                                    `xml:"error-code"`
	ErrorMessagePtr *string                                 `xml:"error-message"`
	JobidPtr        *int                                    `xml:"jobid"`
	StatusPtr       *string                                 `xml:"status"`
	VolumeKeyPtr    *VolumeModifyIterAsyncInfoTypeVolumeKey `xml:"volume-key"`
	// work in progress
}

// NewVolumeModifyIterAsyncInfoType is a factory method for creating new instances of VolumeModifyIterAsyncInfoType objects
func NewVolumeModifyIterAsyncInfoType() *VolumeModifyIterAsyncInfoType {
	return &VolumeModifyIterAsyncInfoType{}
}

// ToXML converts this object into an xml string representation
func (o *VolumeModifyIterAsyncInfoType) ToXML() (string, error) {
	output, err := xml.MarshalIndent(o, " ", "    ")
	if err != nil {
		log.Errorf("error: %v", err)
	}
	return string(output), err
}

// String returns a string representation of this object's fields and implements the Stringer interface
func (o VolumeModifyIterAsyncInfoType) String() string {
	return ToString(reflect.ValueOf(o))
}

// ErrorCode is a 'getter' method
func (o *VolumeModifyIterAsyncInfoType) ErrorCode() int {
	var r int
	if o.ErrorCodePtr == nil {
		return r
	}
	r = *o.ErrorCodePtr
	return r
}

// SetErrorCode is a fluent style 'setter' method that can be chained
func (o *VolumeModifyIterAsyncInfoType) SetErrorCode(newValue int) *VolumeModifyIterAsyncInfoType {
	o.ErrorCodePtr = &newValue
	return o
}

// ErrorMessage is a 'getter' method
func (o *VolumeModifyIterAsyncInfoType) ErrorMessage() string {
	var r string
	if o.ErrorMessagePtr == nil {
		return r
	}
	r = *o.ErrorMessagePtr
	return r
}

// SetErrorMessage is a fluent style 'setter' method that can be chained
func (o *VolumeModifyIterAsyncInfoType) SetErrorMessage(newValue string) *VolumeModifyIterAsyncInfoType {
	o.ErrorMessagePtr = &newValue
	return o
}

// Jobid is a 'getter' method
func (o *VolumeModifyIterAsyncInfoType) Jobid() int {
	var r int
	if o.JobidPtr == nil {
		return r
	}
	r = *o.JobidPtr
	return r
}

// SetJobid is a fluent style 'setter' method that can be chained
func (o *VolumeModifyIterAsyncInfoType) SetJobid(newValue int) *VolumeModifyIterAsyncInfoType {
	o.JobidPtr = &newValue
	return o
}

// Status is a 'getter' method
func (o *VolumeModifyIterAsyncInfoType) Status() string {
	var r string
	if o.StatusPtr == nil {
		return r
	}
	r = *o.StatusPtr
	return r
}

// SetStatus is a fluent style 'setter' method that can be chained
func (o *VolumeModifyIterAsyncInfoType) SetStatus(newValue string) *VolumeModifyIterAsyncInfoType {
	o.StatusPtr = &newValue
	return o
}

// VolumeModifyIterAsyncInfoTypeVolumeKey is a wrapper
type VolumeModifyIterAsyncInfoTypeVolumeKey struct {
	XMLName             xml.Name              `xml:"volume-key"`
	VolumeAttributesPtr *VolumeAttributesType `xml:"volume-attributes"`
}

// VolumeAttributes is a 'getter' method
func (o *VolumeModifyIterAsyncInfoTypeVolumeKey) VolumeAttributes() VolumeAttributesType {
	var r VolumeAttributesType
	if o.VolumeAttributesPtr == nil {
		return r
	}
	r = *o.VolumeAttributesPtr
	return r
}

// SetVolumeAttributes is a fluent style 'setter' method that can be chained
func (o *VolumeModifyIterAsyncInfoTypeVolumeKey) SetVolumeAttributes(newValue VolumeAttributesType) *VolumeModifyIterAsyncInfoTypeVolumeKey {
	o.VolumeAttributesPtr = &newValue
	return o
}

// VolumeKey is a 'getter' method
func (o *VolumeModifyIterAsyncInfoType) VolumeKey() VolumeModifyIterAsyncInfoTypeVolumeKey {
	var r VolumeModifyIterAsyncInfoTypeVolumeKey
	if o.VolumeKeyPtr == nil {
		return r
	}
	r = *o.VolumeKeyPtr
	return r
}

// SetVolumeKey is a fluent style 'setter' method that can be chained
func (o *VolumeModifyIterAsyncInfoType) SetVolumeKey(newValue VolumeModifyIterAsyncInfoTypeVolumeKey) *VolumeModifyIterAsyncInfoType {
	o.VolumeKeyPtr = &newValue
	return o
}
