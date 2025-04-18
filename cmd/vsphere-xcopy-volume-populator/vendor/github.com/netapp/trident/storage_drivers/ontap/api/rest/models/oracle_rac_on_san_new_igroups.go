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

// OracleRacOnSanNewIgroups The list of initiator groups to create.
//
// swagger:model oracle_rac_on_san_new_igroups
type OracleRacOnSanNewIgroups struct {

	// A comment available for use by the administrator.
	Comment *string `json:"comment,omitempty"`

	// The name of the new initiator group.
	// Required: true
	// Max Length: 96
	// Min Length: 1
	Name *string `json:"name"`

	// oracle rac on san new igroups inline igroups
	OracleRacOnSanNewIgroupsInlineIgroups []*OracleRacOnSanNewIgroupsInlineIgroupsInlineArrayItem `json:"igroups,omitempty"`

	// oracle rac on san new igroups inline initiator objects
	OracleRacOnSanNewIgroupsInlineInitiatorObjects []*OracleRacOnSanNewIgroupsInlineInitiatorObjectsInlineArrayItem `json:"initiator_objects,omitempty"`

	// oracle rac on san new igroups inline initiators
	OracleRacOnSanNewIgroupsInlineInitiators []*string `json:"initiators,omitempty"`

	// The name of the host OS accessing the application. The default value is the host OS that is running the application.
	// Enum: [aix hpux hyper_v linux solaris vmware windows xen]
	OsType *string `json:"os_type,omitempty"`

	// The protocol of the new initiator group.
	// Enum: [fcp iscsi mixed]
	Protocol *string `json:"protocol,omitempty"`
}

