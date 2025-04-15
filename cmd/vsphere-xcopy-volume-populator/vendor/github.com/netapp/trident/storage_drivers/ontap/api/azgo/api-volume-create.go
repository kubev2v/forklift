// Code generated automatically. DO NOT EDIT.
// Copyright 2022 NetApp, Inc. All Rights Reserved.

package azgo

import (
	"encoding/xml"
	log "github.com/sirupsen/logrus"
	"reflect"
)

// VolumeCreateRequest is a structure to represent a volume-create Request ZAPI object
type VolumeCreateRequest struct {
	XMLName                         xml.Name `xml:"volume-create"`
	AntivirusOnAccessPolicyPtr      *string  `xml:"antivirus-on-access-policy"`
	CacheRetentionPriorityPtr       *string  `xml:"cache-retention-priority"`
	CachingPolicyPtr                *string  `xml:"caching-policy"`
	ConstituentRolePtr              *string  `xml:"constituent-role"`
	ContainingAggrNamePtr           *string  `xml:"containing-aggr-name"`
	EfficiencyPolicyPtr             *string  `xml:"efficiency-policy"`
	EncryptPtr                      *bool    `xml:"encrypt"`
	ExcludedFromAutobalancePtr      *bool    `xml:"excluded-from-autobalance"`
	ExportPolicyPtr                 *string  `xml:"export-policy"`
	ExtentSizePtr                   *string  `xml:"extent-size"`
	FlexcacheCachePolicyPtr         *string  `xml:"flexcache-cache-policy"`
	FlexcacheFillPolicyPtr          *string  `xml:"flexcache-fill-policy"`
	FlexcacheOriginVolumeNamePtr    *string  `xml:"flexcache-origin-volume-name"`
	GroupIdPtr                      *int     `xml:"group-id"`
	IsJunctionActivePtr             *bool    `xml:"is-junction-active"`
	IsNvfailEnabledPtr              *string  `xml:"is-nvfail-enabled"`
	IsVserverRootPtr                *bool    `xml:"is-vserver-root"`
	JunctionPathPtr                 *string  `xml:"junction-path"`
	LanguageCodePtr                 *string  `xml:"language-code"`
	MaxDirSizePtr                   *int     `xml:"max-dir-size"`
	MaxWriteAllocBlocksPtr          *int     `xml:"max-write-alloc-blocks"`
	PercentageSnapshotReservePtr    *int     `xml:"percentage-snapshot-reserve"`
	QosAdaptivePolicyGroupNamePtr   *string  `xml:"qos-adaptive-policy-group-name"`
	QosPolicyGroupNamePtr           *string  `xml:"qos-policy-group-name"`
	SizePtr                         *string  `xml:"size"`
	SnapshotPolicyPtr               *string  `xml:"snapshot-policy"`
	SpaceReservePtr                 *string  `xml:"space-reserve"`
	SpaceSloPtr                     *string  `xml:"space-slo"`
	StorageServicePtr               *string  `xml:"storage-service"`
	StripeAlgorithmPtr              *string  `xml:"stripe-algorithm"`
	StripeConcurrencyPtr            *string  `xml:"stripe-concurrency"`
	StripeConstituentVolumeCountPtr *int     `xml:"stripe-constituent-volume-count"`
	StripeOptimizePtr               *string  `xml:"stripe-optimize"`
	StripeWidthPtr                  *int     `xml:"stripe-width"`
	TieringPolicyPtr                *string  `xml:"tiering-policy"`
	UnixPermissionsPtr              *string  `xml:"unix-permissions"`
	UserIdPtr                       *int     `xml:"user-id"`
	VmAlignSectorPtr                *int     `xml:"vm-align-sector"`
	VmAlignSuffixPtr                *string  `xml:"vm-align-suffix"`
	VolumePtr                       *string  `xml:"volume"`
	VolumeCommentPtr                *string  `xml:"volume-comment"`
	VolumeSecurityStylePtr          *string  `xml:"volume-security-style"`
	VolumeStatePtr                  *string  `xml:"volume-state"`
	VolumeTypePtr                   *string  `xml:"volume-type"`
	VserverDrProtectionPtr          *string  `xml:"vserver-dr-protection"`
}

// VolumeCreateResponse is a structure to represent a volume-create Response ZAPI object
type VolumeCreateResponse struct {
	XMLName         xml.Name                   `xml:"netapp"`
	ResponseVersion string                     `xml:"version,attr"`
	ResponseXmlns   string                     `xml:"xmlns,attr"`
	Result          VolumeCreateResponseResult `xml:"results"`
}

