// Code generated automatically. DO NOT EDIT.
// Copyright 2022 NetApp, Inc. All Rights Reserved.

package azgo

import (
	"encoding/xml"
	log "github.com/sirupsen/logrus"
	"reflect"
)

// VolumeFlexcacheAttributesType is a structure to represent a volume-flexcache-attributes ZAPI object
type VolumeFlexcacheAttributesType struct {
	XMLName        xml.Name         `xml:"volume-flexcache-attributes"`
	CachePolicyPtr *CachePolicyType `xml:"cache-policy"`
	FillPolicyPtr  *CachePolicyType `xml:"fill-policy"`
	MinReservePtr  *SizeType        `xml:"min-reserve"`
	OriginPtr      *VolumeNameType  `xml:"origin"`
}

// NewVolumeFlexcacheAttributesType is a factory method for creating new instances of VolumeFlexcacheAttributesType objects
func NewVolumeFlexcacheAttributesType() *VolumeFlexcacheAttributesType {
	return &VolumeFlexcacheAttributesType{}
}

// ToXML converts this object into an xml string representation
func (o *VolumeFlexcacheAttributesType) ToXML() (string, error) {
	output, err := xml.MarshalIndent(o, " ", "    ")
	if err != nil {
		log.Errorf("error: %v", err)
	}
	return string(output), err
}

// String returns a string representation of this object's fields and implements the Stringer interface
func (o VolumeFlexcacheAttributesType) String() string {
	return ToString(reflect.ValueOf(o))
}

// CachePolicy is a 'getter' method
func (o *VolumeFlexcacheAttributesType) CachePolicy() CachePolicyType {
	var r CachePolicyType
	if o.CachePolicyPtr == nil {
		return r
	}
	r = *o.CachePolicyPtr
	return r
}

// SetCachePolicy is a fluent style 'setter' method that can be chained
func (o *VolumeFlexcacheAttributesType) SetCachePolicy(newValue CachePolicyType) *VolumeFlexcacheAttributesType {
	o.CachePolicyPtr = &newValue
	return o
}

// FillPolicy is a 'getter' method
func (o *VolumeFlexcacheAttributesType) FillPolicy() CachePolicyType {
	var r CachePolicyType
	if o.FillPolicyPtr == nil {
		return r
	}
	r = *o.FillPolicyPtr
	return r
}

// SetFillPolicy is a fluent style 'setter' method that can be chained
func (o *VolumeFlexcacheAttributesType) SetFillPolicy(newValue CachePolicyType) *VolumeFlexcacheAttributesType {
	o.FillPolicyPtr = &newValue
	return o
}

// MinReserve is a 'getter' method
func (o *VolumeFlexcacheAttributesType) MinReserve() SizeType {
	var r SizeType
	if o.MinReservePtr == nil {
		return r
	}
	r = *o.MinReservePtr
	return r
}

// SetMinReserve is a fluent style 'setter' method that can be chained
func (o *VolumeFlexcacheAttributesType) SetMinReserve(newValue SizeType) *VolumeFlexcacheAttributesType {
	o.MinReservePtr = &newValue
	return o
}

// Origin is a 'getter' method
func (o *VolumeFlexcacheAttributesType) Origin() VolumeNameType {
	var r VolumeNameType
	if o.OriginPtr == nil {
		return r
	}
	r = *o.OriginPtr
	return r
}

// SetOrigin is a fluent style 'setter' method that can be chained
func (o *VolumeFlexcacheAttributesType) SetOrigin(newValue VolumeNameType) *VolumeFlexcacheAttributesType {
	o.OriginPtr = &newValue
	return o
}