// Validate validates this oracle rac on san new igroups
func (m *OracleRacOnSanNewIgroups) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateName(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateOracleRacOnSanNewIgroupsInlineIgroups(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateOracleRacOnSanNewIgroupsInlineInitiatorObjects(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateOsType(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateProtocol(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *OracleRacOnSanNewIgroups) validateName(formats strfmt.Registry) error {

	if err := validate.Required("name", "body", m.Name); err != nil {
		return err
	}

	if err := validate.MinLength("name", "body", *m.Name, 1); err != nil {
		return err
	}

	if err := validate.MaxLength("name", "body", *m.Name, 96); err != nil {
		return err
	}

	return nil
}

func (m *OracleRacOnSanNewIgroups) validateOracleRacOnSanNewIgroupsInlineIgroups(formats strfmt.Registry) error {
	if swag.IsZero(m.OracleRacOnSanNewIgroupsInlineIgroups) { // not required
		return nil
	}

	for i := 0; i < len(m.OracleRacOnSanNewIgroupsInlineIgroups); i++ {
		if swag.IsZero(m.OracleRacOnSanNewIgroupsInlineIgroups[i]) { // not required
			continue
		}

		if m.OracleRacOnSanNewIgroupsInlineIgroups[i] != nil {
			if err := m.OracleRacOnSanNewIgroupsInlineIgroups[i].Validate(formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("igroups" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

func (m *OracleRacOnSanNewIgroups) validateOracleRacOnSanNewIgroupsInlineInitiatorObjects(formats strfmt.Registry) error {
	if swag.IsZero(m.OracleRacOnSanNewIgroupsInlineInitiatorObjects) { // not required
		return nil
	}

	for i := 0; i < len(m.OracleRacOnSanNewIgroupsInlineInitiatorObjects); i++ {
		if swag.IsZero(m.OracleRacOnSanNewIgroupsInlineInitiatorObjects[i]) { // not required
			continue
		}

		if m.OracleRacOnSanNewIgroupsInlineInitiatorObjects[i] != nil {
			if err := m.OracleRacOnSanNewIgroupsInlineInitiatorObjects[i].Validate(formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("initiator_objects" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

var oracleRacOnSanNewIgroupsTypeOsTypePropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["aix","hpux","hyper_v","linux","solaris","vmware","windows","xen"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		oracleRacOnSanNewIgroupsTypeOsTypePropEnum = append(oracleRacOnSanNewIgroupsTypeOsTypePropEnum, v)
	}
}

const (

	// BEGIN DEBUGGING
	// oracle_rac_on_san_new_igroups
	// OracleRacOnSanNewIgroups
	// os_type
	// OsType
	// aix
	// END DEBUGGING
	// OracleRacOnSanNewIgroupsOsTypeAix captures enum value "aix"
	OracleRacOnSanNewIgroupsOsTypeAix string = "aix"

	// BEGIN DEBUGGING
	// oracle_rac_on_san_new_igroups
	// OracleRacOnSanNewIgroups
	// os_type
	// OsType
	// hpux
	// END DEBUGGING
	// OracleRacOnSanNewIgroupsOsTypeHpux captures enum value "hpux"
	OracleRacOnSanNewIgroupsOsTypeHpux string = "hpux"

	// BEGIN DEBUGGING
	// oracle_rac_on_san_new_igroups
	// OracleRacOnSanNewIgroups
	// os_type
	// OsType
	// hyper_v
	// END DEBUGGING
	// OracleRacOnSanNewIgroupsOsTypeHyperv captures enum value "hyper_v"
	OracleRacOnSanNewIgroupsOsTypeHyperv string = "hyper_v"

	// BEGIN DEBUGGING
	// oracle_rac_on_san_new_igroups
	// OracleRacOnSanNewIgroups
	// os_type
	// OsType
	// linux
	// END DEBUGGING
	// OracleRacOnSanNewIgroupsOsTypeLinux captures enum value "linux"
	OracleRacOnSanNewIgroupsOsTypeLinux string = "linux"

	// BEGIN DEBUGGING
	// oracle_rac_on_san_new_igroups
	// OracleRacOnSanNewIgroups
	// os_type
	// OsType
	// solaris
	// END DEBUGGING
	// OracleRacOnSanNewIgroupsOsTypeSolaris captures enum value "solaris"
	OracleRacOnSanNewIgroupsOsTypeSolaris string = "solaris"

	// BEGIN DEBUGGING
	// oracle_rac_on_san_new_igroups
	// OracleRacOnSanNewIgroups
	// os_type
	// OsType
	// vmware
	// END DEBUGGING
	// OracleRacOnSanNewIgroupsOsTypeVmware captures enum value "vmware"
	OracleRacOnSanNewIgroupsOsTypeVmware string = "vmware"

	// BEGIN DEBUGGING
	// oracle_rac_on_san_new_igroups
	// OracleRacOnSanNewIgroups
	// os_type
	// OsType
	// windows
	// END DEBUGGING
	// OracleRacOnSanNewIgroupsOsTypeWindows captures enum value "windows"
	OracleRacOnSanNewIgroupsOsTypeWindows string = "windows"

	// BEGIN DEBUGGING
	// oracle_rac_on_san_new_igroups
	// OracleRacOnSanNewIgroups
	// os_type
	// OsType
	// xen
	// END DEBUGGING
	// OracleRacOnSanNewIgroupsOsTypeXen captures enum value "xen"
	OracleRacOnSanNewIgroupsOsTypeXen string = "xen"
)

// prop value enum
func (m *OracleRacOnSanNewIgroups) validateOsTypeEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, oracleRacOnSanNewIgroupsTypeOsTypePropEnum, true); err != nil {
		return err
	}
	return nil
}

func (m *OracleRacOnSanNewIgroups) validateOsType(formats strfmt.Registry) error {
	if swag.IsZero(m.OsType) { // not required
		return nil
	}

	// value enum
	if err := m.validateOsTypeEnum("os_type", "body", *m.OsType); err != nil {
		return err
	}

	return nil
}

var oracleRacOnSanNewIgroupsTypeProtocolPropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["fcp","iscsi","mixed"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		oracleRacOnSanNewIgroupsTypeProtocolPropEnum = append(oracleRacOnSanNewIgroupsTypeProtocolPropEnum, v)
	}
}

const (

	// BEGIN DEBUGGING
	// oracle_rac_on_san_new_igroups
	// OracleRacOnSanNewIgroups
	// protocol
	// Protocol
	// fcp
	// END DEBUGGING
	// OracleRacOnSanNewIgroupsProtocolFcp captures enum value "fcp"
	OracleRacOnSanNewIgroupsProtocolFcp string = "fcp"

	// BEGIN DEBUGGING
	// oracle_rac_on_san_new_igroups
	// OracleRacOnSanNewIgroups
	// protocol
	// Protocol
	// iscsi
	// END DEBUGGING
	// OracleRacOnSanNewIgroupsProtocolIscsi captures enum value "iscsi"
	OracleRacOnSanNewIgroupsProtocolIscsi string = "iscsi"

	// BEGIN DEBUGGING
	// oracle_rac_on_san_new_igroups
	// OracleRacOnSanNewIgroups
	// protocol
	// Protocol
	// mixed
	// END DEBUGGING
	// OracleRacOnSanNewIgroupsProtocolMixed captures enum value "mixed"
	OracleRacOnSanNewIgroupsProtocolMixed string = "mixed"
)

// prop value enum
func (m *OracleRacOnSanNewIgroups) validateProtocolEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, oracleRacOnSanNewIgroupsTypeProtocolPropEnum, true); err != nil {
		return err
	}
	return nil
}

func (m *OracleRacOnSanNewIgroups) validateProtocol(formats strfmt.Registry) error {
	if swag.IsZero(m.Protocol) { // not required
		return nil
	}

	// value enum
	if err := m.validateProtocolEnum("protocol", "body", *m.Protocol); err != nil {
		return err
	}

	return nil
}

// ContextValidate validate this oracle rac on san new igroups based on the context it is used
func (m *OracleRacOnSanNewIgroups) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := m.contextValidateOracleRacOnSanNewIgroupsInlineIgroups(ctx, formats); err != nil {
		res = append(res, err)
	}

	if err := m.contextValidateOracleRacOnSanNewIgroupsInlineInitiatorObjects(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *OracleRacOnSanNewIgroups) contextValidateOracleRacOnSanNewIgroupsInlineIgroups(ctx context.Context, formats strfmt.Registry) error {

	for i := 0; i < len(m.OracleRacOnSanNewIgroupsInlineIgroups); i++ {

		if m.OracleRacOnSanNewIgroupsInlineIgroups[i] != nil {
			if err := m.OracleRacOnSanNewIgroupsInlineIgroups[i].ContextValidate(ctx, formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("igroups" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

func (m *OracleRacOnSanNewIgroups) contextValidateOracleRacOnSanNewIgroupsInlineInitiatorObjects(ctx context.Context, formats strfmt.Registry) error {

	for i := 0; i < len(m.OracleRacOnSanNewIgroupsInlineInitiatorObjects); i++ {

		if m.OracleRacOnSanNewIgroupsInlineInitiatorObjects[i] != nil {
			if err := m.OracleRacOnSanNewIgroupsInlineInitiatorObjects[i].ContextValidate(ctx, formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("initiator_objects" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

// MarshalBinary interface implementation
func (m *OracleRacOnSanNewIgroups) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *OracleRacOnSanNewIgroups) UnmarshalBinary(b []byte) error {
	var res OracleRacOnSanNewIgroups
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}

// OracleRacOnSanNewIgroupsInlineIgroupsInlineArrayItem oracle rac on san new igroups inline igroups inline array item
//
// swagger:model oracle_rac_on_san_new_igroups_inline_igroups_inline_array_item
type OracleRacOnSanNewIgroupsInlineIgroupsInlineArrayItem struct {

	// The name of an igroup to nest within a parent igroup. Mutually exclusive with initiators and initiator_objects.
	Name *string `json:"name,omitempty"`

	// The UUID of an igroup to nest within a parent igroup Usage: &lt;UUID&gt;
	UUID *string `json:"uuid,omitempty"`
}

// Validate validates this oracle rac on san new igroups inline igroups inline array item
func (m *OracleRacOnSanNewIgroupsInlineIgroupsInlineArrayItem) Validate(formats strfmt.Registry) error {
	return nil
}

// ContextValidate validates this oracle rac on san new igroups inline igroups inline array item based on context it is used
func (m *OracleRacOnSanNewIgroupsInlineIgroupsInlineArrayItem) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *OracleRacOnSanNewIgroupsInlineIgroupsInlineArrayItem) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *OracleRacOnSanNewIgroupsInlineIgroupsInlineArrayItem) UnmarshalBinary(b []byte) error {
	var res OracleRacOnSanNewIgroupsInlineIgroupsInlineArrayItem
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}

// OracleRacOnSanNewIgroupsInlineInitiatorObjectsInlineArrayItem oracle rac on san new igroups inline initiator objects inline array item
//
// swagger:model oracle_rac_on_san_new_igroups_inline_initiator_objects_inline_array_item
type OracleRacOnSanNewIgroupsInlineInitiatorObjectsInlineArrayItem struct {

	// A comment available for use by the administrator.
	Comment *string `json:"comment,omitempty"`

	// The WWPN, IQN, or Alias of the initiator. Mutually exclusive with nested igroups and the initiators array.
	Name *string `json:"name,omitempty"`
}

// Validate validates this oracle rac on san new igroups inline initiator objects inline array item
func (m *OracleRacOnSanNewIgroupsInlineInitiatorObjectsInlineArrayItem) Validate(formats strfmt.Registry) error {
	return nil
}

// ContextValidate validates this oracle rac on san new igroups inline initiator objects inline array item based on context it is used
func (m *OracleRacOnSanNewIgroupsInlineInitiatorObjectsInlineArrayItem) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *OracleRacOnSanNewIgroupsInlineInitiatorObjectsInlineArrayItem) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *OracleRacOnSanNewIgroupsInlineInitiatorObjectsInlineArrayItem) UnmarshalBinary(b []byte) error {
	var res OracleRacOnSanNewIgroupsInlineInitiatorObjectsInlineArrayItem
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
