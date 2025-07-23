/*
 Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package v100

// QueryParams is a map of key value pairs that can be
// appended to any url as query parameters.
type QueryParams map[string]interface{}

// IncludeDetails is boolean flag that can be passed as a query param to the
// volume listing endpoing for getting the extensive details about the snapshots.
const IncludeDetails = "includeDetails"

// SnapshotName can be passed as a query param to the volume listing
// endpoing for filtering the results based on snapshot name
const SnapshotName = "snapshotName"

// InSG can be passed as a query param to the volume listing
// endpoing for filtering the results based on their
// association to a storage group
const InSG = "inSG"

// IsRdf can be passed as a query param to the volume listing
// endpoing for filtering the resluts based on their RDF relationship
const IsRdf = "isRdf"

// VolumeList contains list of device names
type VolumeList struct {
	Name string `json:"name"`
}

// CreateVolumesSnapshot contains parameters to create a volume snapshot
type CreateVolumesSnapshot struct {
	SourceVolumeList []VolumeList `json:"deviceNameListSource"`
	BothSides        bool         `json:"bothSides"`
	Star             bool         `json:"star"`
	Force            bool         `json:"force"`
	TimeInHours      bool         `json:"timeInHours"`
	TimeToLive       int64        `json:"timeToLive"`
	TTL              int64        `json:"ttl,omitempty"`
	Securettl        int64        `json:"securettl,omitempty"`
	ExecutionOption  string       `json:"executionOption"`
}

// ModifyVolumeSnapshot contains input parameters to modify the snapshot
type ModifyVolumeSnapshot struct {
	VolumeNameListSource []VolumeList `json:"deviceNameListSource"`
	VolumeNameListTarget []VolumeList `json:"deviceNameListTarget"`
	Force                bool         `json:"force,omitempty"`
	Star                 bool         `json:"star,omitempty"`
	Exact                bool         `json:"exact,omitempty"`
	Copy                 bool         `json:"copy,omitempty"`
	Remote               bool         `json:"remote,omitempty"`
	Symforce             bool         `json:"symforce,omitempty"`
	NoCopy               bool         `json:"nocopy,omitempty"`
	TTL                  int64        `json:"ttl,omitempty"`
	SecureTTL            int64        `json:"securettl,omitempty"`
	NewSnapshotName      string       `json:"newsnapshotname,omitempty"`
	TimeInHours          bool         `json:"timeInHours"`
	Action               string       `json:"action"`
	Generation           int64        `json:"generation"`
	ExecutionOption      string       `json:"executionOption,omitempty"`
}

// DeleteVolumeSnapshot contains input parameters to delete the snapshot
type DeleteVolumeSnapshot struct {
	DeviceNameListSource []VolumeList `json:"deviceNameListSource"`
	Symforce             bool         `json:"symforce,omitempty"`
	Star                 bool         `json:"star,omitempty"`
	Force                bool         `json:"force,omitempty"`
	Restore              bool         `json:"restore,omitempty"`
	Generation           int64        `json:"generation"`
	ExecutionOption      string       `json:"executionOption,omitempty"`
}

// VolumeSnapshotSource holds information on volume snapshot source
type VolumeSnapshotSource struct {
	SnapshotName         string          `json:"snapshotName"`
	Generation           int64           `json:"generation"`
	TimeStamp            string          `json:"timestamp"`
	State                string          `json:"state"`
	ProtectionExpireTime int64           `json:"protectionExpireTime"`
	GCM                  bool            `json:"gcm"`
	ICDP                 bool            `json:"icdp"`
	Secured              bool            `json:"secured"`
	IsRestored           bool            `json:"isRestored"`
	TTL                  int64           `json:"ttl"`
	Expired              bool            `json:"expired"`
	LinkedVolumes        []LinkedVolumes `json:"linkedDevices"`
}

// LinkedVolumes contains information about linked volumes of the snapshot
type LinkedVolumes struct {
	TargetDevice     string `json:"targetDevice"`
	Timestamp        string `json:"timestamp"`
	State            string `json:"state"`
	TrackSize        int64  `json:"trackSize"`
	Tracks           int64  `json:"tracks"`
	PercentageCopied int64  `json:"percentageCopied"`
	Linked           bool   `json:"linked"`
	Restored         bool   `json:"restored"`
	Defined          bool   `json:"defined"`
	Copy             bool   `json:"copy"`
	Destage          bool   `json:"destage"`
	Modified         bool   `json:"modified"`
	LinkSource       string `json:"linkSourceName"`
	SnapshotName     string `json:"snapshot_name"`
	Generation       int64  `json:"generation"`
}

// VolumeSnapshotLink contains information about linked snapshots
type VolumeSnapshotLink struct {
	TargetDevice     string `json:"targetDevice"`
	Timestamp        string `json:"timestamp"`
	State            string `json:"state"`
	TrackSize        int64  `json:"trackSize"`
	Tracks           int64  `json:"tracks"`
	PercentageCopied int64  `json:"percentageCopied"`
	Linked           bool   `json:"linked"`
	Restored         bool   `json:"restored"`
	Defined          bool   `json:"defined"`
	Copy             bool   `json:"copy"`
	Destage          bool   `json:"destage"`
	Modified         bool   `json:"modified"`
	LinkSource       string `json:"linkSourceName"`
	SnapshotName     string `json:"snapshot_name"`
	Generation       int64  `json:"generation"`
}

// VolumeSnapshot contains list of volume snapshots
type VolumeSnapshot struct {
	DeviceName           string                 `json:"deviceName"`
	SnapshotName         string                 `json:"snapshotName"`
	VolumeSnapshotSource []VolumeSnapshotSource `json:"snapshotSrc"`
	VolumeSnapshotLink   []VolumeSnapshotLink   `json:"snapshotLnk,omitempty"`
}

// SnapshotVolumeGeneration contains information on all snapshots related to a volume
type SnapshotVolumeGeneration struct {
	DeviceName           string                 `json:"deviceName"`
	VolumeSnapshotSource []VolumeSnapshotSource `json:"snapshotSrcs"`
	VolumeSnapshotLink   []VolumeSnapshotLink   `json:"snapshotLnks,omitempty"`
}

// VolumeSnapshotGeneration contains information on generation of a snapshot
type VolumeSnapshotGeneration struct {
	DeviceName           string               `json:"deviceName"`
	SnapshotName         string               `json:"snapshotName"`
	Generation           int64                `json:"generation"`
	VolumeSnapshotSource VolumeSnapshotSource `json:"snapshotSrc"`
	VolumeSnapshotLink   []VolumeSnapshotLink `json:"snapshotLnk,omitempty"`
}

// VolumeSnapshotGenerations contains list of volume snapshot generations
type VolumeSnapshotGenerations struct {
	DeviceName           string                 `json:"deviceName"`
	Generation           []int64                `json:"generation"`
	SnapshotName         string                 `json:"snapshotName"`
	VolumeSnapshotSource []VolumeSnapshotSource `json:"snapshotSrc"`
	VolumeSnapshotLink   []VolumeSnapshotLink   `json:"snapshotLnk,omitempty"`
}

// SnapshotNameAndCounts object for storage group snapshots
type SnapshotNameAndCounts struct {
	Name               string `json:"name"`
	SnapshotCount      int64  `json:"snapshot_count"`
	NewestTimestampUtc int64  `json:"newest_timestamp_utc"`
}

// StorageGroupSnapshot contains a list of storage group snapshots
type StorageGroupSnapshot struct {
	Name                   []string                `json:"name"`
	SlSnapshotName         []string                `json:"sl_snapshot_name"`
	SnapshotNamesAndCounts []SnapshotNameAndCounts `json:"snapshot_names_and_counts"`
}

// StorageGroupSnap a PowerMax Snap Object
type StorageGroupSnap struct {
	Name                    string               `json:"name"`
	Generation              int64                `json:"generation"`
	SnapID                  int64                `json:"snapid"`
	Timestamp               string               `json:"timestamp"`
	TimestampUtc            int64                `json:"timestamp_utc"`
	State                   []string             `json:"state"`
	NumSourceVolumes        int32                `json:"num_source_volumes"`
	SourceVolume            []SourceVolume       `json:"source_volume"`
	NumStorageGroupVolumes  int32                `json:"num_storage_group_volumes"`
	Tracks                  int64                `json:"tracks"`
	NotSharedTracks         int64                `json:"non_shared_tracks"`
	TimeToLiveExpiryDate    string               `json:"time_to_live_expiry_date"`
	SecureExpiryDate        string               `json:"secure_expiry_date"`
	Expired                 bool                 `json:"expired"`
	Linked                  bool                 `json:"linked"`
	Restored                bool                 `json:"restored"`
	LinkedStorageGroupNames []string             `json:"linked_storage_group_names"`
	Persistent              bool                 `json:"persistent"`
	LinkedStorageGroups     []LinkedStorageGroup `json:"linked_storage_group"`
}

// LinkedStorageGroup linked storage group
type LinkedStorageGroup struct {
	Name                       string `json:"name"`
	SourceVolumeName           string `json:"source_volume_name"`
	LinkedVolumeName           string `json:"linked_volume_name"`
	Tracks                     int64  `json:"tracks"`
	TrackSize                  int64  `json:"trackSize"`
	PercentageCopied           int64  `json:"percentageCopied"`
	Defined                    bool   `json:"defined"`
	BackgroundDefineInProgress bool   `json:"background_define_in_progress"`
}

// SourceVolume the source of a volume
type SourceVolume struct {
	Name       string  `json:"name"`
	Capacity   int64   `json:"capacity"`
	CapacityGb float64 `json:"capacity_gb"`
}

// CreateStorageGroupSnapshot object to create a storage group snapshot
type CreateStorageGroupSnapshot struct {
	SnapshotName    string `json:"snapshotName"`
	ExecutionOption string `json:"executionOption"`
	TimeToLive      int32  `json:"timeToLive,omitempty"`
	Secure          int32  `json:"secure,omitempty"`
	TimeInHours     bool   `json:"force,omitempty"`
	Star            bool   `json:"start,omitempty"`
	Bothsides       bool   `json:"bothsides,omitempty"`
}

// ModifyStorageGroupSnapshot Modify a Storage Group snap
type ModifyStorageGroupSnapshot struct {
	ExecutionOption string                   `json:"executionOption,omitempty"`
	Action          string                   `json:"action"`
	Restore         RestoreSnapshotAction    `json:"restore,omitempty"`
	Link            LinkSnapshotAction       `json:"link,omitempty"`
	Relink          RelinkSnapshotAction     `json:"relink,omitempty"`
	Unlink          UnlinkSnapshotAction     `json:"unlink,omitempty"`
	SetMode         SetModeSnapshotAction    `json:"set_mode,omitempty"`
	Rename          RenameSnapshotAction     `json:"rename,omitempty"`
	TimeToLive      TimeToLiveSnapshotAction `json:"time_to_live,omitempty"`
	Secure          SecureSnapshotAction     `json:"secure,omitempty"`
	Persist         PresistSnapshotAction    `json:"persist,omitempty"`
}

// RenameStorageGroupSnapshot Modify a Storage Group snap to rename
type RenameStorageGroupSnapshot struct {
	ExecutionOption string               `json:"executionOption,omitempty"`
	Action          string               `json:"action"`
	Rename          RenameSnapshotAction `json:"rename"`
}

// RestoreStorageGroupSnapshot Modify a Storage Group snap to restore
type RestoreStorageGroupSnapshot struct {
	ExecutionOption string                `json:"executionOption,omitempty"`
	Action          string                `json:"action"`
	Restore         RestoreSnapshotAction `json:"restore"`
}

// LinkStorageGroupSnapshot Modify a Storage Group snap to link
type LinkStorageGroupSnapshot struct {
	ExecutionOption string             `json:"executionOption,omitempty"`
	Action          string             `json:"action"`
	Link            LinkSnapshotAction `json:"link"`
}

// RelinkStorageGroupSnapshot Modify a Storage Group snap to relink
type RelinkStorageGroupSnapshot struct {
	ExecutionOption string               `json:"executionOption,omitempty"`
	Action          string               `json:"action"`
	Relink          RelinkSnapshotAction `json:"relink"`
}

// UnlinkStorageGroupSnapshot Modify a Storage Group snap to unlink
type UnlinkStorageGroupSnapshot struct {
	ExecutionOption string               `json:"executionOption,omitempty"`
	Action          string               `json:"action"`
	Unlink          UnlinkSnapshotAction `json:"unlink"`
}

// SetModeStorageGroupSnapshot Modify a Storage Group snaps set mode
type SetModeStorageGroupSnapshot struct {
	ExecutionOption string                `json:"executionOption,omitempty"`
	Action          string                `json:"action"`
	SetMode         SetModeSnapshotAction `json:"set_mode"`
}

// TimeToLiveStorageGroupSnapshot Modify a Storage Group snaps time to live
type TimeToLiveStorageGroupSnapshot struct {
	ExecutionOption string                   `json:"executionOption,omitempty"`
	Action          string                   `json:"action"`
	TimeToLive      TimeToLiveSnapshotAction `json:"time_to_live"`
}

// SecureStorageGroupSnapshot Modify a Storage Group snap be secure
type SecureStorageGroupSnapshot struct {
	ExecutionOption string               `json:"executionOption,omitempty"`
	Action          string               `json:"action"`
	Secure          SecureSnapshotAction `json:"secure"`
}

// PersistStorageGroupSnapshot Modify a Storage Group snap to persist
type PersistStorageGroupSnapshot struct {
	ExecutionOption string                `json:"executionOption,omitempty"`
	Action          string                `json:"action"`
	Persist         PresistSnapshotAction `json:"persist"`
}

// RestoreSnapshotAction an action on a Storage Group snap
type RestoreSnapshotAction struct {
	Force  bool `json:"force,omitempty"`
	Star   bool `json:"star,omitempty"`
	Remote bool `json:"remote,omitempty"`
}

// LinkSnapshotAction an action on a Storage Group snap
type LinkSnapshotAction struct {
	Force            bool   `json:"force,omitempty"`
	Star             bool   `json:"star,omitempty"`
	Remote           bool   `json:"remote,omitempty"`
	StorageGroupName string `json:"storage_group_name"`
	NoCompression    bool   `json:"no_compression,omitempty"`
	Exact            bool   `json:"exact,omitempty"`
	Copy             bool   `json:"copy,omitempty"`
}

// RelinkSnapshotAction an action on a Storage Group snap
type RelinkSnapshotAction struct {
	Force            bool   `json:"force,omitempty"`
	Star             bool   `json:"star,omitempty"`
	Remote           bool   `json:"remote,omitempty"`
	StorageGroupName string `json:"storage_group_name"`
	Exact            bool   `json:"exact,omitempty"`
	Copy             bool   `json:"copy,omitempty"`
}

// UnlinkSnapshotAction an action on a Storage Group snap
type UnlinkSnapshotAction struct {
	Force            bool   `json:"force,omitempty"`
	Star             bool   `json:"star,omitempty"`
	Symforce         bool   `json:"symforce,omitempty"`
	StorageGroupName string `json:"storage_group_name"`
}

// SetModeSnapshotAction an action on a Storage Group snap
type SetModeSnapshotAction struct {
	Force            bool   `json:"force,omitempty"`
	Star             bool   `json:"star,omitempty"`
	StorageGroupName string `json:"storage_group_name"`
	Copy             bool   `json:"copy,omitempty"`
}

// RenameSnapshotAction an action on a Storage Group snap
type RenameSnapshotAction struct {
	Force                       bool   `json:"force,omitempty"`
	Star                        bool   `json:"star,omitempty"`
	NewStorageGroupSnapshotName string `json:"new_snapshot_name"`
}

// TimeToLiveSnapshotAction an action on a Storage Group snap
type TimeToLiveSnapshotAction struct {
	Force       bool  `json:"force,omitempty"`
	Star        bool  `json:"star,omitempty"`
	TimeToLive  int32 `json:"time_to_live,omitempty"`
	TimeInHours bool  `json:"time_in_hours,omitempty"`
}

// SecureSnapshotAction an action on a Storage Group snap
type SecureSnapshotAction struct {
	Force       bool  `json:"force,omitempty"`
	Star        bool  `json:"star,omitempty"`
	Secure      int32 `json:"secure,omitempty"`
	TimeInHours bool  `json:"time_in_hours,omitempty"`
}

// PresistSnapshotAction an action on a Storage Group snap
type PresistSnapshotAction struct {
	Force   bool `json:"force,omitempty"`
	Star    bool `json:"star,omitempty"`
	Remote  bool `json:"remote,omitempty"`
	Persist bool `json:"persist,omitempty"`
}

// SnapID list of snap ids related to a Storage Group snapshot
type SnapID struct {
	SnapIDs []int64 `json:"snapids"`
}

// SymDevice list of devices on a particular symmetrix system
type SymDevice struct {
	SymmetrixID string     `json:"symmetrixId"`
	Name        string     `json:"name"`
	Snapshot    []Snapshot `json:"snapshot"`
	RdfgNumbers []int64    `json:"rdfgNumbers"`
}

// Snapshot contains information for a snapshot
type Snapshot struct {
	Name       string `json:"name"`
	Generation int64  `json:"generation"`
	Linked     bool   `json:"linked"`
	Restored   bool   `json:"restored"`
	Timestamp  string `json:"timestamp"`
	State      string `json:"state"`
}

// SymVolumeList contains information on private volume get
type SymVolumeList struct {
	Name      []string    `json:"name"`
	SymDevice []SymDevice `json:"device"`
}

// SymmetrixCapability holds replication capabilities
type SymmetrixCapability struct {
	SymmetrixID   string `json:"symmetrixId"`
	SnapVxCapable bool   `json:"snapVxCapable"`
	RdfCapable    bool   `json:"rdfCapable"`
}

// SymReplicationCapabilities holds whether or not snapshot is licensed
type SymReplicationCapabilities struct {
	SymmetrixCapability []SymmetrixCapability `json:"symmetrixCapability"`
	Successful          bool                  `json:"successful,omitempty"`
	FailMessage         string                `json:"failMessage,omitempty"`
}

// PrivVolumeResultList : volume list resulted
type PrivVolumeResultList struct {
	PrivVolumeList []VolumeResultPrivate `json:"result"`
	From           int                   `json:"from"`
	To             int                   `json:"to"`
}

// PrivVolumeIterator : holds the iterator of resultant volume list
type PrivVolumeIterator struct {
	ResultList PrivVolumeResultList `json:"resultList"`
	ID         string               `json:"id"`
	Count      int                  `json:"count"`
	// What units is ExpirationTime in?
	ExpirationTime int64 `json:"expirationTime"`
	MaxPageSize    int   `json:"maxPageSize"`
}

// VolumeResultPrivate holds private volume information
type VolumeResultPrivate struct {
	VolumeHeader   VolumeHeader   `json:"volumeHeader"`
	TimeFinderInfo TimeFinderInfo `json:"timeFinderInfo"`
}

// VolumeHeader holds private volume header information
type VolumeHeader struct {
	VolumeID              string   `json:"volumeId"`
	NameModifier          string   `json:"nameModifier"`
	FormattedName         string   `json:"formattedName"`
	PhysicalDeviceName    string   `json:"physicalDeviceName"`
	Configuration         string   `json:"configuration"`
	SRP                   string   `json:"SRP"`
	ServiceLevel          string   `json:"serviceLevel"`
	ServiceLevelBaseName  string   `json:"serviceLevelBaseName"`
	Workload              string   `json:"workload"`
	StorageGroup          []string `json:"storageGroup"`
	FastStorageGroup      string   `json:"fastStorageGroup"`
	ServiceState          string   `json:"serviceState"`
	Status                string   `json:"status"`
	CapTB                 float64  `json:"capTB"`
	CapGB                 float64  `json:"capGB"`
	CapMB                 float64  `json:"capMB"`
	BlockSize             int64    `json:"blockSize"`
	AllocatedPercent      int64    `json:"allocatedPercent"`
	EmulationType         string   `json:"emulationType"`
	SystemResource        bool     `json:"system_resource"`
	Encapsulated          bool     `json:"encapsulated"`
	BCV                   bool     `json:"BCV"`
	SplitName             string   `json:"splitName"`
	SplitSerialNumber     string   `json:"splitSerialNumber"`
	FBA                   bool     `json:"FBA"`
	CKD                   bool     `json:"CKD"`
	Mapped                bool     `json:"mapped"`
	Private               bool     `json:"private"`
	DataDev               bool     `json:"dataDev"`
	VVol                  bool     `json:"VVol"`
	MobilityID            bool     `json:"mobilityID"`
	Meta                  bool     `json:"meta"`
	MetaHead              bool     `json:"metaHead"`
	NumSymDevMaskingViews int64    `json:"numSymDevMaskingViews"`
	NumStorageGroups      int64    `json:"numStorageGroups"`
	NumDGs                int64    `json:"numDGs"`
	NumCGs                int64    `json:"numCGs"`
	Lun                   string   `json:"lun"`
	MetaConfigNumber      int64    `json:"metaConfigNumber"`
	WWN                   string   `json:"wwn"`
	HasEffectiveWWN       bool     `json:"hasEffectiveWWN"`
	EffectiveWWN          string   `json:"effectiveWWN"`
	PersistentAllocation  string   `json:"persistentAllocation"`
	CUImageNum            string   `json:"CUImageNum"`
	CUImageStatus         string   `json:"CUImageStatus"`
	SSID                  string   `json:"SSID"`
	CUImageBaseAddress    string   `json:"CUImageBaseAddress"`
	PAVMode               string   `json:"PAVMode"`
	FEDirPorts            []string `json:"FEDirPorts"`
	CompressionEnabled    bool     `json:"compressionEnabled"`
	CompressionRatio      string   `json:"compressionRatio"`
}

// TimeFinderInfo contains snap information for a volume
type TimeFinderInfo struct {
	SnapSource    bool            `json:"snapSource"`
	SnapTarget    bool            `json:"snapTarget"`
	SnapVXSrc     bool            `json:"snapVXSrc"`
	SnapVXTgt     bool            `json:"snapVXTgt"`
	Mirror        bool            `json:"mirror"`
	CloneSrc      bool            `json:"cloneSrc"`
	CloneTarget   bool            `json:"cloneTarget"`
	SnapVXSession []SnapVXSession `json:"snapVXSession"`
	CloneSession  []CloneSession  `json:"cloneSession"`
	MirrorSession []MirrorSession `json:"MirrorSession"`
}

// SnapVXSession holds snapshot session information
type SnapVXSession struct {
	SourceSnapshotGenInfo       []SourceSnapshotGenInfo      `json:"srcSnapshotGenInfo"`
	LinkSnapshotGenInfo         []LinkSnapshotGenInfo        `json:"lnkSnapshotGenInfo"`
	TargetSourceSnapshotGenInfo *TargetSourceSnapshotGenInfo `json:"tgtSrcSnapshotGenInfo"`
}

// SourceSnapshotGenInfo contains source snapshot generation info
type SourceSnapshotGenInfo struct {
	SnapshotHeader      SnapshotHeader        `json:"snapshotHeader"`
	LinkSnapshotGenInfo []LinkSnapshotGenInfo `json:"lnkSnapshotGenInfo"`
}

// SnapshotHeader contians information for snapshot header
type SnapshotHeader struct {
	Device       string `json:"device"`
	SnapshotName string `json:"snapshotName"`
	Generation   int64  `json:"generation"`
	Secured      bool   `json:"secured"`
	Expired      bool   `json:"expired"`
	TimeToLive   int64  `json:"timeToLive"`
	Timestamp    int64  `json:"timestamp"`
}

// LinkSnapshotGenInfo contains information on snapshot generation for links
type LinkSnapshotGenInfo struct {
	TargetDevice  string `json:"targetDevice"`
	State         string `json:"state"`
	Restored      bool   `json:"restored"`
	Defined       bool   `json:"defined"`
	Destaged      bool   `json:"destaged"`
	BackgroundDef bool   `json:"backgroundDef"`
}

// TargetSourceSnapshotGenInfo contains information on target snapshot generation
type TargetSourceSnapshotGenInfo struct {
	TargetDevice string `json:"targetDevice"`
	SourceDevice string `json:"sourceDevice"`
	SnapshotName string `json:"snapshotName"`
	Generation   int64  `json:"generation"`
	Secured      bool   `json:"secured"`
	Expired      bool   `json:"expired"`
	TimeToLive   int64  `json:"timeToLive"`
	Timestamp    int64  `json:"timestamp"`
	Defined      string `json:"state"`
}

// CloneSession contains information on a clone session
type CloneSession struct {
	SourceVolume  string `json:"sourceVolume"`
	TargetVolume  string `json:"targetVolume"`
	Timestamp     int64  `json:"timestamp"`
	State         string `json:"state"`
	RemoteVolumes string `json:"remoteVolumes"`
}

// MirrorSession contains info about mirrored session
type MirrorSession struct {
	Timestamp    int64  `json:"timestamp"`
	State        string `json:"state"`
	SourceVolume string `json:"sourceVolume"`
	TargetVolume string `json:"targetVolume"`
}

// SnapTarget contains target information
type SnapTarget struct {
	Target  string
	Defined bool
	CpMode  bool
}
