// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
)

// ZappNvme An NVME application.
//
// swagger:model zapp_nvme
type ZappNvme struct {

	// The name of the host OS running the application.
	// Enum: [aix linux vmware windows]
	OsType *string `json:"os_type,omitempty"`

	// rpo
	Rpo *ZappNvmeInlineRpo `json:"rpo,omitempty"`

	// zapp nvme inline components
	// Required: true
	// Max Items: 10
	// Min Items: 1
	ZappNvmeInlineComponents []*ZappNvmeInlineComponentsInlineArrayItem `json:"components"`
}

// Validate validates this zapp nvme
func (m *ZappNvme) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateOsType(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateRpo(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateZappNvmeInlineComponents(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

var zappNvmeTypeOsTypePropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["aix","linux","vmware","windows"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		zappNvmeTypeOsTypePropEnum = append(zappNvmeTypeOsTypePropEnum, v)
	}
}

const (

	// BEGIN DEBUGGING
	// zapp_nvme
	// ZappNvme
	// os_type
	// OsType
	// aix
	// END DEBUGGING
	// ZappNvmeOsTypeAix captures enum value "aix"
	ZappNvmeOsTypeAix string = "aix"

	// BEGIN DEBUGGING
	// zapp_nvme
	// ZappNvme
	// os_type
	// OsType
	// linux
	// END DEBUGGING
	// ZappNvmeOsTypeLinux captures enum value "linux"
	ZappNvmeOsTypeLinux string = "linux"

	// BEGIN DEBUGGING
	// zapp_nvme
	// ZappNvme
	// os_type
	// OsType
	// vmware
	// END DEBUGGING
	// ZappNvmeOsTypeVmware captures enum value "vmware"
	ZappNvmeOsTypeVmware string = "vmware"

	// BEGIN DEBUGGING
	// zapp_nvme
	// ZappNvme
	// os_type
	// OsType
	// windows
	// END DEBUGGING
	// ZappNvmeOsTypeWindows captures enum value "windows"
	ZappNvmeOsTypeWindows string = "windows"
)

// prop value enum
func (m *ZappNvme) validateOsTypeEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, zappNvmeTypeOsTypePropEnum, true); err != nil {
		return err
	}
	return nil
}

func (m *ZappNvme) validateOsType(formats strfmt.Registry) error {
	if swag.IsZero(m.OsType) { // not required
		return nil
	}

	// value enum
	if err := m.validateOsTypeEnum("os_type", "body", *m.OsType); err != nil {
		return err
	}

	return nil
}

func (m *ZappNvme) validateRpo(formats strfmt.Registry) error {
	if swag.IsZero(m.Rpo) { // not required
		return nil
	}

	if m.Rpo != nil {
		if err := m.Rpo.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("rpo")
			}
			return err
		}
	}

	return nil
}

