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

// RDFGroup contains information about an RDF group
type RDFGroup struct {
	RdfgNumber               int      `json:"rdfgNumber"`
	Label                    string   `json:"label"`
	RemoteRdfgNumber         int      `json:"remoteRdfgNumber"`
	RemoteSymmetrix          string   `json:"remoteSymmetrix"`
	NumDevices               int      `json:"numDevices"`
	TotalDeviceCapacity      float64  `json:"totalDeviceCapacity"`
	LocalPorts               []string `json:"localPorts"`
	RemotePorts              []string `json:"remotePorts"`
	Modes                    []string `json:"modes"`
	Type                     string   `json:"type"`
	Metro                    bool     `json:"metro"`
	Async                    bool     `json:"async"`
	Witness                  bool     `json:"witness"`
	WitnessName              string   `json:"witnessName"`
	WitnessProtectedPhysical bool     `json:"witnessProtectedPhysical"`
	WitnessProtectedVirtual  bool     `json:"witnessProtectedVirtual"`
	WitnessConfigured        bool     `json:"witnessConfigured"`
	WitnessEffective         bool     `json:"witnessEffective"`
	BiasConfigured           bool     `json:"biasConfigured"`
	BiasEffective            bool     `json:"biasEffective"`
	WitnessDegraded          bool     `json:"witnessDegraded"`
	LocalOnlinePorts         []string `json:"localOnlinePorts"`
	RemoteOnlinePorts        []string `json:"remoteOnlinePorts"`
	DevicePolarity           string   `json:"device_polarity"`
	Offline                  bool     `json:"offline"`
}

// RDFGroupIDL contains the RDF group when we list RDF groups
type RDFGroupIDL struct {
	RDFGNumber  int    `json:"rdfgNumber"`
	Label       string `json:"label"`
	RemoteSymID string `json:"remote_symmetrix_id"`
	GroupType   string `json:"group_type"`
}

// RDFGroupList has list of RDF group
type RDFGroupList struct {
	RDFGroupCount int           `json:"rdfg_count"`
	RDFGroupIDs   []RDFGroupIDL `json:"rdfGroupID"`
}

// RDFPortDetails has RDF ports details
type RDFPortDetails struct {
	SymmID     string `json:"symmetrixID"`
	DirNum     int    `json:"directorNumber"`
	DirID      string `json:"directorId"`
	PortNum    int    `json:"portNumber"`
	PortOnline bool   `json:"online"`
	PortWWN    string `json:"wwn"`
}

// RDFGroupCreate RDF Group Create Action
type RDFGroupCreate struct {
	Label        string           `json:"label"`
	LocalRDFNum  int              `json:"local_rdfg_number"`
	LocalPorts   []RDFPortDetails `json:"local_ports"`
	RemoteRDFNum int              `json:"remote_rdfg_number"`
	RemotePorts  []RDFPortDetails `json:"remote_ports"`
}

// RDFDirList gets a List of RDF Directors
type RDFDirList struct {
	RdfDirs []string `json:"directorID"`
}

// RDFDirDetails gets details of a given RDF Director
type RDFDirDetails struct {
	SymID           string `json:"symmetrixID"`
	DirNum          int    `json:"directorNumber"`
	DirID           string `json:"directorId"`
	DirOnline       string `json:"online"`
	DirProtocolFC   bool   `json:"fiber"`
	DirProtocolGigE bool   `json:"gige"`
	DirHWCompress   bool   `json:"hwCompressionSupported"`
}

// RDFPortList gets a List of RDF Ports
type RDFPortList struct {
	RdfPorts []string `json:"portNumber"`
}

// RemoteRDFPortDetails gets a list of Remote Directors:Port that are zoned to a given Local RDF Port.
type RemoteRDFPortDetails struct {
	RemotePorts []RDFPortDetails `json:"remotePort"`
}

// NextFreeRDFGroup - Free RDFg contains information about the Next free RDFg in R1 and R2
type NextFreeRDFGroup struct {
	LocalRdfGroup  []int `json:"rdfg_number"`
	RemoteRdfGroup []int `json:"remote_rdfg_number"`
}

// Suspend action
type Suspend struct {
	Force      bool `json:"force"`
	SymForce   bool `json:"symForce"`
	Star       bool `json:"star"`
	Hop2       bool `json:"hop2"`
	Bypass     bool `json:"bypass"`
	Immediate  bool `json:"immediate"`
	ConsExempt bool `json:"consExempt"`
	MetroBias  bool `json:"metroBias"`
}

// Resume action
type Resume struct {
	Force        bool `json:"force"`
	SymForce     bool `json:"symForce"`
	Star         bool `json:"star"`
	Hop2         bool `json:"hop2"`
	Bypass       bool `json:"bypass"`
	Remote       bool `json:"remote"`
	RecoverPoint bool `json:"recoverPoint,omitempty"`
}