// NewVolumeCreateResponse is a factory method for creating new instances of VolumeCreateResponse objects
func NewVolumeCreateResponse() *VolumeCreateResponse {
	return &VolumeCreateResponse{}
}

// String returns a string representation of this object's fields and implements the Stringer interface
func (o VolumeCreateResponse) String() string {
	return ToString(reflect.ValueOf(o))
}

// ToXML converts this object into an xml string representation
func (o *VolumeCreateResponse) ToXML() (string, error) {
	output, err := xml.MarshalIndent(o, " ", "    ")
	if err != nil {
		log.Errorf("error: %v", err)
	}
	return string(output), err
}

// VolumeCreateResponseResult is a structure to represent a volume-create Response Result ZAPI object
type VolumeCreateResponseResult struct {
	XMLName          xml.Name `xml:"results"`
	ResultStatusAttr string   `xml:"status,attr"`
	ResultReasonAttr string   `xml:"reason,attr"`
	ResultErrnoAttr  string   `xml:"errno,attr"`
}

// NewVolumeCreateRequest is a factory method for creating new instances of VolumeCreateRequest objects
func NewVolumeCreateRequest() *VolumeCreateRequest {
	return &VolumeCreateRequest{}
}

// NewVolumeCreateResponseResult is a factory method for creating new instances of VolumeCreateResponseResult objects
func NewVolumeCreateResponseResult() *VolumeCreateResponseResult {
	return &VolumeCreateResponseResult{}
}

// ToXML converts this object into an xml string representation
func (o *VolumeCreateRequest) ToXML() (string, error) {
	output, err := xml.MarshalIndent(o, " ", "    ")
	if err != nil {
		log.Errorf("error: %v", err)
	}
	return string(output), err
}

// ToXML converts this object into an xml string representation
func (o *VolumeCreateResponseResult) ToXML() (string, error) {
	output, err := xml.MarshalIndent(o, " ", "    ")
	if err != nil {
		log.Errorf("error: %v", err)
	}
	return string(output), err
}

// String returns a string representation of this object's fields and implements the Stringer interface
func (o VolumeCreateRequest) String() string {
	return ToString(reflect.ValueOf(o))
}

// String returns a string representation of this object's fields and implements the Stringer interface
func (o VolumeCreateResponseResult) String() string {
	return ToString(reflect.ValueOf(o))
}

// ExecuteUsing converts this object to a ZAPI XML representation and uses the supplied ZapiRunner to send to a filer

func (o *VolumeCreateRequest) ExecuteUsing(zr *ZapiRunner) (*VolumeCreateResponse, error) {
	return o.executeWithoutIteration(zr)
}

// executeWithoutIteration converts this object to a ZAPI XML representation and uses the supplied ZapiRunner to send to a filer

func (o *VolumeCreateRequest) executeWithoutIteration(zr *ZapiRunner) (*VolumeCreateResponse, error) {
	result, err := zr.ExecuteUsing(o, "VolumeCreateRequest", NewVolumeCreateResponse())
	if result == nil {
		return nil, err
	}
	return result.(*VolumeCreateResponse), err
}

// AntivirusOnAccessPolicy is a 'getter' method
func (o *VolumeCreateRequest) AntivirusOnAccessPolicy() string {
	var r string
	if o.AntivirusOnAccessPolicyPtr == nil {
		return r
	}
	r = *o.AntivirusOnAccessPolicyPtr
	return r
}

// SetAntivirusOnAccessPolicy is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetAntivirusOnAccessPolicy(newValue string) *VolumeCreateRequest {
	o.AntivirusOnAccessPolicyPtr = &newValue
	return o
}

// CacheRetentionPriority is a 'getter' method
func (o *VolumeCreateRequest) CacheRetentionPriority() string {
	var r string
	if o.CacheRetentionPriorityPtr == nil {
		return r
	}
	r = *o.CacheRetentionPriorityPtr
	return r
}

// SetCacheRetentionPriority is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetCacheRetentionPriority(newValue string) *VolumeCreateRequest {
	o.CacheRetentionPriorityPtr = &newValue
	return o
}

// CachingPolicy is a 'getter' method
func (o *VolumeCreateRequest) CachingPolicy() string {
	var r string
	if o.CachingPolicyPtr == nil {
		return r
	}
	r = *o.CachingPolicyPtr
	return r
}