func (m *ZappNvme) validateZappNvmeInlineComponents(formats strfmt.Registry) error {

	if err := validate.Required("components", "body", m.ZappNvmeInlineComponents); err != nil {
		return err
	}

	iZappNvmeInlineComponentsSize := int64(len(m.ZappNvmeInlineComponents))

	if err := validate.MinItems("components", "body", iZappNvmeInlineComponentsSize, 1); err != nil {
		return err
	}

	if err := validate.MaxItems("components", "body", iZappNvmeInlineComponentsSize, 10); err != nil {
		return err
	}

	for i := 0; i < len(m.ZappNvmeInlineComponents); i++ {
		if swag.IsZero(m.ZappNvmeInlineComponents[i]) { // not required
			continue
		}

		if m.ZappNvmeInlineComponents[i] != nil {
			if err := m.ZappNvmeInlineComponents[i].Validate(formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("components" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

// ContextValidate validate this zapp nvme based on the context it is used
func (m *ZappNvme) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidateRpo(ctx, formats); err != nil {
		res = append(res, err)
	}

	if err := m.contextValidateZappNvmeInlineComponents(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ZappNvme) contextValidateRpo(ctx context.Context, formats strfmt.Registry) error {

	if m.Rpo != nil {
		if err := m.Rpo.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("rpo")
			}
			return err
		}
	}

	return nil
}

func (m *ZappNvme) contextValidateZappNvmeInlineComponents(ctx context.Context, formats strfmt.Registry) error {

	for i := 0; i < len(m.ZappNvmeInlineComponents); i++ {

		if m.ZappNvmeInlineComponents[i] != nil {
			if err := m.ZappNvmeInlineComponents[i].ContextValidate(ctx, formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("components" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

// MarshalBinary interface implementation
func (m *ZappNvme) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *ZappNvme) UnmarshalBinary(b []byte) error {
	var res ZappNvme
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}

// ZappNvmeInlineComponentsInlineArrayItem zapp nvme inline components inline array item
//
// swagger:model zapp_nvme_inline_components_inline_array_item
type ZappNvmeInlineComponentsInlineArrayItem struct {

	// The name of the application component.
	// Required: true
	// Max Length: 512
	// Min Length: 1
	Name *string `json:"name"`

	// The number of namespaces in the component.
	// Maximum: 1024
	// Minimum: 1
	NamespaceCount *int64 `json:"namespace_count,omitempty"`

	// The name of the host OS running the application.
	// Enum: [aix linux vmware windows]
	OsType *string `json:"os_type,omitempty"`

	// performance
	Performance *ZappNvmeInlineComponentsInlineArrayItemInlinePerformance `json:"performance,omitempty"`

	// qos
	Qos *ZappNvmeInlineComponentsInlineArrayItemInlineQos `json:"qos,omitempty"`

	// subsystem
	Subsystem *ZappNvmeComponentsSubsystem `json:"subsystem,omitempty"`

	// tiering
	Tiering *ZappNvmeComponentsTiering `json:"tiering,omitempty"`

	// The total size of the component, spread across member namespaces. Usage: {&lt;integer&gt;[KB|MB|GB|TB|PB]}
	TotalSize *int64 `json:"total_size,omitempty"`
}

// Validate validates this zapp nvme inline components inline array item
func (m *ZappNvmeInlineComponentsInlineArrayItem) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateName(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateNamespaceCount(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateOsType(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validatePerformance(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateQos(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateSubsystem(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateTiering(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItem) validateName(formats strfmt.Registry) error {

	if err := validate.Required("name", "body", m.Name); err != nil {
		return err
	}

	if err := validate.MinLength("name", "body", *m.Name, 1); err != nil {
		return err
	}

	if err := validate.MaxLength("name", "body", *m.Name, 512); err != nil {
		return err
	}

	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItem) validateNamespaceCount(formats strfmt.Registry) error {
	if swag.IsZero(m.NamespaceCount) { // not required
		return nil
	}

	if err := validate.MinimumInt("namespace_count", "body", *m.NamespaceCount, 1, false); err != nil {
		return err
	}

	if err := validate.MaximumInt("namespace_count", "body", *m.NamespaceCount, 1024, false); err != nil {
		return err
	}

	return nil
}

var zappNvmeInlineComponentsInlineArrayItemTypeOsTypePropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["aix","linux","vmware","windows"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		zappNvmeInlineComponentsInlineArrayItemTypeOsTypePropEnum = append(zappNvmeInlineComponentsInlineArrayItemTypeOsTypePropEnum, v)
	}
}

const (

	// BEGIN DEBUGGING
	// zapp_nvme_inline_components_inline_array_item
	// ZappNvmeInlineComponentsInlineArrayItem
	// os_type
	// OsType
	// aix
	// END DEBUGGING
	// ZappNvmeInlineComponentsInlineArrayItemOsTypeAix captures enum value "aix"
	ZappNvmeInlineComponentsInlineArrayItemOsTypeAix string = "aix"

	// BEGIN DEBUGGING
	// zapp_nvme_inline_components_inline_array_item
	// ZappNvmeInlineComponentsInlineArrayItem
	// os_type
	// OsType
	// linux
	// END DEBUGGING
	// ZappNvmeInlineComponentsInlineArrayItemOsTypeLinux captures enum value "linux"
	ZappNvmeInlineComponentsInlineArrayItemOsTypeLinux string = "linux"

	// BEGIN DEBUGGING
	// zapp_nvme_inline_components_inline_array_item
	// ZappNvmeInlineComponentsInlineArrayItem
	// os_type
	// OsType
	// vmware
	// END DEBUGGING
	// ZappNvmeInlineComponentsInlineArrayItemOsTypeVmware captures enum value "vmware"
	ZappNvmeInlineComponentsInlineArrayItemOsTypeVmware string = "vmware"

	// BEGIN DEBUGGING
	// zapp_nvme_inline_components_inline_array_item
	// ZappNvmeInlineComponentsInlineArrayItem
	// os_type
	// OsType
	// windows
	// END DEBUGGING
	// ZappNvmeInlineComponentsInlineArrayItemOsTypeWindows captures enum value "windows"
	ZappNvmeInlineComponentsInlineArrayItemOsTypeWindows string = "windows"
)

// prop value enum
func (m *ZappNvmeInlineComponentsInlineArrayItem) validateOsTypeEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, zappNvmeInlineComponentsInlineArrayItemTypeOsTypePropEnum, true); err != nil {
		return err
	}
	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItem) validateOsType(formats strfmt.Registry) error {
	if swag.IsZero(m.OsType) { // not required
		return nil
	}

	// value enum
	if err := m.validateOsTypeEnum("os_type", "body", *m.OsType); err != nil {
		return err
	}

	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItem) validatePerformance(formats strfmt.Registry) error {
	if swag.IsZero(m.Performance) { // not required
		return nil
	}

	if m.Performance != nil {
		if err := m.Performance.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("performance")
			}
			return err
		}
	}

	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItem) validateQos(formats strfmt.Registry) error {
	if swag.IsZero(m.Qos) { // not required
		return nil
	}

	if m.Qos != nil {
		if err := m.Qos.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("qos")
			}
			return err
		}
	}

	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItem) validateSubsystem(formats strfmt.Registry) error {
	if swag.IsZero(m.Subsystem) { // not required
		return nil
	}

	if m.Subsystem != nil {
		if err := m.Subsystem.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("subsystem")
			}
			return err
		}
	}

	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItem) validateTiering(formats strfmt.Registry) error {
	if swag.IsZero(m.Tiering) { // not required
		return nil
	}

	if m.Tiering != nil {
		if err := m.Tiering.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("tiering")
			}
			return err
		}
	}

	return nil
}

// ContextValidate validate this zapp nvme inline components inline array item based on the context it is used
func (m *ZappNvmeInlineComponentsInlineArrayItem) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidatePerformance(ctx, formats); err != nil {
		res = append(res, err)
	}

	if err := m.contextValidateQos(ctx, formats); err != nil {
		res = append(res, err)
	}

	if err := m.contextValidateSubsystem(ctx, formats); err != nil {
		res = append(res, err)
	}

	if err := m.contextValidateTiering(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItem) contextValidatePerformance(ctx context.Context, formats strfmt.Registry) error {

	if m.Performance != nil {
		if err := m.Performance.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("performance")
			}
			return err
		}
	}

	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItem) contextValidateQos(ctx context.Context, formats strfmt.Registry) error {

	if m.Qos != nil {
		if err := m.Qos.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("qos")
			}
			return err
		}
	}

	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItem) contextValidateSubsystem(ctx context.Context, formats strfmt.Registry) error {

	if m.Subsystem != nil {
		if err := m.Subsystem.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("subsystem")
			}
			return err
		}
	}

	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItem) contextValidateTiering(ctx context.Context, formats strfmt.Registry) error {

	if m.Tiering != nil {
		if err := m.Tiering.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("tiering")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (m *ZappNvmeInlineComponentsInlineArrayItem) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *ZappNvmeInlineComponentsInlineArrayItem) UnmarshalBinary(b []byte) error {
	var res ZappNvmeInlineComponentsInlineArrayItem
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}

// ZappNvmeInlineComponentsInlineArrayItemInlinePerformance zapp nvme inline components inline array item inline performance
//
// swagger:model zapp_nvme_inline_components_inline_array_item_inline_performance
type ZappNvmeInlineComponentsInlineArrayItemInlinePerformance struct {

	// storage service
	StorageService *ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageService `json:"storage_service,omitempty"`
}

// Validate validates this zapp nvme inline components inline array item inline performance
func (m *ZappNvmeInlineComponentsInlineArrayItemInlinePerformance) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateStorageService(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItemInlinePerformance) validateStorageService(formats strfmt.Registry) error {
	if swag.IsZero(m.StorageService) { // not required
		return nil
	}

	if m.StorageService != nil {
		if err := m.StorageService.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("performance" + "." + "storage_service")
			}
			return err
		}
	}

	return nil
}