// Failover action
type Failover struct {
	Force     bool `json:"force"`
	SymForce  bool `json:"symForce"`
	Star      bool `json:"star"`
	Hop2      bool `json:"hop2"`
	Bypass    bool `json:"bypass"`
	Immediate bool `json:"immediate"`
	Establish bool `json:"establish"`
	Restore   bool `json:"restore"`
	Remote    bool `json:"remote"`
}

// Swap action
type Swap struct {
	Force     bool `json:"force"`
	SymForce  bool `json:"symForce"`
	Star      bool `json:"star"`
	Hop2      bool `json:"hop2"`
	Bypass    bool `json:"bypass"`
	HalfSwap  bool `json:"halfSwap"`
	RefreshR1 bool `json:"refreshR1"`
	RefreshR2 bool `json:"refreshR2"`
}

// Failback action
type Failback struct {
	Force        bool `json:"force"`
	SymForce     bool `json:"symForce"`
	Star         bool `json:"star"`
	Hop2         bool `json:"hop2"`
	Bypass       bool `json:"bypass"`
	Remote       bool `json:"remote"`
	RecoverPoint bool `json:"recoverPoint,omitempty"`
}

// Establish action
type Establish struct {
	Force     bool `json:"force"`
	SymForce  bool `json:"symForce"`
	Star      bool `json:"star"`
	Hop2      bool `json:"hop2"`
	Bypass    bool `json:"bypass"`
	Full      bool `json:"full"`
	MetroBias bool `json:"metroBias"`
}

// ModifySGRDFGroup holds parameters for rdf storage group updates
type ModifySGRDFGroup struct {
	Action          string     `json:"action"`
	Establish       *Establish `json:"establish,omitempty"`
	Suspend         *Suspend   `json:"suspend,omitempty"`
	Resume          *Resume    `json:"resume,omitempty"`
	Failback        *Failback  `json:"failback,omitempty"`
	Failover        *Failover  `json:"failover,omitempty"`
	Swap            *Swap      `json:"swap,omitempty"`
	ExecutionOption string     `json:"executionOption"`
}

// CreateSGSRDF contains parameters to create storage group replication {in u4p a.k.a "storageGroupSrdfCreate"}
type CreateSGSRDF struct {
	RemoteSymmID           string `json:"remoteSymmId"`
	ReplicationMode        string `json:"replicationMode"`
	RdfgNumber             int    `json:"rdfgNumber"`
	ForceNewRdfGroup       string `json:"forceNewRdfGroup"`
	Establish              bool   `json:"establish"`
	MetroBias              bool   `json:"metroBias"`
	RemoteStorageGroupName string `json:"remoteStorageGroupName"`
	ThinPool               string `json:"thinPool"`
	FastPolicy             string `json:"fastPolicy"`
	RemoteSLO              string `json:"remoteSLO"`
	NoCompression          bool   `json:"noCompression"`
	ExecutionOption        string `json:"executionOption"`
}

// SGRDFInfo contains parameters to hold srdf information of a storage group {in u4p a.k.a "storageGroupRDFg"}
type SGRDFInfo struct {
	SymmetrixID               string   `json:"symmetrixId"`
	StorageGroupName          string   `json:"storageGroupName"`
	RdfGroupNumber            int      `json:"rdfGroupNumber"`
	VolumeRdfTypes            []string `json:"volumeRdfTypes"`
	States                    []string `json:"states"`
	Modes                     []string `json:"modes"`
	Hop2Rdfgs                 []int    `json:"hop2Rdfgs"`
	Hop2States                []string `json:"hop2States"`
	Hop2Modes                 []string `json:"hop2Modes"`
	LargerRdfSides            []string `json:"largerRdfSides"`
	TotalTracks               int      `json:"totalTracks"`
	CapacityMB                float64  `json:"capacity_mb"`
	LocalR1InvalidTracksHop1  int      `json:"localR1InvalidTracksHop1"`
	LocalR2InvalidTracksHop1  int      `json:"localR2InvalidTracksHop1"`
	RemoteR1InvalidTracksHop1 int      `json:"remoteR1InvalidTracksHop1"`
	RemoteR2InvalidTracksHop1 int      `json:"remoteR2InvalidTracksHop1"`
	SrcR1InvalidTracksHop2    int      `json:"srcR1InvalidTracksHop2"`
	SrcR2InvalidTracksHop2    int      `json:"srcR2InvalidTracksHop2"`
	TgtR1InvalidTracksHop2    int      `json:"tgtR1InvalidTracksHop2"`
	TgtR2InvalidTracksHop2    int      `json:"tgtR2InvalidTracksHop2"`
	Domino                    []string `json:"domino"`
	ConsistencyProtection     string   `json:"consistency_protection"`
	ConsistencyProtectionHop2 string   `json:"consistency_protection_hop2"`
}

// SGRDFGList contains list of all RDF enabled storage groups {in u4p a.k.a "storageGroupRDFg"}
type SGRDFGList struct {
	RDFGList []string `json:"rdfgs"`
}