// SetCachingPolicy is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetCachingPolicy(newValue string) *VolumeCreateRequest {
	o.CachingPolicyPtr = &newValue
	return o
}

// ConstituentRole is a 'getter' method
func (o *VolumeCreateRequest) ConstituentRole() string {
	var r string
	if o.ConstituentRolePtr == nil {
		return r
	}
	r = *o.ConstituentRolePtr
	return r
}

// SetConstituentRole is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetConstituentRole(newValue string) *VolumeCreateRequest {
	o.ConstituentRolePtr = &newValue
	return o
}

// ContainingAggrName is a 'getter' method
func (o *VolumeCreateRequest) ContainingAggrName() string {
	var r string
	if o.ContainingAggrNamePtr == nil {
		return r
	}
	r = *o.ContainingAggrNamePtr
	return r
}

// SetContainingAggrName is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetContainingAggrName(newValue string) *VolumeCreateRequest {
	o.ContainingAggrNamePtr = &newValue
	return o
}

// EfficiencyPolicy is a 'getter' method
func (o *VolumeCreateRequest) EfficiencyPolicy() string {
	var r string
	if o.EfficiencyPolicyPtr == nil {
		return r
	}
	r = *o.EfficiencyPolicyPtr
	return r
}

// SetEfficiencyPolicy is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetEfficiencyPolicy(newValue string) *VolumeCreateRequest {
	o.EfficiencyPolicyPtr = &newValue
	return o
}

// Encrypt is a 'getter' method
func (o *VolumeCreateRequest) Encrypt() bool {
	var r bool
	if o.EncryptPtr == nil {
		return r
	}
	r = *o.EncryptPtr
	return r
}

// SetEncrypt is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetEncrypt(newValue bool) *VolumeCreateRequest {
	o.EncryptPtr = &newValue
	return o
}

// ExcludedFromAutobalance is a 'getter' method
func (o *VolumeCreateRequest) ExcludedFromAutobalance() bool {
	var r bool
	if o.ExcludedFromAutobalancePtr == nil {
		return r
	}
	r = *o.ExcludedFromAutobalancePtr
	return r
}

// SetExcludedFromAutobalance is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetExcludedFromAutobalance(newValue bool) *VolumeCreateRequest {
	o.ExcludedFromAutobalancePtr = &newValue
	return o
}

// ExportPolicy is a 'getter' method
func (o *VolumeCreateRequest) ExportPolicy() string {
	var r string
	if o.ExportPolicyPtr == nil {
		return r
	}
	r = *o.ExportPolicyPtr
	return r
}

// SetExportPolicy is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetExportPolicy(newValue string) *VolumeCreateRequest {
	o.ExportPolicyPtr = &newValue
	return o
}

// ExtentSize is a 'getter' method
func (o *VolumeCreateRequest) ExtentSize() string {
	var r string
	if o.ExtentSizePtr == nil {
		return r
	}
	r = *o.ExtentSizePtr
	return r
}

// SetExtentSize is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetExtentSize(newValue string) *VolumeCreateRequest {
	o.ExtentSizePtr = &newValue
	return o
}

// FlexcacheCachePolicy is a 'getter' method
func (o *VolumeCreateRequest) FlexcacheCachePolicy() string {
	var r string
	if o.FlexcacheCachePolicyPtr == nil {
		return r
	}
	r = *o.FlexcacheCachePolicyPtr
	return r
}

// SetFlexcacheCachePolicy is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetFlexcacheCachePolicy(newValue string) *VolumeCreateRequest {
	o.FlexcacheCachePolicyPtr = &newValue
	return o
}

// FlexcacheFillPolicy is a 'getter' method
func (o *VolumeCreateRequest) FlexcacheFillPolicy() string {
	var r string
	if o.FlexcacheFillPolicyPtr == nil {
		return r
	}
	r = *o.FlexcacheFillPolicyPtr
	return r
}

// SetFlexcacheFillPolicy is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetFlexcacheFillPolicy(newValue string) *VolumeCreateRequest {
	o.FlexcacheFillPolicyPtr = &newValue
	return o
}

// FlexcacheOriginVolumeName is a 'getter' method
func (o *VolumeCreateRequest) FlexcacheOriginVolumeName() string {
	var r string
	if o.FlexcacheOriginVolumeNamePtr == nil {
		return r
	}
	r = *o.FlexcacheOriginVolumeNamePtr
	return r
}

// SetFlexcacheOriginVolumeName is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetFlexcacheOriginVolumeName(newValue string) *VolumeCreateRequest {
	o.FlexcacheOriginVolumeNamePtr = &newValue
	return o
}