// ContextValidate validate this zapp nvme inline components inline array item inline performance based on the context it is used
func (m *ZappNvmeInlineComponentsInlineArrayItemInlinePerformance) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidateStorageService(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItemInlinePerformance) contextValidateStorageService(ctx context.Context, formats strfmt.Registry) error {

	if m.StorageService != nil {
		if err := m.StorageService.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("performance" + "." + "storage_service")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (m *ZappNvmeInlineComponentsInlineArrayItemInlinePerformance) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *ZappNvmeInlineComponentsInlineArrayItemInlinePerformance) UnmarshalBinary(b []byte) error {
	var res ZappNvmeInlineComponentsInlineArrayItemInlinePerformance
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}

// ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageService zapp nvme inline components inline array item inline performance inline storage service
//
// swagger:model zapp_nvme_inline_components_inline_array_item_inline_performance_inline_storage_service
type ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageService struct {

	// The storage service of the application component.
	// Enum: [extreme performance value]
	Name *string `json:"name,omitempty"`
}

// Validate validates this zapp nvme inline components inline array item inline performance inline storage service
func (m *ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageService) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateName(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

var zappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageServiceTypeNamePropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["extreme","performance","value"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		zappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageServiceTypeNamePropEnum = append(zappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageServiceTypeNamePropEnum, v)
	}
}

const (

	// BEGIN DEBUGGING
	// zapp_nvme_inline_components_inline_array_item_inline_performance_inline_storage_service
	// ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageService
	// name
	// Name
	// extreme
	// END DEBUGGING
	// ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageServiceNameExtreme captures enum value "extreme"
	ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageServiceNameExtreme string = "extreme"

	// BEGIN DEBUGGING
	// zapp_nvme_inline_components_inline_array_item_inline_performance_inline_storage_service
	// ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageService
	// name
	// Name
	// performance
	// END DEBUGGING
	// ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageServiceNamePerformance captures enum value "performance"
	ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageServiceNamePerformance string = "performance"

	// BEGIN DEBUGGING
	// zapp_nvme_inline_components_inline_array_item_inline_performance_inline_storage_service
	// ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageService
	// name
	// Name
	// value
	// END DEBUGGING
	// ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageServiceNameValue captures enum value "value"
	ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageServiceNameValue string = "value"
)

// prop value enum
func (m *ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageService) validateNameEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, zappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageServiceTypeNamePropEnum, true); err != nil {
		return err
	}
	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageService) validateName(formats strfmt.Registry) error {
	if swag.IsZero(m.Name) { // not required
		return nil
	}

	// value enum
	if err := m.validateNameEnum("performance"+"."+"storage_service"+"."+"name", "body", *m.Name); err != nil {
		return err
	}

	return nil
}