// RDFStorageGroup contains information about protected SG {in u4p a.k.a "StorageGroup"}
type RDFStorageGroup struct {
	Name                        string                  `json:"name"`
	SymmetrixID                 string                  `json:"symmetrixId"`
	ParentName                  string                  `json:"parentName"`
	ChildNames                  []string                `json:"childNames"`
	NumDevicesNonGk             int                     `json:"numDevicesNonGk"`
	CapacityGB                  float64                 `json:"capacityGB"`
	NumSnapVXSnapshots          int                     `json:"numSnapVXSnapshots"`
	SnapVXSnapshots             []string                `json:"snapVXSnapshots"`
	NumCloudSnapshots           int                     `json:"num_cloud_snapshots"`
	Rdf                         bool                    `json:"rdf"`
	IsLinkTarget                bool                    `json:"isLinkTarget"`
	SnapshotPolicies            []string                `json:"snapshot_policies"`
	RDFGroups                   []int                   `json:"rdf_groups"`
	NumCloneTargetStorageGroups int                     `json:"num_clone_target_storage_groups"`
	RemoteStorageGroups         []RemoteRDFStorageGroup `json:"remote_storage_groups"`
}

// RemoteRDFStorageGroup holds information about remote storage groups
type RemoteRDFStorageGroup struct {
	SymmetrixID      string `json:"symmetrix_id"`
	StorageGroupID   string `json:"storage_group_id"`
	StorageGroupUUID string `json:"storage_group_uuid"`
}

// LocalDeviceAutoCriteria holds parameters for auto selecting local device parameters
type LocalDeviceAutoCriteria struct {
	PairCount          int    `json:"pairCount"`
	Emulation          string `json:"emulation"`
	Capacity           int64  `json:"capacity"`
	CapacityUnit       string `json:"capacityUnit"`
	LocalThinPoolName  string `json:"localThinPoolName"`
	RemoteThinPoolName string `json:"remoteThinPoolName"`
}

// LocalDeviceListCriteria holds parameters for local device lis
type LocalDeviceListCriteria struct {
	LocalDeviceList    []string `json:"localDeviceList"`
	RemoteThinPoolName string   `json:"remoteThinPoolName"`
}

// CreateRDFPair holds SG create replica pair parameters
type CreateRDFPair struct {
	RdfMode                 string                   `json:"rdfMode"`
	RdfType                 string                   `json:"rdfType"`
	InvalidateR1            bool                     `json:"invalidateR1"`
	InvalidateR2            bool                     `json:"invalidateR2"`
	Establish               bool                     `json:"establish"`
	Restore                 bool                     `json:"restore"`
	Format                  bool                     `json:"format"`
	Exempt                  bool                     `json:"exempt"`
	NoWD                    bool                     `json:"noWD"`
	Remote                  bool                     `json:"remote"`
	Bias                    bool                     `json:"bias"`
	RecoverPoint            bool                     `json:"recoverPoint,omitempty"`
	LocalDeviceAutoCriteria *LocalDeviceAutoCriteria `json:"localDeviceAutoCriteriaParam"`
	LocalDeviceListCriteria *LocalDeviceListCriteria `json:"localDeviceListCriteriaParam"`
	ExecutionOption         string                   `json:"executionOption"`
}

// RDFDevicePair holds RDF volume pair information
type RDFDevicePair struct {
	LocalSymmID          string `json:"localSymmetrixId"`
	RemoteSymmID         string `json:"remoteSymmetrixId"`
	LocalRdfGroupNumber  int    `json:"localRdfGroupNumber"`
	RemoteRdfGroupNumber int    `json:"remoteRdfGroupNumber"`
	LocalVolumeName      string `json:"localVolumeName"`
	RemoteVolumeName     string `json:"remoteVolumeName"`
	LocalVolumeState     string `json:"localVolumeState"`
	RemoteVolumeState    string `json:"remoteVolumeState"`
	VolumeConfig         string `json:"volumeConfig"`
	RdfMode              string `json:"rdfMode"`
	RdfpairState         string `json:"rdfpairState"`
	LargerRdfSide        string `json:"largerRdfSide"`
	LocalWWNExternal     string `json:"local_wwn_external"`
	RemoteWWNExternal    string `json:"remote_wwn_external"`
}

// RDFDevicePairList holds list of newly created RDF volume pair information
type RDFDevicePairList struct {
	RDFDevicePair []RDFDevicePair `json:"devicePair"`
}

// StorageGroupRDFG holds information about protected storage group
type StorageGroupRDFG struct {
	SymmetrixID      string   `json:"symmetrixId"`
	StorageGroupName string   `json:"storageGroupName"`
	RdfGroupNumber   int      `json:"rdfGroupNumber"`
	VolumeRdfTypes   []string `json:"volumeRdfTypes"`
	States           []string `json:"states"`
	Modes            []string `json:"modes"`
	LargerRdfSides   []string `json:"largerRdfSides"`
}