// GroupId is a 'getter' method
func (o *VolumeCreateRequest) GroupId() int {
	var r int
	if o.GroupIdPtr == nil {
		return r
	}
	r = *o.GroupIdPtr
	return r
}

// SetGroupId is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetGroupId(newValue int) *VolumeCreateRequest {
	o.GroupIdPtr = &newValue
	return o
}

// IsJunctionActive is a 'getter' method
func (o *VolumeCreateRequest) IsJunctionActive() bool {
	var r bool
	if o.IsJunctionActivePtr == nil {
		return r
	}
	r = *o.IsJunctionActivePtr
	return r
}

// SetIsJunctionActive is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetIsJunctionActive(newValue bool) *VolumeCreateRequest {
	o.IsJunctionActivePtr = &newValue
	return o
}

// IsNvfailEnabled is a 'getter' method
func (o *VolumeCreateRequest) IsNvfailEnabled() string {
	var r string
	if o.IsNvfailEnabledPtr == nil {
		return r
	}
	r = *o.IsNvfailEnabledPtr
	return r
}

// SetIsNvfailEnabled is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetIsNvfailEnabled(newValue string) *VolumeCreateRequest {
	o.IsNvfailEnabledPtr = &newValue
	return o
}

// IsVserverRoot is a 'getter' method
func (o *VolumeCreateRequest) IsVserverRoot() bool {
	var r bool
	if o.IsVserverRootPtr == nil {
		return r
	}
	r = *o.IsVserverRootPtr
	return r
}

// SetIsVserverRoot is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetIsVserverRoot(newValue bool) *VolumeCreateRequest {
	o.IsVserverRootPtr = &newValue
	return o
}

// JunctionPath is a 'getter' method
func (o *VolumeCreateRequest) JunctionPath() string {
	var r string
	if o.JunctionPathPtr == nil {
		return r
	}
	r = *o.JunctionPathPtr
	return r
}

// SetJunctionPath is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetJunctionPath(newValue string) *VolumeCreateRequest {
	o.JunctionPathPtr = &newValue
	return o
}

// LanguageCode is a 'getter' method
func (o *VolumeCreateRequest) LanguageCode() string {
	var r string
	if o.LanguageCodePtr == nil {
		return r
	}
	r = *o.LanguageCodePtr
	return r
}

// SetLanguageCode is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetLanguageCode(newValue string) *VolumeCreateRequest {
	o.LanguageCodePtr = &newValue
	return o
}

// MaxDirSize is a 'getter' method
func (o *VolumeCreateRequest) MaxDirSize() int {
	var r int
	if o.MaxDirSizePtr == nil {
		return r
	}
	r = *o.MaxDirSizePtr
	return r
}

// SetMaxDirSize is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetMaxDirSize(newValue int) *VolumeCreateRequest {
	o.MaxDirSizePtr = &newValue
	return o
}

// MaxWriteAllocBlocks is a 'getter' method
func (o *VolumeCreateRequest) MaxWriteAllocBlocks() int {
	var r int
	if o.MaxWriteAllocBlocksPtr == nil {
		return r
	}
	r = *o.MaxWriteAllocBlocksPtr
	return r
}

// SetMaxWriteAllocBlocks is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetMaxWriteAllocBlocks(newValue int) *VolumeCreateRequest {
	o.MaxWriteAllocBlocksPtr = &newValue
	return o
}

// PercentageSnapshotReserve is a 'getter' method
func (o *VolumeCreateRequest) PercentageSnapshotReserve() int {
	var r int
	if o.PercentageSnapshotReservePtr == nil {
		return r
	}
	r = *o.PercentageSnapshotReservePtr
	return r
}

// SetPercentageSnapshotReserve is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetPercentageSnapshotReserve(newValue int) *VolumeCreateRequest {
	o.PercentageSnapshotReservePtr = &newValue
	return o
}

// QosAdaptivePolicyGroupName is a 'getter' method
func (o *VolumeCreateRequest) QosAdaptivePolicyGroupName() string {
	var r string
	if o.QosAdaptivePolicyGroupNamePtr == nil {
		return r
	}
	r = *o.QosAdaptivePolicyGroupNamePtr
	return r
}

// SetQosAdaptivePolicyGroupName is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetQosAdaptivePolicyGroupName(newValue string) *VolumeCreateRequest {
	o.QosAdaptivePolicyGroupNamePtr = &newValue
	return o
}