// ContextValidate validates this zapp nvme inline components inline array item inline performance inline storage service based on context it is used
func (m *ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageService) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageService) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageService) UnmarshalBinary(b []byte) error {
	var res ZappNvmeInlineComponentsInlineArrayItemInlinePerformanceInlineStorageService
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}

// ZappNvmeInlineComponentsInlineArrayItemInlineQos zapp nvme inline components inline array item inline qos
//
// swagger:model zapp_nvme_inline_components_inline_array_item_inline_qos
type ZappNvmeInlineComponentsInlineArrayItemInlineQos struct {

	// policy
	Policy *ZappNvmeInlineComponentsInlineArrayItemInlineQosInlinePolicy `json:"policy,omitempty"`
}

// Validate validates this zapp nvme inline components inline array item inline qos
func (m *ZappNvmeInlineComponentsInlineArrayItemInlineQos) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validatePolicy(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItemInlineQos) validatePolicy(formats strfmt.Registry) error {
	if swag.IsZero(m.Policy) { // not required
		return nil
	}

	if m.Policy != nil {
		if err := m.Policy.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("qos" + "." + "policy")
			}
			return err
		}
	}

	return nil
}

// ContextValidate validate this zapp nvme inline components inline array item inline qos based on the context it is used
func (m *ZappNvmeInlineComponentsInlineArrayItemInlineQos) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidatePolicy(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ZappNvmeInlineComponentsInlineArrayItemInlineQos) contextValidatePolicy(ctx context.Context, formats strfmt.Registry) error {

	if m.Policy != nil {
		if err := m.Policy.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("qos" + "." + "policy")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (m *ZappNvmeInlineComponentsInlineArrayItemInlineQos) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *ZappNvmeInlineComponentsInlineArrayItemInlineQos) UnmarshalBinary(b []byte) error {
	var res ZappNvmeInlineComponentsInlineArrayItemInlineQos
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}

// ZappNvmeInlineComponentsInlineArrayItemInlineQosInlinePolicy zapp nvme inline components inline array item inline qos inline policy
//
// swagger:model zapp_nvme_inline_components_inline_array_item_inline_qos_inline_policy
type ZappNvmeInlineComponentsInlineArrayItemInlineQosInlinePolicy struct {

	// The name of an existing QoS policy.
	Name *string `json:"name,omitempty"`

	// The UUID of an existing QoS policy. Usage: &lt;UUID&gt;
	UUID *string `json:"uuid,omitempty"`
}

// Validate validates this zapp nvme inline components inline array item inline qos inline policy
func (m *ZappNvmeInlineComponentsInlineArrayItemInlineQosInlinePolicy) Validate(formats strfmt.Registry) error {
	return nil
}

// ContextValidate validates this zapp nvme inline components inline array item inline qos inline policy based on context it is used
func (m *ZappNvmeInlineComponentsInlineArrayItemInlineQosInlinePolicy) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *ZappNvmeInlineComponentsInlineArrayItemInlineQosInlinePolicy) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *ZappNvmeInlineComponentsInlineArrayItemInlineQosInlinePolicy) UnmarshalBinary(b []byte) error {
	var res ZappNvmeInlineComponentsInlineArrayItemInlineQosInlinePolicy
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}

