package model

import (
	"encoding/json"
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

// Base model with indexed fields + JSON object storage for flexible schema.
// Indexes essential fields for efficient queries, preserves complete AWS data in Object field.
type Base struct {
	UID      string `sql:"pk"`                             // Primary key - AWS resource ID
	Name     string `sql:"d0,index(name)"`                 // Resource name
	Kind     string `sql:"d0,index(kind)"`                 // Resource type (Instance, Volume, Network, Storage)
	Provider string `sql:"d0,index(provider)"`             // Provider UID
	Revision int64  `sql:"incremented,d0,index(revision)"` // Change tracking for updates

	// Complete AWS object as JSON - stores full AWS API response
	Object string `sql:"d0"` // JSON-encoded full object
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

// tagsContainer is used to extract AWS tags from the JSON object.
// AWS resources store tags as an array of {Key, Value} objects.
type tagsContainer struct {
	Tags []struct {
		Key   *string `json:"Key"`
		Value *string `json:"Value"`
	} `json:"Tags"`
}

// Labels returns AWS tags as labels.
// Extracts tags from the stored JSON object and converts them to libmodel.Labels.
func (m *Base) Labels() libmodel.Labels {
	var container tagsContainer
	if err := json.Unmarshal([]byte(m.Object), &container); err != nil {
		return nil
	}

	if len(container.Tags) == 0 {
		return nil
	}

	labels := make(libmodel.Labels, len(container.Tags))
	for _, tag := range container.Tags {
		if tag.Key != nil && tag.Value != nil {
			labels[*tag.Key] = *tag.Value
		}
	}
	return labels
}

// GetObject returns the stored AWS object.
// Unmarshals the JSON Object field.
func (m *Base) GetObject(obj interface{}) error {
	return json.Unmarshal([]byte(m.Object), obj)
}

// SetObject stores an AWS object as JSON.
// Marshals the provided object and stores it in the Object field.
func (m *Base) SetObject(obj interface{}) error {
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	m.Object = string(jsonBytes)
	return nil
}

// HasChanged checks if the model's data content has changed.
// Compares name, kind, and object content (skips revision since it's a tracking field).
// Returns true if any actual data differs.
func (m *Base) HasChanged(new *Base) bool {
	// Quick check: indexed fields that represent actual data
	if m.Name != new.Name || m.Kind != new.Kind {
		return true
	}

	// Deep check: JSON object content (handles key ordering differences)
	var oldObj, newObj map[string]interface{}

	if err := json.Unmarshal([]byte(m.Object), &oldObj); err != nil {
		// Fallback to string comparison if unmarshal fails
		return m.Object != new.Object
	}

	if err := json.Unmarshal([]byte(new.Object), &newObj); err != nil {
		// Fallback to string comparison if unmarshal fails
		return m.Object != new.Object
	}

	return !reflect.DeepEqual(oldObj, newObj)
}

//
// Resource-Specific Models
//

// Instance represents an EC2 instance (virtual machine).
// Extends Base with additional indexed fields for efficient querying.
type Instance struct {
	Base
	InstanceType string `sql:"d0,index(instanceType)"` // t2.micro, m5.large, etc.
	State        string `sql:"d0,index(state)"`        // running, stopped, terminated, etc.
	Platform     string `sql:"d0,index(platform)"`     // Linux, Windows, etc.
}

func (m *Instance) GetDetails() (*InstanceDetails, error) {
	details := &InstanceDetails{}
	err := m.GetObject(details)
	if err != nil {
		return nil, err
	}
	details.ID = m.UID
	details.Name = m.Name
	details.Kind = m.Kind
	details.Provider = m.Provider
	details.Revision = m.Revision
	return details, nil
}

// HasChanged checks if the instance has changed by comparing base fields and instance-specific fields.
func (m *Instance) HasChanged(new *Instance) bool {
	if m.Base.HasChanged(&new.Base) {
		return true
	}
	return m.InstanceType != new.InstanceType ||
		m.State != new.State ||
		m.Platform != new.Platform
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

type InstanceBlockDeviceMapping struct {
	DeviceName  *string                 `json:"DeviceName,omitempty"`
	Ebs         *EbsInstanceBlockDevice `json:"Ebs,omitempty"`
	VirtualName *string                 `json:"VirtualName,omitempty"`
}

type EbsInstanceBlockDevice struct {
	VolumeId *string `json:"VolumeId,omitempty"`
}

type InstanceNetworkInterface struct {
	SubnetId   *string `json:"SubnetId,omitempty"`
	MacAddress *string `json:"MacAddress,omitempty"`
}

// Volume represents an EBS volume (block storage).
// Extends Base with volume-specific indexed fields.
type Volume struct {
	Base
	VolumeType string `sql:"d0,index(volumeType)"` // gp2, gp3, io1, io2, st1, sc1, etc.
	State      string `sql:"d0,index(state)"`      // available, in-use, creating, deleting, etc.
	Size       int64  `sql:"d0,index(size)"`       // Size in GB
}

func (m *Volume) GetDetails() (*VolumeDetails, error) {
	details := &VolumeDetails{}
	err := m.GetObject(details)
	if err != nil {
		return nil, err
	}
	details.ID = m.UID
	details.Name = m.Name
	details.Kind = m.Kind
	details.Provider = m.Provider
	details.Revision = m.Revision
	return details, nil
}

// HasChanged checks if the volume has changed by comparing base fields and volume-specific fields.
func (m *Volume) HasChanged(new *Volume) bool {
	if m.Base.HasChanged(&new.Base) {
		return true
	}
	return m.VolumeType != new.VolumeType ||
		m.State != new.State ||
		m.Size != new.Size
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
// Stores both VPCs and Subnets with network-specific indexed fields.
type Network struct {
	Base
	NetworkType string `sql:"d0,index(networkType)"` // vpc, subnet
	CIDR        string `sql:"d0,index(cidr)"`        // CIDR block (e.g., 10.0.0.0/16)
}

func (m *Network) GetDetails() (*NetworkDetails, error) {
	details := &NetworkDetails{}
	err := m.GetObject(details)
	if err != nil {
		return nil, err
	}
	details.ID = m.UID
	details.Name = m.Name
	details.Kind = m.Kind
	details.Provider = m.Provider
	details.Revision = m.Revision
	return details, nil
}

// HasChanged checks if the network has changed by comparing base fields and network-specific fields.
func (m *Network) HasChanged(new *Network) bool {
	if m.Base.HasChanged(&new.Base) {
		return true
	}
	return m.NetworkType != new.NetworkType ||
		m.CIDR != new.CIDR
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
// The Object field contains volume type details (IOPS limits, throughput, etc.).
type Storage struct {
	Base
	VolumeType string `sql:"d0,index(volumeType)"` // gp2, gp3, io1, io2, st1, sc1, etc.
}

func (m *Storage) GetDetails() (*StorageDetails, error) {
	details := &StorageDetails{}
	err := m.GetObject(details)
	if err != nil {
		return nil, err
	}
	details.ID = m.UID
	details.Name = m.Name
	details.Kind = m.Kind
	details.Provider = m.Provider
	details.Revision = m.Revision
	return details, nil
}

// HasChanged checks if the storage has changed by comparing base fields and storage-specific fields.
func (m *Storage) HasChanged(new *Storage) bool {
	if m.Base.HasChanged(&new.Base) {
		return true
	}
	return m.VolumeType != new.VolumeType
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