// QosPolicyGroupName is a 'getter' method
func (o *VolumeCreateRequest) QosPolicyGroupName() string {
	var r string
	if o.QosPolicyGroupNamePtr == nil {
		return r
	}
	r = *o.QosPolicyGroupNamePtr
	return r
}

// SetQosPolicyGroupName is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetQosPolicyGroupName(newValue string) *VolumeCreateRequest {
	o.QosPolicyGroupNamePtr = &newValue
	return o
}

// Size is a 'getter' method
func (o *VolumeCreateRequest) Size() string {
	var r string
	if o.SizePtr == nil {
		return r
	}
	r = *o.SizePtr
	return r
}

// SetSize is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetSize(newValue string) *VolumeCreateRequest {
	o.SizePtr = &newValue
	return o
}

// SnapshotPolicy is a 'getter' method
func (o *VolumeCreateRequest) SnapshotPolicy() string {
	var r string
	if o.SnapshotPolicyPtr == nil {
		return r
	}
	r = *o.SnapshotPolicyPtr
	return r
}

// SetSnapshotPolicy is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetSnapshotPolicy(newValue string) *VolumeCreateRequest {
	o.SnapshotPolicyPtr = &newValue
	return o
}

// SpaceReserve is a 'getter' method
func (o *VolumeCreateRequest) SpaceReserve() string {
	var r string
	if o.SpaceReservePtr == nil {
		return r
	}
	r = *o.SpaceReservePtr
	return r
}

// SetSpaceReserve is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetSpaceReserve(newValue string) *VolumeCreateRequest {
	o.SpaceReservePtr = &newValue
	return o
}

// SpaceSlo is a 'getter' method
func (o *VolumeCreateRequest) SpaceSlo() string {
	var r string
	if o.SpaceSloPtr == nil {
		return r
	}
	r = *o.SpaceSloPtr
	return r
}

// SetSpaceSlo is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetSpaceSlo(newValue string) *VolumeCreateRequest {
	o.SpaceSloPtr = &newValue
	return o
}

// StorageService is a 'getter' method
func (o *VolumeCreateRequest) StorageService() string {
	var r string
	if o.StorageServicePtr == nil {
		return r
	}
	r = *o.StorageServicePtr
	return r
}

// SetStorageService is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetStorageService(newValue string) *VolumeCreateRequest {
	o.StorageServicePtr = &newValue
	return o
}

// StripeAlgorithm is a 'getter' method
func (o *VolumeCreateRequest) StripeAlgorithm() string {
	var r string
	if o.StripeAlgorithmPtr == nil {
		return r
	}
	r = *o.StripeAlgorithmPtr
	return r
}

// SetStripeAlgorithm is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetStripeAlgorithm(newValue string) *VolumeCreateRequest {
	o.StripeAlgorithmPtr = &newValue
	return o
}

// StripeConcurrency is a 'getter' method
func (o *VolumeCreateRequest) StripeConcurrency() string {
	var r string
	if o.StripeConcurrencyPtr == nil {
		return r
	}
	r = *o.StripeConcurrencyPtr
	return r
}

// SetStripeConcurrency is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetStripeConcurrency(newValue string) *VolumeCreateRequest {
	o.StripeConcurrencyPtr = &newValue
	return o
}

// StripeConstituentVolumeCount is a 'getter' method
func (o *VolumeCreateRequest) StripeConstituentVolumeCount() int {
	var r int
	if o.StripeConstituentVolumeCountPtr == nil {
		return r
	}
	r = *o.StripeConstituentVolumeCountPtr
	return r
}

// SetStripeConstituentVolumeCount is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetStripeConstituentVolumeCount(newValue int) *VolumeCreateRequest {
	o.StripeConstituentVolumeCountPtr = &newValue
	return o
}

// StripeOptimize is a 'getter' method
func (o *VolumeCreateRequest) StripeOptimize() string {
	var r string
	if o.StripeOptimizePtr == nil {
		return r
	}
	r = *o.StripeOptimizePtr
	return r
}

// SetStripeOptimize is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetStripeOptimize(newValue string) *VolumeCreateRequest {
	o.StripeOptimizePtr = &newValue
	return o
}

// StripeWidth is a 'getter' method
func (o *VolumeCreateRequest) StripeWidth() int {
	var r int
	if o.StripeWidthPtr == nil {
		return r
	}
	r = *o.StripeWidthPtr
	return r
}

// SetStripeWidth is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetStripeWidth(newValue int) *VolumeCreateRequest {
	o.StripeWidthPtr = &newValue
	return o
}

