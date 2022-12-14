package openstack

import (
	"time"

	"github.com/konveyor/forklift-controller/pkg/controller/provider/model/base"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
)

//
// Errors
var NotFound = libmodel.NotFound

type InvalidRefError = base.InvalidRefError

const (
	MaxDetail = base.MaxDetail
)

//
// Types
type Model = base.Model
type ListOptions = base.ListOptions
type Concern = base.Concern
type Ref = base.Ref

//
// Base OpenStack model.
type Base struct {
	// Managed object ID.
	ID string `sql:"pk"`
	// Name
	Name string `sql:"d0,index(name)"`
	// Revision
	Revision int64 `sql:"incremented,d0,index(revision)"`
}

//
// Get the PK.
func (m *Base) Pk() string {
	return m.ID
}

//
// String representation.
func (m *Base) String() string {
	return m.ID
}

type Region struct {
	Base
	Description    string `sql:""`
	ParentRegionID string `sql:""`
}

type Project struct {
	Base
	Description string `sql:""`
	DomainID    string `sql:""`
	ParentID    string `sql:""`
	Enabled     bool   `sql:""`
	IsDomain    bool   `sql:""`
}

type Image struct {
	Base
	Status                      string                 `sql:""`
	Tags                        []string               `sql:""`
	ContainerFormat             string                 `sql:""`
	DiskFormat                  string                 `sql:""`
	MinDiskGigabytes            int                    `sql:""`
	MinRAMMegabytes             int                    `sql:""`
	Owner                       string                 `sql:""`
	Protected                   bool                   `sql:""`
	Visibility                  string                 `sql:""`
	Hidden                      bool                   `sql:""`
	Checksum                    string                 `sql:""`
	SizeBytes                   int64                  `sql:""`
	Metadata                    map[string]string      `sql:""`
	Properties                  map[string]interface{} `sql:""`
	CreatedAt                   time.Time              `sql:""`
	UpdatedAt                   time.Time              `sql:""`
	File                        string                 `sql:""`
	Schema                      string                 `sql:""`
	VirtualSize                 int64                  `sql:""`
	OpenStackImageImportMethods []string               `sql:""`
	OpenStackImageStoreIDs      []string               `sql:""`
}

type Flavor struct {
	Base
	Disk        int     `sql:"" `
	RAM         int     `sql:"" `
	RxTxFactor  float64 `sql:"" `
	Swap        int     `sql:"" `
	VCPUs       int     `sql:"" `
	IsPublic    bool    `sql:"" `
	Ephemeral   int     `sql:"" `
	Description string  `sql:""`
}

type AttachedVolume struct {
	ID string `sql:"d0,fk(volume)"`
}

type Fault struct {
	Code    int       `sql:""`
	Created time.Time `sql:""`
	Details string    `sql:""`
	Message string    `sql:""`
}

type VM struct {
	Base
	RevisionValidated int64                    `sql:"d0,index(revisionValidated)"`
	PolicyVersion     int                      `sql:"d0,index(policyVersion)" eq:"-"`
	TenantID          string                   `sql:"d0,fk(project +cascade)"`
	UserID            string                   `sql:""`
	Updated           time.Time                `sql:""`
	Created           time.Time                `sql:""`
	HostID            string                   `sql:""`
	Status            string                   `sql:""`
	Progress          int                      `sql:""`
	AccessIPv4        string                   `sql:""`
	AccessIPv6        string                   `sql:""`
	ImageID           string                   `sql:"d0,fk(image +cascade)"`
	FlavorID          string                   `sql:"d0,fk(flavor +cascade)"`
	Addresses         map[string]interface{}   `sql:""`
	Metadata          map[string]string        `sql:""`
	KeyName           string                   `sql:""`
	AdminPass         string                   `sql:""`
	SecurityGroups    []map[string]interface{} `sql:"" `
	AttachedVolumes   []AttachedVolume         `sql:"" `
	Fault             Fault                    `sql:"" `
	Tags              *[]string                `sql:"" `
	ServerGroups      *[]string                `sql:"" `
	Concerns          []Concern                `sql:"" eq:"-"`
}

//
// Determine if current revision has been validated.
func (m *VM) Validated() bool {
	return m.RevisionValidated == m.Revision
}

type Attachment struct {
	AttachedAt   time.Time `sql:""`
	AttachmentID string    `sql:""`
	Device       string    `sql:""`
	HostName     string    `sql:""`
	ID           string    `sql:""`
	ServerID     string    `sql:""`
	VolumeID     string    `sql:""`
}

type VolumeType struct {
	Base
	ExtraSpecs   map[string]interface{} `sql:""`
	IsPublic     bool                   `sql:""`
	QosSpecID    string                 `sql:""`
	PublicAccess bool                   `sql:""`
}

type Snapshot struct {
	Base
	CreatedAt time.Time         `sql:""`
	UpdatedAt time.Time         `sql:""`
	VolumeID  string            `sql:""`
	Status    string            `sql:""`
	Size      int               `sql:""`
	Metadata  map[string]string `sql:""`
}

type Volume struct {
	Base
	Status              string            `sql:""`
	Size                int               `sql:""`
	AvailabilityZone    string            `sql:""`
	CreatedAt           time.Time         `sql:""`
	UpdatedAt           time.Time         `sql:""`
	Attachments         []Attachment      `sql:""`
	Description         string            `sql:""`
	VolumeType          string            `sql:""`
	SnapshotID          string            `sql:""`
	SourceVolID         string            `sql:""`
	BackupID            *string           `sql:""`
	Metadata            map[string]string `sql:""`
	UserID              string            `sql:""`
	Bootable            string            `sql:""`
	Encrypted           bool              `sql:""`
	ReplicationStatus   string            `sql:""`
	ConsistencyGroupID  string            `sql:""`
	Multiattach         bool              `sql:""`
	VolumeImageMetadata map[string]string `sql:""`
}