// ZappNvmeInlineRpo zapp nvme inline rpo
//
// swagger:model zapp_nvme_inline_rpo
type ZappNvmeInlineRpo struct {

	// local
	Local *ZappNvmeInlineRpoInlineLocal `json:"local,omitempty"`
}

// Validate validates this zapp nvme inline rpo
func (m *ZappNvmeInlineRpo) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateLocal(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ZappNvmeInlineRpo) validateLocal(formats strfmt.Registry) error {
	if swag.IsZero(m.Local) { // not required
		return nil
	}

	if m.Local != nil {
		if err := m.Local.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("rpo" + "." + "local")
			}
			return err
		}
	}

	return nil
}

// ContextValidate validate this zapp nvme inline rpo based on the context it is used
func (m *ZappNvmeInlineRpo) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidateLocal(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ZappNvmeInlineRpo) contextValidateLocal(ctx context.Context, formats strfmt.Registry) error {

	if m.Local != nil {
		if err := m.Local.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("rpo" + "." + "local")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (m *ZappNvmeInlineRpo) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *ZappNvmeInlineRpo) UnmarshalBinary(b []byte) error {
	var res ZappNvmeInlineRpo
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}

// ZappNvmeInlineRpoInlineLocal zapp nvme inline rpo inline local
//
// swagger:model zapp_nvme_inline_rpo_inline_local
type ZappNvmeInlineRpoInlineLocal struct {

	// The local RPO of the application.
	// Enum: [hourly none]
	Name *string `json:"name,omitempty"`

	// The Snapshot copy policy to apply to each volume in the smart container. This property is only supported for smart containers. Usage: &lt;snapshot policy&gt;
	Policy *string `json:"policy,omitempty"`
}

// Validate validates this zapp nvme inline rpo inline local
func (m *ZappNvmeInlineRpoInlineLocal) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateName(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

var zappNvmeInlineRpoInlineLocalTypeNamePropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["hourly","none"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		zappNvmeInlineRpoInlineLocalTypeNamePropEnum = append(zappNvmeInlineRpoInlineLocalTypeNamePropEnum, v)
	}
}

const (

	// BEGIN DEBUGGING
	// zapp_nvme_inline_rpo_inline_local
	// ZappNvmeInlineRpoInlineLocal
	// name
	// Name
	// hourly
	// END DEBUGGING
	// ZappNvmeInlineRpoInlineLocalNameHourly captures enum value "hourly"
	ZappNvmeInlineRpoInlineLocalNameHourly string = "hourly"

	// BEGIN DEBUGGING
	// zapp_nvme_inline_rpo_inline_local
	// ZappNvmeInlineRpoInlineLocal
	// name
	// Name
	// none
	// END DEBUGGING
	// ZappNvmeInlineRpoInlineLocalNameNone captures enum value "none"
	ZappNvmeInlineRpoInlineLocalNameNone string = "none"
)

// prop value enum
func (m *ZappNvmeInlineRpoInlineLocal) validateNameEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, zappNvmeInlineRpoInlineLocalTypeNamePropEnum, true); err != nil {
		return err
	}
	return nil
}

func (m *ZappNvmeInlineRpoInlineLocal) validateName(formats strfmt.Registry) error {
	if swag.IsZero(m.Name) { // not required
		return nil
	}

	// value enum
	if err := m.validateNameEnum("rpo"+"."+"local"+"."+"name", "body", *m.Name); err != nil {
		return err
	}

	return nil
}

// ContextValidate validates this zapp nvme inline rpo inline local based on context it is used
func (m *ZappNvmeInlineRpoInlineLocal) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *ZappNvmeInlineRpoInlineLocal) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *ZappNvmeInlineRpoInlineLocal) UnmarshalBinary(b []byte) error {
	var res ZappNvmeInlineRpoInlineLocal
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