// TieringPolicy is a 'getter' method
func (o *VolumeCreateRequest) TieringPolicy() string {
	var r string
	if o.TieringPolicyPtr == nil {
		return r
	}
	r = *o.TieringPolicyPtr
	return r
}

// SetTieringPolicy is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetTieringPolicy(newValue string) *VolumeCreateRequest {
	o.TieringPolicyPtr = &newValue
	return o
}

// UnixPermissions is a 'getter' method
func (o *VolumeCreateRequest) UnixPermissions() string {
	var r string
	if o.UnixPermissionsPtr == nil {
		return r
	}
	r = *o.UnixPermissionsPtr
	return r
}

// SetUnixPermissions is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetUnixPermissions(newValue string) *VolumeCreateRequest {
	o.UnixPermissionsPtr = &newValue
	return o
}

// UserId is a 'getter' method
func (o *VolumeCreateRequest) UserId() int {
	var r int
	if o.UserIdPtr == nil {
		return r
	}
	r = *o.UserIdPtr
	return r
}

// SetUserId is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetUserId(newValue int) *VolumeCreateRequest {
	o.UserIdPtr = &newValue
	return o
}

// VmAlignSector is a 'getter' method
func (o *VolumeCreateRequest) VmAlignSector() int {
	var r int
	if o.VmAlignSectorPtr == nil {
		return r
	}
	r = *o.VmAlignSectorPtr
	return r
}

// SetVmAlignSector is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetVmAlignSector(newValue int) *VolumeCreateRequest {
	o.VmAlignSectorPtr = &newValue
	return o
}

// VmAlignSuffix is a 'getter' method
func (o *VolumeCreateRequest) VmAlignSuffix() string {
	var r string
	if o.VmAlignSuffixPtr == nil {
		return r
	}
	r = *o.VmAlignSuffixPtr
	return r
}

// SetVmAlignSuffix is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetVmAlignSuffix(newValue string) *VolumeCreateRequest {
	o.VmAlignSuffixPtr = &newValue
	return o
}

// Volume is a 'getter' method
func (o *VolumeCreateRequest) Volume() string {
	var r string
	if o.VolumePtr == nil {
		return r
	}
	r = *o.VolumePtr
	return r
}

// SetVolume is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetVolume(newValue string) *VolumeCreateRequest {
	o.VolumePtr = &newValue
	return o
}

// VolumeComment is a 'getter' method
func (o *VolumeCreateRequest) VolumeComment() string {
	var r string
	if o.VolumeCommentPtr == nil {
		return r
	}
	r = *o.VolumeCommentPtr
	return r
}

// SetVolumeComment is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetVolumeComment(newValue string) *VolumeCreateRequest {
	o.VolumeCommentPtr = &newValue
	return o
}

// VolumeSecurityStyle is a 'getter' method
func (o *VolumeCreateRequest) VolumeSecurityStyle() string {
	var r string
	if o.VolumeSecurityStylePtr == nil {
		return r
	}
	r = *o.VolumeSecurityStylePtr
	return r
}

// SetVolumeSecurityStyle is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetVolumeSecurityStyle(newValue string) *VolumeCreateRequest {
	o.VolumeSecurityStylePtr = &newValue
	return o
}

// VolumeState is a 'getter' method
func (o *VolumeCreateRequest) VolumeState() string {
	var r string
	if o.VolumeStatePtr == nil {
		return r
	}
	r = *o.VolumeStatePtr
	return r
}

// SetVolumeState is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetVolumeState(newValue string) *VolumeCreateRequest {
	o.VolumeStatePtr = &newValue
	return o
}

// VolumeType is a 'getter' method
func (o *VolumeCreateRequest) VolumeType() string {
	var r string
	if o.VolumeTypePtr == nil {
		return r
	}
	r = *o.VolumeTypePtr
	return r
}

// SetVolumeType is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetVolumeType(newValue string) *VolumeCreateRequest {
	o.VolumeTypePtr = &newValue
	return o
}

// VserverDrProtection is a 'getter' method
func (o *VolumeCreateRequest) VserverDrProtection() string {
	var r string
	if o.VserverDrProtectionPtr == nil {
		return r
	}
	r = *o.VserverDrProtectionPtr
	return r
}

// SetVserverDrProtection is a fluent style 'setter' method that can be chained
func (o *VolumeCreateRequest) SetVserverDrProtection(newValue string) *VolumeCreateRequest {
	o.VserverDrProtectionPtr = &newValue
	return o
}
