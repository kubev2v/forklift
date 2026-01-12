package model

import (
	"reflect"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Errors
var NotFound = libmodel.NotFound

const (
	MaxDetail = 3
)

//
// Base Model
//

// Base model with indexed fields for efficient queries.
// Each resource type adds its own typed Object field.
type Base struct {
	UID      string `sql:"pk"`                             // Primary key - AWS resource ID
	Name     string `sql:"d0,index(name)"`                 // Resource name
	Kind     string `sql:"d0,index(kind)"`                 // Resource type (Instance, Volume, Network, Storage)
	Provider string `sql:"d0,index(provider)"`             // Provider UID
	Revision int64  `sql:"incremented,d0,index(revision)"` // Change tracking for updates
}

//
// Base Model Methods
//

// Pk returns the primary key.
func (m *Base) Pk() string {
	return m.UID
}

// String representation.
func (m *Base) String() string {
	return m.UID
}

//
// Resource-Specific Models
//

// Instance represents an EC2 instance (virtual machine).
// Extends Base with additional indexed fields and typed AWS object.
type Instance struct {
	Base
	InstanceType string            `sql:"d0,index(instanceType)"` // t2.micro, m5.large, etc.
	State        string            `sql:"d0,index(state)"`        // running, stopped, terminated, etc.
	Platform     string            `sql:"d0,index(platform)"`     // Linux, Windows, etc.
	Object       ec2types.Instance `sql:"d0"`                     // Complete AWS Instance object
}

// Labels returns AWS tags as labels for label-based filtering.
func (m *Instance) Labels() libmodel.Labels {
	if len(m.Object.Tags) == 0 {
		return nil
	}
	labels := make(libmodel.Labels, len(m.Object.Tags))
	for _, tag := range m.Object.Tags {
		if tag.Key != nil && tag.Value != nil {
			labels[*tag.Key] = *tag.Value
		}
	}
	return labels
}

// GetDetails returns the instance details for API responses.
func (m *Instance) GetDetails() (*InstanceDetails, error) {
	details := &InstanceDetails{
		Instance: m.Object,
		ID:       m.UID,
		Name:     m.Name,
		Kind:     m.Kind,
		Provider: m.Provider,
		Revision: m.Revision,
	}

	// Convert AWS BlockDeviceMappings to our simplified type
	for _, bdm := range m.Object.BlockDeviceMappings {
		mapping := InstanceBlockDeviceMapping{
			DeviceName: bdm.DeviceName,
		}
		if bdm.Ebs != nil {
			mapping.Ebs = &EbsInstanceBlockDevice{
				VolumeId: bdm.Ebs.VolumeId,
			}
		}
		details.BlockDeviceMappings = append(details.BlockDeviceMappings, mapping)
	}

	// Convert AWS NetworkInterfaces to our simplified type
	for _, nic := range m.Object.NetworkInterfaces {
		iface := InstanceNetworkInterface{
			SubnetId:   nic.SubnetId,
			MacAddress: nic.MacAddress,
		}
		details.NetworkInterfaces = append(details.NetworkInterfaces, iface)
	}

	return details, nil
}

// HasChanged checks if the instance has changed.
func (m *Instance) HasChanged(new *Instance) bool {
	if m.Name != new.Name || m.Kind != new.Kind {
		return true
	}
	if m.InstanceType != new.InstanceType || m.State != new.State || m.Platform != new.Platform {
		return true
	}
	return !reflect.DeepEqual(m.Object, new.Object)
}

type InstanceDetails struct {
	ec2types.Instance
	BlockDeviceMappings []InstanceBlockDeviceMapping `json:"BlockDeviceMappings,omitempty"`
	NetworkInterfaces   []InstanceNetworkInterface   `json:"NetworkInterfaces,omitempty"`
	ID                  string                       `json:"id"`
	Name                string                       `json:"name"`
	Kind                string                       `json:"kind"`
	Provider            string                       `json:"provider"`
	Revision            int64                        `json:"revision"`
}

// InstanceBlockDeviceMapping represents a simplified block device mapping.
type InstanceBlockDeviceMapping struct {
	DeviceName  *string                 `json:"DeviceName,omitempty"`
	Ebs         *EbsInstanceBlockDevice `json:"Ebs,omitempty"`
	VirtualName *string                 `json:"VirtualName,omitempty"`
}

// EbsInstanceBlockDevice contains EBS-specific block device info.
type EbsInstanceBlockDevice struct {
	VolumeId *string `json:"VolumeId,omitempty"`
}

// InstanceNetworkInterface represents a simplified network interface.
type InstanceNetworkInterface struct {
	SubnetId   *string `json:"SubnetId,omitempty"`
	MacAddress *string `json:"MacAddress,omitempty"`
}

// Volume represents an EBS volume (block storage).
// Extends Base with volume-specific indexed fields and typed AWS object.
type Volume struct {
	Base
	VolumeType string          `sql:"d0,index(volumeType)"` // gp2, gp3, io1, io2, st1, sc1, etc.
	State      string          `sql:"d0,index(state)"`      // available, in-use, creating, deleting, etc.
	Size       int64           `sql:"d0,index(size)"`       // Size in GB
	Object     ec2types.Volume `sql:"d0"`                   // Complete AWS Volume object
}

// Labels returns AWS tags as labels for label-based filtering.
func (m *Volume) Labels() libmodel.Labels {
	if len(m.Object.Tags) == 0 {
		return nil
	}
	labels := make(libmodel.Labels, len(m.Object.Tags))
	for _, tag := range m.Object.Tags {
		if tag.Key != nil && tag.Value != nil {
			labels[*tag.Key] = *tag.Value
		}
	}
	return labels
}

// GetDetails returns the volume details for API responses.
func (m *Volume) GetDetails() (*VolumeDetails, error) {
	return &VolumeDetails{
		Volume:   m.Object,
		ID:       m.UID,
		Name:     m.Name,
		Kind:     m.Kind,
		Provider: m.Provider,
		Revision: m.Revision,
	}, nil
}

// HasChanged checks if the volume has changed.
func (m *Volume) HasChanged(new *Volume) bool {
	if m.Name != new.Name || m.Kind != new.Kind {
		return true
	}
	if m.VolumeType != new.VolumeType || m.State != new.State || m.Size != new.Size {
		return true
	}
	return !reflect.DeepEqual(m.Object, new.Object)
}

type VolumeDetails struct {
	ec2types.Volume
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Provider string `json:"provider"`
	Revision int64  `json:"revision"`
}

// Network represents VPC/Subnet networking resources.
// Stores Subnets with network-specific indexed fields and typed AWS object.
// VPCs are stored with minimal data (no full object) since subnets are primary for migration.
type Network struct {
	Base
	NetworkType string          `sql:"d0,index(networkType)"` // vpc, subnet
	CIDR        string          `sql:"d0,index(cidr)"`        // CIDR block (e.g., 10.0.0.0/16)
	Object      ec2types.Subnet `sql:"d0"`                    // Complete AWS Subnet object (for subnets)
}

// Labels returns AWS tags as labels for label-based filtering.
// For subnets, uses tags from the typed Object.
// For VPCs, labels are not available (would require separate VPC object storage).
func (m *Network) Labels() libmodel.Labels {
	// Only subnets have the typed Object populated
	if m.NetworkType != "subnet" || len(m.Object.Tags) == 0 {
		return nil
	}
	labels := make(libmodel.Labels, len(m.Object.Tags))
	for _, tag := range m.Object.Tags {
		if tag.Key != nil && tag.Value != nil {
			labels[*tag.Key] = *tag.Value
		}
	}
	return labels
}

// GetDetails returns the network details for API responses.
func (m *Network) GetDetails() (*NetworkDetails, error) {
	return &NetworkDetails{
		Subnet:   m.Object,
		ID:       m.UID,
		Name:     m.Name,
		Kind:     m.Kind,
		Provider: m.Provider,
		Revision: m.Revision,
	}, nil
}

// HasChanged checks if the network has changed.
func (m *Network) HasChanged(new *Network) bool {
	if m.Name != new.Name || m.Kind != new.Kind {
		return true
	}
	if m.NetworkType != new.NetworkType || m.CIDR != new.CIDR {
		return true
	}
	return !reflect.DeepEqual(m.Object, new.Object)
}

type NetworkDetails struct {
	ec2types.Subnet
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Provider string `json:"provider"`
	Revision int64  `json:"revision"`
}

// Storage represents EBS volume types (analogous to storage classes).
// Used for mapping source volume types to target storage classes.
// Uses a custom StorageData struct since this is not an AWS API object.
type Storage struct {
	Base
	VolumeType string      `sql:"d0,index(volumeType)"` // gp2, gp3, io1, io2, st1, sc1, etc.
	Object     StorageData `sql:"d0"`                   // Volume type details
}

// StorageData contains EBS volume type characteristics.
type StorageData struct {
	Type          string `json:"type"`
	Description   string `json:"description"`
	MaxIOPS       int32  `json:"maxIOPS"`
	MaxThroughput int32  `json:"maxThroughput"`
}

// Labels returns nil since storage types don't have AWS tags.
func (m *Storage) Labels() libmodel.Labels {
	return nil
}

// GetDetails returns the storage details for API responses.
func (m *Storage) GetDetails() (*StorageDetails, error) {
	return &StorageDetails{
		ID:            m.UID,
		Name:          m.Name,
		Kind:          m.Kind,
		Provider:      m.Provider,
		Revision:      m.Revision,
		Type:          m.Object.Type,
		Description:   m.Object.Description,
		MaxIOPS:       m.Object.MaxIOPS,
		MaxThroughput: m.Object.MaxThroughput,
	}, nil
}

// HasChanged checks if the storage has changed.
func (m *Storage) HasChanged(new *Storage) bool {
	if m.Name != new.Name || m.Kind != new.Kind {
		return true
	}
	if m.VolumeType != new.VolumeType {
		return true
	}
	return !reflect.DeepEqual(m.Object, new.Object)
}

type StorageDetails struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Provider      string `json:"provider"`
	Revision      int64  `json:"revision"`
	Type          string `json:"type"`
	Description   string `json:"description"`
	MaxIOPS       int32  `json:"maxIOPS"`
	MaxThroughput int32  `json:"maxThroughput"`
}

type SnapshotDetails struct {
	ec2types.Snapshot
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Provider string `json:"provider"`
	Revision int64  `json:"revision"`
}

//
// Model Registration
//

// All returns all EC2 model types for database registration.
// This function is called during inventory initialization to create database tables.
func All() []interface{} {
	return []interface{}{
		&Instance{},
		&Volume{},
		&Network{},
		&Storage{},
	}
}
