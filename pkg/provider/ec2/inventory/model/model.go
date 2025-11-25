package model

import (
	"encoding/json"
	"reflect"

	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// Labels returns resource labels (empty for EC2).
func (m *Base) Labels() libmodel.Labels {
	return nil
}

// Equals compares two models by primary key.
func (m *Base) Equals(other libmodel.Model) bool {
	if o, ok := other.(interface{ Pk() string }); ok {
		return m.Pk() == o.Pk()
	}
	return false
}

// GetObject returns the stored AWS object as unstructured.
// Unmarshals the JSON Object field and adds indexed metadata fields.
func (m *Base) GetObject() (*unstructured.Unstructured, error) {
	var obj map[string]interface{}
	err := json.Unmarshal([]byte(m.Object), &obj)
	if err != nil {
		return nil, err
	}

	// Add indexed fields to the object
	obj["id"] = m.UID
	obj["name"] = m.Name
	obj["kind"] = m.Kind
	obj["provider"] = m.Provider
	obj["revision"] = m.Revision

	return &unstructured.Unstructured{Object: obj}, nil
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

// HasChanged checks if the model has changed by comparing indexed fields and JSON content.
// Returns true if revision, name, kind, or object content differs.
func (m *Base) HasChanged(new *Base) bool {
	// Quick check: indexed fields
	if m.Revision != new.Revision || m.Name != new.Name || m.Kind != new.Kind {
		return true
	}

	// Deep check: JSON object content
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

// Volume represents an EBS volume (block storage).
// Extends Base with volume-specific indexed fields.
type Volume struct {
	Base
	VolumeType string `sql:"d0,index(volumeType)"` // gp2, gp3, io1, io2, st1, sc1, etc.
	State      string `sql:"d0,index(state)"`      // available, in-use, creating, deleting, etc.
	Size       int64  `sql:"d0,index(size)"`       // Size in GB
}

// Network represents VPC/Subnet networking resources.
// Stores both VPCs and Subnets with network-specific indexed fields.
type Network struct {
	Base
	NetworkType string `sql:"d0,index(networkType)"` // vpc, subnet
	CIDR        string `sql:"d0,index(cidr)"`        // CIDR block (e.g., 10.0.0.0/16)
}

// Storage represents EBS volume types (analogous to storage classes).
// Used for mapping source volume types to target storage classes.
// The Object field contains volume type details (IOPS limits, throughput, etc.).
type Storage struct {
	Base
	VolumeType string `sql:"d0,index(volumeType)"` // gp2, gp3, io1, io2, st1, sc1, etc.
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
