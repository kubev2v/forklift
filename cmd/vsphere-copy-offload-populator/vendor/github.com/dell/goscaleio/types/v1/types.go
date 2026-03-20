// Copyright © 2019 - 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package goscaleio

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const errorWithDetails = "Error with details"
const (
	trueType  = "TRUE"
	falseType = "FALSE"
)

// GetBoolType returns the true and false strings expected by the REST API
func GetBoolType(b bool) string {
	if b {
		return trueType
	}
	return falseType
}

// ErrorMessageDetails defines contents of an error msg
type ErrorMessageDetails struct {
	Error        string `json:"error"`
	ReturnCode   int    `json:"rc"`
	ErrorMessage string `json:"errorMessage"`
}

// Error struct defines the structure of an error
type Error struct {
	Message        string                `json:"message"`
	HTTPStatusCode int                   `json:"httpStatusCode"`
	ErrorCode      int                   `json:"errorCode"`
	ErrorDetails   []ErrorMessageDetails `json:"details"`
}

func (e Error) Error() string {
	if e.Message == errorWithDetails && len(e.ErrorDetails) > 0 {
		fmt.Printf("goscaleio.Error Error with details  %#v\n", e)
		if e.ErrorDetails[0].ErrorMessage != "" {
			e.Message = e.ErrorDetails[0].ErrorMessage
			return e.ErrorDetails[0].ErrorMessage
		}
		if e.ErrorDetails[0].Error != "" {
			translation := TranslateErrorCodeToErrorMessage(e.ErrorDetails[0].Error)
			if translation != "" {
				e.Message = translation
				return translation
			}
		}
		// No ErrorMessage or Error string, have to punt
	}
	return e.Message
}

// type session struct {
// 	Link []*types.Link `xml:"Link"`
// }

// System defines struct of PowerFlex array
type System struct {
	MdmMode                               string   `json:"mdmMode"`
	MdmClusterState                       string   `json:"mdmClusterState"`
	SecondaryMdmActorIPList               []string `json:"secondaryMdmActorIpList"`
	InstallID                             string   `json:"installId"`
	PrimaryActorIPList                    []string `json:"primaryMdmActorIpList"`
	SystemVersionName                     string   `json:"systemVersionName"`
	CapacityAlertHighThresholdPercent     int      `json:"capacityAlertHighThresholdPercent"`
	CapacityAlertCriticalThresholdPercent int      `json:"capacityAlertCriticalThresholdPercent"`
	RemoteReadOnlyLimitState              bool     `json:"remoteReadOnlyLimitState"`
	PrimaryMdmActorPort                   int      `json:"primaryMdmActorPort"`
	SecondaryMdmActorPort                 int      `json:"secondaryMdmActorPort"`
	TiebreakerMdmActorPort                int      `json:"tiebreakerMdmActorPort"`
	MdmManagementPort                     int      `json:"mdmManagementPort"`
	TiebreakerMdmIPList                   []string `json:"tiebreakerMdmIpList"`
	MdmManagementIPList                   []string `json:"mdmManagementIPList"`
	DefaultIsVolumeObfuscated             bool     `json:"defaultIsVolumeObfuscated"`
	RestrictedSdcModeEnabled              bool     `json:"restrictedSdcModeEnabled"`
	RestrictedSdcMode                     string   `json:"restrictedSdcMode"`
	Swid                                  string   `json:"swid"`
	DaysInstalled                         int      `json:"daysInstalled"`
	MaxCapacityInGb                       string   `json:"maxCapacityInGb"`
	CapacityTimeLeftInDays                string   `json:"capacityTimeLeftInDays"`
	EnterpriseFeaturesEnabled             bool     `json:"enterpriseFeaturesEnabled"`
	IsInitialLicense                      bool     `json:"isInitialLicense"`
	Name                                  string   `json:"name"`
	ID                                    string   `json:"id"`
	Links                                 []*Link  `json:"links"`
	PerformanceProfile                    string   `json:"perfProfile"`
}

// Constants representing cluster mode
const (
	FiveNodesClusterMode  = "FiveNodes"
	ThreeNodesClusterMode = "ThreeNodes"
)

// Constants representing MDM role
const (
	Manager    = "Manager"
	TieBreaker = "TieBreaker"
)

// MdmCluster defines struct for MDM cluster
type MdmCluster struct {
	ID              string   `json:"id"`
	ClusterState    string   `json:"clusterState"`
	ClusterMode     string   `json:"clusterMode"`
	GoodNodesNum    int      `json:"goodNodesNum"`
	GoodReplicasNum int      `json:"goodReplicasNum"`
	VirtualIPs      []string `json:"virtualIps"`
	PrimaryMDM      Mdm      `json:"master"`
	SecondaryMDM    []Mdm    `json:"slaves"`
	TiebreakerMdm   []Mdm    `json:"tieBreakers"`
	StandByMdm      []Mdm    `json:"standbyMDMs"`
}

// Mdm defines struct for a MDM
type Mdm struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Port              int      `json:"port"`
	IPs               []string `json:"ips"`
	ManagementIPs     []string `json:"managementIPs"`
	Role              string   `json:"role"`
	Status            string   `json:"status"`
	VirtualInterfaces []string `json:"virtualInterfaces"`
	VersionInfo       string   `json:"versionInfo"`
	OpenSslVersion    string   `json:"opensslVersion"`
}

// StandByMdm defines struct for StandBy MDM
type StandByMdm struct {
	Name               string   `json:"name,omitempty"`
	Port               string   `json:"port,omitempty"`
	IPs                []string `json:"ips"`
	ManagementIPs      []string `json:"managementIps,omitempty"`
	Role               string   `json:"role"`
	VirtualInterfaces  []string `json:"virtIpIntfs,omitempty"`
	ForceClean         string   `json:"forceClean,omitempty"`
	AllowAsymmetricIps string   `json:"allowAsymmetricIps,omitempty"`
}

// SetRestrictedMode defines struct for setting restricted mode
type SetRestrictedMode struct {
	RestrictedSdcMode string `json:"restrictedSdcMode"`
}

// SetApprovedIps defines struct for setting approved ips
type SetApprovedIps struct {
	SdcID          string   `json:"sdcId"`
	SdcApprovedIps []string `json:"sdcApprovedIps"`
}

// RemoveStandByMdmParam defines struct for removing standby MDM
type RemoveStandByMdmParam struct {
	ID string `json:"id"`
}

// ChangeMdmOwnerShip defines struct for changing MDM ownership
type ChangeMdmOwnerShip struct {
	ID string `json:"id"`
}

// ChangeMdmPerfProfile defines struct for modifying performance profile
type ChangeMdmPerfProfile struct {
	PerfProfile string `json:"perfProfile"`
}

// SwitchClusterMode defines struct for switching cluster mode
type SwitchClusterMode struct {
	Mode                string   `json:"mode"`
	AddSecondaryMdms    []string `json:"addSlaveMdmIdList,omitempty"`
	AddTBMdms           []string `json:"addTBIdList,omitempty"`
	RemoveSecondaryMdms []string `json:"removeSlaveMdmIdList,omitempty"`
	RemoveTBMdms        []string `json:"removeTBIdList,omitempty"`
}

// RenameMdm defines struct for modifying MDM name
type RenameMdm struct {
	ID      string `json:"id"`
	NewName string `json:"newName"`
}

// Link defines struct of Link
type Link struct {
	Rel   string `json:"rel"`
	HREF  string `json:"href"`
	Title string `json:"title,omitempty"`
	Type  string `json:"type,omitempty"`
}

// BWC defines struct of BWC
type BWC struct {
	TotalWeightInKb int `json:"totalWeightInKb"`
	NumOccured      int `json:"numOccured"`
	NumSeconds      int `json:"numSeconds"`
}

// Statistics defines struct of Statistics for PowerFlex Array
type Statistics struct {
	PrimaryReadFromDevBwc                    BWC `json:"primaryReadFromDevBwc"`
	NumOfStoragePools                        int `json:"numOfStoragePools"`
	ProtectedCapacityInKb                    int `json:"protectedCapacityInKb"`
	MovingCapacityInKb                       int `json:"movingCapacityInKb"`
	SnapCapacityInUseOccupiedInKb            int `json:"snapCapacityInUseOccupiedInKb"`
	SnapCapacityInUseInKb                    int `json:"snapCapacityInUseInKb"`
	ActiveFwdRebuildCapacityInKb             int `json:"activeFwdRebuildCapacityInKb"`
	DegradedHealthyVacInKb                   int `json:"degradedHealthyVacInKb"`
	ActiveMovingRebalanceJobs                int `json:"activeMovingRebalanceJobs"`
	TotalReadBwc                             BWC `json:"totalReadBwc"`
	MaxCapacityInKb                          int `json:"maxCapacityInKb"`
	PendingBckRebuildCapacityInKb            int `json:"pendingBckRebuildCapacityInKb"`
	ActiveMovingOutFwdRebuildJobs            int `json:"activeMovingOutFwdRebuildJobs"`
	CapacityLimitInKb                        int `json:"capacityLimitInKb"`
	SecondaryVacInKb                         int `json:"secondaryVacInKb"`
	PendingFwdRebuildCapacityInKb            int `json:"pendingFwdRebuildCapacityInKb"`
	ThinCapacityInUseInKb                    int `json:"thinCapacityInUseInKb"`
	AtRestCapacityInKb                       int `json:"atRestCapacityInKb"`
	ActiveMovingInBckRebuildJobs             int `json:"activeMovingInBckRebuildJobs"`
	DegradedHealthyCapacityInKb              int `json:"degradedHealthyCapacityInKb"`
	NumOfScsiInitiators                      int `json:"numOfScsiInitiators"`
	NumOfUnmappedVolumes                     int `json:"numOfUnmappedVolumes"`
	FailedCapacityInKb                       int `json:"failedCapacityInKb"`
	SecondaryReadFromDevBwc                  BWC `json:"secondaryReadFromDevBwc"`
	NumOfVolumes                             int `json:"numOfVolumes"`
	SecondaryWriteBwc                        BWC `json:"secondaryWriteBwc"`
	ActiveBckRebuildCapacityInKb             int `json:"activeBckRebuildCapacityInKb"`
	FailedVacInKb                            int `json:"failedVacInKb"`
	PendingMovingCapacityInKb                int `json:"pendingMovingCapacityInKb"`
	ActiveMovingInRebalanceJobs              int `json:"activeMovingInRebalanceJobs"`
	PendingMovingInRebalanceJobs             int `json:"pendingMovingInRebalanceJobs"`
	BckRebuildReadBwc                        BWC `json:"bckRebuildReadBwc"`
	DegradedFailedVacInKb                    int `json:"degradedFailedVacInKb"`
	NumOfSnapshots                           int `json:"numOfSnapshots"`
	RebalanceCapacityInKb                    int `json:"rebalanceCapacityInKb"`
	FwdRebuildReadBwc                        BWC `json:"fwdRebuildReadBwc"`
	NumOfSdc                                 int `json:"numOfSdc"`
	ActiveMovingInFwdRebuildJobs             int `json:"activeMovingInFwdRebuildJobs"`
	NumOfVtrees                              int `json:"numOfVtrees"`
	ThickCapacityInUseInKb                   int `json:"thickCapacityInUseInKb"`
	ProtectedVacInKb                         int `json:"protectedVacInKb"`
	PendingMovingInBckRebuildJobs            int `json:"pendingMovingInBckRebuildJobs"`
	CapacityAvailableForVolumeAllocationInKb int `json:"capacityAvailableForVolumeAllocationInKb"`
	VolumeAllocationLimitInKb                int `json:"volumeAllocationLimitInKb"`
	PendingRebalanceCapacityInKb             int `json:"pendingRebalanceCapacityInKb"`
	PendingMovingRebalanceJobs               int `json:"pendingMovingRebalanceJobs"`
	NumOfProtectionDomains                   int `json:"numOfProtectionDomains"`
	NumOfSds                                 int `json:"numOfSds"`
	CapacityInUseInKb                        int `json:"capacityInUseInKb"`
	BckRebuildWriteBwc                       BWC `json:"bckRebuildWriteBwc"`
	DegradedFailedCapacityInKb               int `json:"degradedFailedCapacityInKb"`
	NumOfThinBaseVolumes                     int `json:"numOfThinBaseVolumes"`
	PendingMovingOutFwdRebuildJobs           int `json:"pendingMovingOutFwdRebuildJobs"`
	SecondaryReadBwc                         BWC `json:"secondaryReadBwc"`
	PendingMovingOutBckRebuildJobs           int `json:"pendingMovingOutBckRebuildJobs"`
	RebalanceWriteBwc                        BWC `json:"rebalanceWriteBwc"`
	PrimaryReadBwc                           BWC `json:"primaryReadBwc"`
	NumOfVolumesInDeletion                   int `json:"numOfVolumesInDeletion"`
	NumOfDevices                             int `json:"numOfDevices"`
	RebalanceReadBwc                         BWC `json:"rebalanceReadBwc"`
	InUseVacInKb                             int `json:"inUseVacInKb"`
	UnreachableUnusedCapacityInKb            int `json:"unreachableUnusedCapacityInKb"`
	TotalWriteBwc                            BWC `json:"totalWriteBwc"`
	SpareCapacityInKb                        int `json:"spareCapacityInKb"`
	ActiveMovingOutBckRebuildJobs            int `json:"activeMovingOutBckRebuildJobs"`
	PrimaryVacInKb                           int `json:"primaryVacInKb"`
	NumOfThickBaseVolumes                    int `json:"numOfThickBaseVolumes"`
	BckRebuildCapacityInKb                   int `json:"bckRebuildCapacityInKb"`
	NumOfMappedToAllVolumes                  int `json:"numOfMappedToAllVolumes"`
	ActiveMovingCapacityInKb                 int `json:"activeMovingCapacityInKb"`
	PendingMovingInFwdRebuildJobs            int `json:"pendingMovingInFwdRebuildJobs"`
	ActiveRebalanceCapacityInKb              int `json:"activeRebalanceCapacityInKb"`
	RmcacheSizeInKb                          int `json:"rmcacheSizeInKb"`
	FwdRebuildCapacityInKb                   int `json:"fwdRebuildCapacityInKb"`
	FwdRebuildWriteBwc                       BWC `json:"fwdRebuildWriteBwc"`
	PrimaryWriteBwc                          BWC `json:"primaryWriteBwc"`
	NetUserDataCapacityInKb                  int `json:"netUserDataCapacityInKb"`
	NetUnusedCapacityInKb                    int `json:"netUnusedCapacityInKb"`
	VolumeAddressSpaceInKb                   int `json:"volumeAddressSpaceInKb"`
}

// OSRepository defines struct of OS Repository
type OSRepository struct {
	CreatedDate  string                    `json:"createdDate,omitempty"`
	ImageType    string                    `json:"imageType,omitempty"`
	SourcePath   string                    `json:"sourcePath,omitempty"`
	RazorName    string                    `json:"razorName,omitempty"`
	InUse        bool                      `json:"inUse,omitempty"`
	UserName     string                    `json:"username,omitempty"`
	CreatedBy    string                    `json:"createdBy,omitempty"`
	Password     string                    `json:"password,omitempty"`
	Name         string                    `json:"name,omitempty"`
	ID           string                    `json:"id,omitempty"`
	State        string                    `json:"state,omitempty"`
	RepoType     string                    `json:"repoType,omitempty"`
	RCMPath      string                    `json:"rcmPath,omitempty"`
	FirmwareRepo FirmwareRepositoryDetails `json:"firmwareRepository,omitempty"`
	Metadata     OSRepositoryMetadata      `json:"metadata,omitempty"`
	BaseURL      string                    `json:"baseUrl,omitempty"`
	FromWeb      bool                      `json:"fromWeb,omitempty"`
}

// OSRepositoryMetadata defines struct of Metadata for OS Repository
type OSRepositoryMetadata struct {
	Repos []OSRepositoryMetadataRepo `json:"repos,omitempty"`
}

// OSRepositoryMetadataRepo defines struct of OSRepositoryMetadataRepo
type OSRepositoryMetadataRepo struct {
	BasePath    string `json:"base_path,omitempty"`
	Description string `json:"description,omitempty"`
	GPGKey      string `json:"gpg_key,omitempty"`
	Name        string `json:"name,omitempty"`
	OSPackages  bool   `json:"os_packages,omitempty"`
}

// CompatibilityManagement defines struct of CompatibilityManagement
type CompatibilityManagement struct {
	ID                     string `json:"id,omitempty"`
	Source                 string `json:"source,omitempty"`
	RepositoryPath         string `json:"repositoryPath,omitempty"`
	CurrentVersion         string `json:"currentVersion,omitempty"`
	AvailableVersion       string `json:"availableVersion,omitempty"`
	CompatibilityData      string `json:"compatibilityData,omitempty"`
	CompatibilityDataBytes string `json:"compatibilityDataBytes,omitempty"`
}

// CompatibilityManagementPost defines struct of CompatibilityManagementPost Body
type CompatibilityManagementPost struct {
	ID                     string `json:"id,omitempty"`
	Source                 string `json:"source,omitempty"`
	RepositoryPath         string `json:"repositoryPath,omitempty"`
	CurrentVersion         string `json:"currentVersion,omitempty"`
	AvailableVersion       string `json:"availableVersion,omitempty"`
	CompatibilityData      string `json:"compatibilityData,omitempty"`
	CompatibilityDataBytes []byte `json:"compatibilityDataBytes,omitempty"`
}

// SdcStatistics defines struct of Statistics for PowerFlex SDC
type SdcStatistics struct {
	UserDataReadBwc         BWC      `json:"userDataReadBwc"`
	UserDataWriteBwc        BWC      `json:"userDataWriteBwc"`
	UserDataTrimBwc         BWC      `json:"userDataTrimBwc"`
	UserDataSdcReadLatency  BWC      `json:"userDataSdcReadLatency"`
	UserDataSdcWriteLatency BWC      `json:"userDataSdcWriteLatency"`
	UserDataSdcTrimLatency  BWC      `json:"userDataSdcTrimLatency"`
	VolumeIDs               []string `json:"volumeIds"`
	NumOfMappedVolumes      int      `json:"numOfMappedVolumes"`
}

// VolumeStatistics defines struct of Statistics for PowerFlex volume
type VolumeStatistics struct {
	UserDataReadBwc         BWC      `json:"userDataReadBwc"`
	UserDataWriteBwc        BWC      `json:"userDataWriteBwc"`
	UserDataTrimBwc         BWC      `json:"userDataTrimBwc"`
	UserDataSdcReadLatency  BWC      `json:"userDataSdcReadLatency"`
	UserDataSdcWriteLatency BWC      `json:"userDataSdcWriteLatency"`
	UserDataSdcTrimLatency  BWC      `json:"userDataSdcTrimLatency"`
	MappedSdcIDs            []string `json:"mappedSdcIds"`
	NumOfMappedSdcs         int      `json:"numOfMappedSdcs"`
}

// User defines struct of User for PowerFlex array
type User struct {
	SystemID               string  `json:"systemId"`
	UserRole               string  `json:"userRole"`
	PasswordChangeRequired bool    `json:"passwordChangeRequired"`
	Name                   string  `json:"name"`
	ID                     string  `json:"id"`
	Links                  []*Link `json:"links"`
}

// UserParam defines struct for creating a new user on the pflex array.
type UserParam struct {
	Name     string `json:"name"`
	UserRole string `json:"userRole"`
	Password string `json:"password"`
}

// UserResp defines struct for the response which you get after creating the user.
type UserResp struct {
	ID string `json:"id"`
}

// UserRoleParam defines struct for changing the user role.
type UserRoleParam struct {
	UserRole string `json:"userRole"`
}

// ScsiInitiator defines struct for ScsiInitiator
type ScsiInitiator struct {
	Name     string  `json:"name"`
	IQN      string  `json:"iqn"`
	SystemID string  `json:"systemID"`
	Links    []*Link `json:"links"`
}

// PDRfCacheOpMode is an enum type for Protection Domain Rf Cache Operational Mode
type PDRfCacheOpMode string

// Available values for enum type PDRfCacheOpMode
const (
	PDRCModeRead         PDRfCacheOpMode = "Read"
	PDRCModeWrite        PDRfCacheOpMode = "Write"
	PDRCModeReadAndWrite PDRfCacheOpMode = "ReadAndWrite"
	PDRCModeWriteMiss    PDRfCacheOpMode = "WriteMiss"
)

// PDCounterWindow defines one window for a Protection Domain Failure Counter
type PDCounterWindow struct {
	Threshold       int `json:"threshold"`
	WindowSizeInSec int `json:"windowSizeInSec"`
}

// PDCounterParams defines all the windows for a Protection Domain Failure Counter
type PDCounterParams struct {
	ShortWindow  PDCounterWindow `json:"shortWindow"`
	MediumWindow PDCounterWindow `json:"mediumWindow"`
	LongWindow   PDCounterWindow `json:"longWindow"`
}

// PDConnInfo defines Protection Domain Connection information
type PDConnInfo struct {
	ClientServerConnStatus string  `json:"clientServerConnStatus"`
	DisconnectedClientID   *string `json:"disconnectedClientId"`
	DisconnectedClientName *string `json:"disconnectedClientName"`
	DisconnectedServerID   *string `json:"disconnectedServerId"`
	DisconnectedServerName *string `json:"disconnectedServerName"`
	DisconnectedServerIP   *string `json:"disconnectedServerIp"`
}

// ProtectionDomain defines struct for PowerFlex ProtectionDomain
type ProtectionDomain struct {
	SystemID                    string     `json:"systemId"`
	SdrSdsConnectivityInfo      PDConnInfo `json:"sdrSdsConnectivityInfo"`
	ReplicationCapacityMaxRatio *int       `json:"replicationCapacityMaxRatio"`

	// SDS Network throttling params
	RebuildNetworkThrottlingInKbps                   int  `json:"rebuildNetworkThrottlingInKbps"`
	RebalanceNetworkThrottlingInKbps                 int  `json:"rebalanceNetworkThrottlingInKbps"`
	OverallIoNetworkThrottlingInKbps                 int  `json:"overallIoNetworkThrottlingInKbps"`
	VTreeMigrationNetworkThrottlingInKbps            int  `json:"vtreeMigrationNetworkThrottlingInKbps"`
	ProtectedMaintenanceModeNetworkThrottlingInKbps  int  `json:"protectedMaintenanceModeNetworkThrottlingInKbps"`
	OverallIoNetworkThrottlingEnabled                bool `json:"overallIoNetworkThrottlingEnabled"`
	RebuildNetworkThrottlingEnabled                  bool `json:"rebuildNetworkThrottlingEnabled"`
	RebalanceNetworkThrottlingEnabled                bool `json:"rebalanceNetworkThrottlingEnabled"`
	VTreeMigrationNetworkThrottlingEnabled           bool `json:"vtreeMigrationNetworkThrottlingEnabled"`
	ProtectedMaintenanceModeNetworkThrottlingEnabled bool `json:"protectedMaintenanceModeNetworkThrottlingEnabled"`

	// Fine Granularity Params
	FglDefaultNumConcurrentWrites int  `json:"fglDefaultNumConcurrentWrites"`
	FglMetadataCacheEnabled       bool `json:"fglMetadataCacheEnabled"`
	FglDefaultMetadataCacheSize   int  `json:"fglDefaultMetadataCacheSize"`

	// RfCache Params
	RfCacheEnabled         bool            `json:"rfcacheEnabled"`
	RfCacheAccpID          string          `json:"rfcacheAccpId"`
	RfCacheOperationalMode PDRfCacheOpMode `json:"rfcacheOpertionalMode"`
	RfCachePageSizeKb      int             `json:"rfcachePageSizeKb"`
	RfCacheMaxIoSizeKb     int             `json:"rfcacheMaxIoSizeKb"`

	// Counter Params
	SdsConfigurationFailureCP            PDCounterParams `json:"sdsConfigurationFailureCounter"`
	SdsDecoupledCP                       PDCounterParams `json:"sdsDecoupledCounterParameters"`
	MdmSdsNetworkDisconnectionsCP        PDCounterParams `json:"mdmSdsNetworkDisconnectionsCounterParameters"`
	SdsSdsNetworkDisconnectionsCP        PDCounterParams `json:"sdsSdsNetworkDisconnectionsCounterParameters"`
	SdsReceiveBufferAllocationFailuresCP PDCounterParams `json:"sdsReceiveBufferAllocationFailuresCounterParameters"`

	ProtectionDomainState string  `json:"protectionDomainState"`
	Name                  string  `json:"name"`
	ID                    string  `json:"id"`
	Links                 []*Link `json:"links"`
}

// ProtectionDomainParam defines struct for ProtectionDomainParam
type ProtectionDomainParam struct {
	Name string `json:"name"`
}

// ChangeSdcNameParam defines struct for passing parameters to changeSDCname endpoint
type ChangeSdcNameParam struct {
	SdcName string `json:"sdcName"`
}

// ChangeSdcPerfProfile defines struct for passing parameters to setSdcPerformanceParameters endpoint
type ChangeSdcPerfProfile struct {
	PerfProfile string `json:"perfProfile"`
}

// ApproveSdcParam defines struct for ApproveSdcParam
type ApproveSdcParam struct {
	SdcGUID string   `json:"sdcGuid,,omitempty"`
	SdcIP   string   `json:"sdcIp,omitempty"`
	SdcIps  []string `json:"sdcIps,omitempty"`
	Name    string   `json:"name,omitempty"`
}

// ApproveSdcByGUIDResponse defines struct for ApproveSdcByGUIDResponse
type ApproveSdcByGUIDResponse struct {
	SdcID string `json:"id"`
}

// ProtectionDomainResp defines struct for ProtectionDomainResp
type ProtectionDomainResp struct {
	ID string `json:"id"`
}

// Sdc defines struct for PowerFlex Sdc
type Sdc struct {
	SystemID           string   `json:"systemId"`
	SdcApproved        bool     `json:"sdcApproved"`
	SdcIP              string   `json:"SdcIp"`
	SdcIPs             []string `json:"SdcIps"`
	OnVMWare           bool     `json:"onVmWare"`
	SdcGUID            string   `json:"sdcGuid"`
	MdmConnectionState string   `json:"mdmConnectionState"`
	Name               string   `json:"name"`
	PerfProfile        string   `json:"perfProfile"`
	OSType             string   `json:"osType"`
	HostType           string   `json:"hostType"`
	ID                 string   `json:"id"`
	Links              []*Link  `json:"links"`
	SdcApprovedIPs     []string `json:"sdcApprovedIps"`
}

// SdsIP defines struct for SdsIP
type SdsIP struct {
	IP   string `json:"ip"`
	Role string `json:"role,omitempty"`
}

// SdsIPList defines struct for SdsIPList
type SdsIPList struct {
	SdsIP SdsIP `json:"SdsIp"`
}

// SdsWindowType defines struct for SdsWindowType
type SdsWindowType struct {
	Threshold            int `json:"threshold,omitempty"`
	WindowSizeInSec      int `json:"windowSizeInSec,omitempty"`
	LastOscillationCount int `json:"lastOscillationCount,omitempty"`
	LastOscillationTime  int `json:"lastOscillationTime,omitempty"`
	MaxFailuresCount     int `json:"maxFailuresCount,omitempty"`
}

// SdsWindow defines struct for SdsWindow
type SdsWindow struct {
	ShortWindow  SdsWindowType `json:"shortWindow,omitempty"`
	MediumWindow SdsWindowType `json:"mediumWindow,omitempty"`
	LongWindow   SdsWindowType `json:"longWindow,omitempty"`
}

// RaidControllers defines struct for raid controllers
type RaidControllers struct {
	SerialNumber    string `json:"serialNumber,omitempty"`
	ModelName       string `json:"modelName,omitempty"`
	VendorName      string `json:"vendorName,omitempty"`
	FirmwareVersion string `json:"firmwareVersion,omitempty"`
	DriverVersion   string `json:"driverVersion,omitempty"`
	DriverName      string `json:"driverName,omitempty"`
	PciAddress      string `json:"pciAddress,omitempty"`
	Status          string `json:"status,omitempty"`
	BatteryStatus   string `json:"batteryStatus,omitempty"`
}

// CertificateInfo defines struct for certificate information
type CertificateInfo struct {
	Subject             string `json:"subject,omitempty"`
	Issuer              string `json:"issuer,omitempty"`
	ValidFrom           string `json:"validFrom,omitempty"`
	ValidTo             string `json:"validTo,omitempty"`
	Thumbprint          string `json:"thumbprint,omitempty"`
	ValidFromAsn1Format string `json:"validFromAsn1Format,omitempty"`
	ValidToAsn1Format   string `json:"validToAsn1Format,omitempty"`
}

// Sds defines struct for Sds
type Sds struct {
	ID                                          string            `json:"id"`
	Name                                        string            `json:"name,omitempty"`
	ProtectionDomainID                          string            `json:"protectionDomainId"`
	IPList                                      []*SdsIP          `json:"ipList"`
	Port                                        int               `json:"port,omitempty"`
	SdsState                                    string            `json:"sdsState"`
	MembershipState                             string            `json:"membershipState"`
	MdmConnectionState                          string            `json:"mdmConnectionState"`
	DrlMode                                     string            `json:"drlMode,omitempty"`
	RmcacheEnabled                              bool              `json:"rmcacheEnabled,omitempty"`
	RmcacheSizeInKb                             int               `json:"rmcacheSizeInKb,omitempty"`
	RmcacheFrozen                               bool              `json:"rmcacheFrozen,omitempty"`
	IsOnVMware                                  bool              `json:"isOnVmWare,omitempty"`
	FaultSetID                                  string            `json:"faultSetId,omitempty"`
	NumOfIoBuffers                              int               `json:"numOfIoBuffers,omitempty"`
	RmcacheMemoryAllocationState                string            `json:"RmcacheMemoryAllocationState,omitempty"`
	PerformanceProfile                          string            `json:"perfProfile,omitempty"`
	SoftwareVersionInfo                         string            `json:"softwareVersionInfo,omitempty"`
	ConfiguredDrlMode                           string            `json:"configuredDrlMode,omitempty"`
	RfcacheEnabled                              bool              `json:"rfcacheEnabled,omitempty"`
	MaintenanceState                            string            `json:"maintenanceState,omitempty"`
	MaintenanceType                             string            `json:"maintenanceType,omitempty"`
	RfcacheErrorLowResources                    bool              `json:"rfcacheErrorLowResources,omitempty"`
	RfcacheErrorAPIVersionMismatch              bool              `json:"rfcacheErrorApiVersionMismatch,omitempty"`
	RfcacheErrorInconsistentCacheConfiguration  bool              `json:"rfcacheErrorInconsistentCacheConfiguration,omitempty"`
	RfcacheErrorInconsistentSourceConfiguration bool              `json:"rfcacheErrorInconsistentSourceConfiguration,omitempty"`
	RfcacheErrorInvalidDriverPath               bool              `json:"rfcacheErrorInvalidDriverPath,omitempty"`
	RfcacheErrorDeviceDoesNotExist              bool              `json:"rfcacheErrorDeviceDoesNotExist,omitempty"`
	AuthenticationError                         string            `json:"authenticationError,omitempty"`
	FglNumConcurrentWrites                      int               `json:"fglNumConcurrentWrites,omitempty"`
	FglMetadataCacheState                       string            `json:"fglMetadataCacheState,omitempty"`
	FglMetadataCacheSize                        int               `json:"fglMetadataCacheSize,omitempty"`
	NumRestarts                                 int               `json:"numRestarts,omitempty"`
	LastUpgradeTime                             int               `json:"lastUpgradeTime,omitempty"`
	SdsDecoupled                                SdsWindow         `json:"sdsDecoupled,omitempty"`
	SdsConfigurationFailure                     SdsWindow         `json:"sdsConfigurationFailure,omitempty"`
	SdsReceiveBufferAllocationFailures          SdsWindow         `json:"sdsReceiveBufferAllocationFailures,omitempty"`
	RaidControllers                             []RaidControllers `json:"raidControllers,omitempty"`
	CertificateInfo                             CertificateInfo   `json:"certificateInfo,omitempty"`
	Links                                       []*Link           `json:"links"`
}

// DeviceInfo defines struct for DeviceInfo
type DeviceInfo struct {
	DevicePath    string `json:"devicePath"`
	StoragePoolID string `json:"storagePoolId"`
	DeviceName    string `json:"deviceName,omitempty"`
}

// Constants representing states of SDS
const (
	SdsDrlModeVolatile        = "Volatile"
	SdsDrlModeNonVolatile     = "NonVolatile"
	PerformanceProfileHigh    = "HighPerformance"
	PerformanceProfileCompact = "Compact"
)

// SdsParam defines struct for SdsParam
type SdsParam struct {
	Name               string        `json:"name,omitempty"`
	IPList             []*SdsIPList  `json:"sdsIpList"`
	Port               string        `json:"sdsPort,omitempty"`
	DrlMode            string        `json:"drlMode,omitempty"`
	RmcacheEnabled     string        `json:"rmcacheEnabled,omitempty"`
	RmcacheSizeInKb    string        `json:"rmcacheSizeInKb,omitempty"`
	RmcacheFrozen      bool          `json:"rmcacheFrozen,omitempty"`
	ProtectionDomainID string        `json:"protectionDomainId"`
	FaultSetID         string        `json:"faultSetId,omitempty"`
	NumOfIoBuffers     string        `json:"numOfIoBuffers,omitempty"`
	DeviceInfoList     []*DeviceInfo `json:"deviceInfoList,omitempty"`
	ForceClean         bool          `json:"forceClean,omitempty"`
	DeviceTestTimeSecs int           `json:"deviceTestTimeSecs ,omitempty"`
	DeviceTestMode     string        `json:"deviceTestMode,omitempty"`
}

// SdsResp defines struct for SdsResp
type SdsResp struct {
	ID string `json:"id"`
}

// SdsIPRole defines struct for Sds IP and Role
type SdsIPRole struct {
	SdsIPToSet string `json:"sdsIpToSet"`
	NewRole    string `json:"newRole"`
}

// SdsName defines struct for Sds Name
type SdsName struct {
	Name string `json:"name"`
}

// SdsPort defines struct for Sds Port
type SdsPort struct {
	SdsPort string `json:"sdsPort"`
}

// ResourceCredentials defines struct for list of resource credential
type ResourceCredentials struct {
	TotalRecords int                  `json:"totalRecords"`
	Credentials  []ResourceCredential `json:"credentialList"`
}

// ResourceCredential defines struct for resource credential
type ResourceCredential struct {
	Link       Link    `json:"link"`
	Credential CredObj `json:"credential"`
	Reference  CredRef `json:"reference"`
}

// CredObj defines struct for credential object
type CredObj struct {
	Type        string `json:"type"`
	CreateDate  string `json:"createdDate"`
	CreatedBy   string `json:"createdBy"`
	UpdatedBy   string `json:"updatedBy"`
	UpdatedDate string `json:"updatedDate"`
	Label       string `json:"label"`
	Domain      string `json:"domain"`
	Link        Link   `json:"link"`
	Username    string `json:"username"`
	ID          string `json:"id"`
}

// CredRef defines struct for credential reference
type CredRef struct {
	Devices  int `json:"devices"`
	Policies int `json:"policies"`
}

// Device defines struct of Device for PowerFlex Array
type Device struct {
	FglNvdimmMetadataAmortizationX100 int                     `json:"fglNvdimmMetadataAmortizationX100,omitempty"`
	LogicalSectorSizeInBytes          int                     `json:"logicalSectorSizeInBytes,omitempty"`
	FglNvdimmWriteCacheSize           int                     `json:"fglNvdimmWriteCacheSize,omitempty"`
	AccelerationPoolID                string                  `json:"accelerationPoolId,omitempty"`
	RfcacheProps                      RfcachePropsParams      `json:"rfcacheProps,omitempty"`
	SdsID                             string                  `json:"sdsId"`
	StoragePoolID                     string                  `json:"storagePoolId"`
	CapacityLimitInKb                 int                     `json:"capacityLimitInKb,omitempty"`
	ErrorState                        string                  `json:"errorState,omitempty"`
	Capacity                          int                     `json:"capacity,omitempty"`
	DeviceType                        string                  `json:"deviceType,omitempty"`
	PersistentChecksumState           string                  `json:"persistentChecksumState,omitempty"`
	DeviceState                       string                  `json:"deviceState,omitempty"`
	LedSetting                        string                  `json:"ledSetting,omitempty"`
	MaxCapacityInKb                   int                     `json:"maxCapacityInKb,omitempty"`
	SpSdsID                           string                  `json:"spSdsId,omitempty"`
	LongSuccessfulIos                 LongSuccessfulIosParams `json:"longSuccessfulIos,omitempty"`
	AggregatedState                   string                  `json:"aggregatedState,omitempty"`
	TemperatureState                  string                  `json:"temperatureState,omitempty"`
	SsdEndOfLifeState                 string                  `json:"ssdEndOfLifeState,omitempty"`
	ModelName                         string                  `json:"modelName,omitempty"`
	VendorName                        string                  `json:"vendorName,omitempty"`
	RaidControllerSerialNumber        string                  `json:"raidControllerSerialNumber,omitempty"`
	FirmwareVersion                   string                  `json:"firmwareVersion,omitempty"`
	CacheLookAheadActive              bool                    `json:"cacheLookAheadActive,omitempty"`
	WriteCacheActive                  bool                    `json:"writeCacheActive,omitempty"`
	AtaSecurityActive                 bool                    `json:"ataSecurityActive,omitempty"`
	PhysicalSectorSizeInBytes         int                     `json:"physicalSectorSizeInBytes,omitempty"`
	MediaFailing                      bool                    `json:"mediaFailing,omitempty"`
	SlotNumber                        string                  `json:"slotNumber,omitempty"`
	ExternalAccelerationType          string                  `json:"externalAccelerationType,omitempty"`
	AutoDetectMediaType               string                  `json:"autoDetectMediaType,omitempty"`
	StorageProps                      StoragePropsParams      `json:"storageProps,omitempty"`
	AccelerationProps                 AccelerationPropsParams `json:"accelerationProps,omitempty"`
	DeviceCurrentPathName             string                  `json:"deviceCurrentPathName"`
	DeviceOriginalPathName            string                  `json:"deviceOriginalPathName,omitempty"`
	RfcacheErrorDeviceDoesNotExist    bool                    `json:"rfcacheErrorDeviceDoesNotExist,omitempty"`
	MediaType                         string                  `json:"mediaType,omitempty"`
	SerialNumber                      string                  `json:"serialNumber,omitempty"`
	Name                              string                  `json:"name,omitempty"`
	ID                                string                  `json:"id,omitempty"`
	Links                             []*Link                 `json:"links"`
}

// LongSuccessfulIosParams defines struct for Device
type LongSuccessfulIosParams struct {
	ShortWindow  DeviceWindowType `json:"shortWindow,omitempty"`
	MediumWindow DeviceWindowType `json:"mediumWindow,omitempty"`
	LongWindow   DeviceWindowType `json:"longWindow,omitempty"`
}

// DeviceWindowType defines struct for LongSuccessfulIosParams
type DeviceWindowType struct {
	Threshold            int `json:"threshold,omitempty"`
	WindowSizeInSec      int `json:"windowSizeInSec,omitempty"`
	LastOscillationCount int `json:"lastOscillationCount,omitempty"`
	LastOscillationTime  int `json:"lastOscillationTime,omitempty"`
	MaxFailuresCount     int `json:"maxFailuresCount,omitempty"`
}

// AccelerationPropsParams defines struct for Device
type AccelerationPropsParams struct {
	AccUsedCapacityInKb string `json:"accUsedCapacityInKb,omitempty"`
}

// RfcachePropsParams defines struct for Device
type RfcachePropsParams struct {
	DeviceUUID                     string `json:"deviceUuid,omitempty"`
	RfcacheErrorStuckIO            bool   `json:"rfcacheErrorStuckIo,omitempty"`
	RfcacheErrorHeavyLoadCacheSkip bool   `json:"rfcacheErrorHeavyLoadCacheSkip,omitempty"`
	RfcacheErrorCardIoError        bool   `json:"rfcacheErrorCardIoError,omitempty"`
}

// StoragePropsParams defines struct for Device
type StoragePropsParams struct {
	FglAccDeviceID                   string `json:"fglAccDeviceId,omitempty"`
	FglNvdimmSizeMb                  int    `json:"fglNvdimmSizeMb,omitempty"`
	DestFglNvdimmSizeMb              int    `json:"destFglNvdimmSizeMb,omitempty"`
	DestFglAccDeviceID               string `json:"destFglAccDeviceId,omitempty"`
	ChecksumMode                     string `json:"checksumMode,omitempty"`
	DestChecksumMode                 string `json:"destChecksumMode,omitempty"`
	ChecksumAccDeviceID              string `json:"checksumAccDeviceId,omitempty"`
	DestChecksumAccDeviceID          string `json:"destChecksumAccDeviceId,omitempty"`
	ChecksumSizeMb                   int    `json:"checksumSizeMb,omitempty"`
	IsChecksumFullyCalculated        bool   `json:"isChecksumFullyCalculated,omitempty"`
	ChecksumChangelogAccDeviceID     string `json:"checksumChangelogAccDeviceId,omitempty"`
	DestChecksumChangelogAccDeviceID string `json:"destChecksumChangelogAccDeviceId,omitempty"`
	ChecksumChangelogSizeMb          int    `json:"checksumChangelogSizeMb,omitempty"`
	DestChecksumChangelogSizeMb      int    `json:"destChecksumChangelogSizeMb,omitempty"`
}

// DeviceParam defines struct for DeviceParam
type DeviceParam struct {
	Name                     string `json:"name,omitempty"`
	DeviceCurrentPathname    string `json:"deviceCurrentPathname"`
	CapacityLimitInKb        int    `json:"capacityLimitInKb,omitempty"`
	StoragePoolID            string `json:"storagePoolId"`
	SdsID                    string `json:"sdsId"`
	TestTimeSecs             int    `json:"testTimeSecs,omitempty"`
	TestMode                 string `json:"testMode,omitempty"`
	MediaType                string `json:"mediaType,omitempty"`
	ExternalAccelerationType string `json:"externalAccelerationType,omitempty"`
}

// SetDeviceName defines struct for setting device name
type SetDeviceName struct {
	Name string `json:"newName"`
}

// SetDeviceMediaType defines struct for setting device media type
type SetDeviceMediaType struct {
	MediaType string `json:"mediaType"`
}

// SetDeviceExternalAccelerationType defines struct for device external acceleration type
type SetDeviceExternalAccelerationType struct {
	ExternalAccelerationType string `json:"externalAccelerationType"`
}

// SetDeviceCapacityLimit defines struct for setting device capacity limit
type SetDeviceCapacityLimit struct {
	DeviceCapacityLimit string `json:"capacityLimitInGB"`
}

// DeviceResp defines struct for DeviceParam
type DeviceResp struct {
	ID string `json:"id"`
}

// StoragePool defines struct for PowerFlex StoragePool
type StoragePool struct {
	ProtectionDomainID                                              string  `json:"protectionDomainId"`
	RebalanceioPriorityPolicy                                       string  `json:"rebalanceIoPriorityPolicy"`
	RebuildioPriorityPolicy                                         string  `json:"rebuildIoPriorityPolicy"`
	RebuildioPriorityBwLimitPerDeviceInKbps                         int     `json:"rebuildIoPriorityBwLimitPerDeviceInKbps"`
	RebuildioPriorityNumOfConcurrentIosPerDevice                    int     `json:"rebuildIoPriorityNumOfConcurrentIosPerDevice"`
	RebalanceioPriorityNumOfConcurrentIosPerDevice                  int     `json:"rebalanceIoPriorityNumOfConcurrentIosPerDevice"`
	RebalanceioPriorityBwLimitPerDeviceInKbps                       int     `json:"rebalanceIoPriorityBwLimitPerDeviceInKbps"`
	RebuildioPriorityAppIopsPerDeviceThreshold                      int     `json:"rebuildIoPriorityAppIopsPerDeviceThreshold"`
	RebalanceioPriorityAppIopsPerDeviceThreshold                    int     `json:"rebalanceIoPriorityAppIopsPerDeviceThreshold"`
	RebuildioPriorityAppBwPerDeviceThresholdInKbps                  int     `json:"rebuildIoPriorityAppBwPerDeviceThresholdInKbps"`
	RebalanceioPriorityAppBwPerDeviceThresholdInKbps                int     `json:"rebalanceIoPriorityAppBwPerDeviceThresholdInKbps"`
	RebuildioPriorityQuietPeriodInMsec                              int     `json:"rebuildIoPriorityQuietPeriodInMsec"`
	RebalanceioPriorityQuietPeriodInMsec                            int     `json:"rebalanceIoPriorityQuietPeriodInMsec"`
	ZeroPaddingEnabled                                              bool    `json:"zeroPaddingEnabled"`
	UseRmcache                                                      bool    `json:"useRmcache"`
	SparePercentage                                                 int     `json:"sparePercentage"`
	RmCacheWriteHandlingMode                                        string  `json:"rmcacheWriteHandlingMode"`
	RebuildEnabled                                                  bool    `json:"rebuildEnabled"`
	RebalanceEnabled                                                bool    `json:"rebalanceEnabled"`
	NumofParallelRebuildRebalanceJobsPerDevice                      int     `json:"numOfParallelRebuildRebalanceJobsPerDevice"`
	Name                                                            string  `json:"name"`
	ID                                                              string  `json:"id"`
	Links                                                           []*Link `json:"links"`
	BackgroundScannerBWLimitKBps                                    int     `json:"backgroundScannerBWLimitKBps"`
	ProtectedMaintenanceModeIoPriorityNumOfConcurrentIosPerDevice   int     `json:"protectedMaintenanceModeIoPriorityNumOfConcurrentIosPerDevice"`
	DataLayout                                                      string  `json:"dataLayout"`
	VtreeMigrationIoPriorityBwLimitPerDeviceInKbps                  int     `json:"vtreeMigrationIoPriorityBwLimitPerDeviceInKbps"`
	VtreeMigrationIoPriorityPolicy                                  string  `json:"vtreeMigrationIoPriorityPolicy"`
	AddressSpaceUsage                                               string  `json:"addressSpaceUsage"`
	ExternalAccelerationType                                        string  `json:"externalAccelerationType"`
	PersistentChecksumState                                         string  `json:"persistentChecksumState"`
	UseRfcache                                                      bool    `json:"useRfcache"`
	ChecksumEnabled                                                 bool    `json:"checksumEnabled"`
	CompressionMethod                                               string  `json:"compressionMethod"`
	FragmentationEnabled                                            bool    `json:"fragmentationEnabled"`
	CapacityUsageState                                              string  `json:"capacityUsageState"`
	CapacityUsageType                                               string  `json:"capacityUsageType"`
	AddressSpaceUsageType                                           string  `json:"addressSpaceUsageType"`
	BgScannerCompareErrorAction                                     string  `json:"bgScannerCompareErrorAction"`
	BgScannerReadErrorAction                                        string  `json:"bgScannerReadErrorAction"`
	ReplicationCapacityMaxRatio                                     int     `json:"replicationCapacityMaxRatio"`
	PersistentChecksumEnabled                                       bool    `json:"persistentChecksumEnabled"`
	PersistentChecksumBuilderLimitKb                                int     `json:"persistentChecksumBuilderLimitKb"`
	PersistentChecksumValidateOnRead                                bool    `json:"persistentChecksumValidateOnRead"`
	VtreeMigrationIoPriorityNumOfConcurrentIosPerDevice             int     `json:"vtreeMigrationIoPriorityNumOfConcurrentIosPerDevice"`
	ProtectedMaintenanceModeIoPriorityPolicy                        string  `json:"protectedMaintenanceModeIoPriorityPolicy"`
	BackgroundScannerMode                                           string  `json:"backgroundScannerMode"`
	MediaType                                                       string  `json:"mediaType"`
	CapacityAlertHighThreshold                                      int     `json:"capacityAlertHighThreshold"`
	CapacityAlertCriticalThreshold                                  int     `json:"capacityAlertCriticalThreshold"`
	VtreeMigrationIoPriorityAppIopsPerDeviceThreshold               int     `json:"vtreeMigrationIoPriorityAppIopsPerDeviceThreshold"`
	VtreeMigrationIoPriorityAppBwPerDeviceThresholdInKbps           int     `json:"vtreeMigrationIoPriorityAppBwPerDeviceThresholdInKbps"`
	VtreeMigrationIoPriorityQuietPeriodInMsec                       int     `json:"vtreeMigrationIoPriorityQuietPeriodInMsec"`
	FglAccpID                                                       string  `json:"fglAccpId"`
	FglExtraCapacity                                                int     `json:"fglExtraCapacity"`
	FglOverProvisioningFactor                                       int     `json:"fglOverProvisioningFactor"`
	FglWriteAtomicitySize                                           int     `json:"fglWriteAtomicitySize"`
	FglNvdimmWriteCacheSizeInMb                                     int     `json:"fglNvdimmWriteCacheSizeInMb"`
	FglNvdimmMetadataAmortizationX100                               int     `json:"fglNvdimmMetadataAmortizationX100"`
	FglPerfProfile                                                  string  `json:"fglPerfProfile"`
	ProtectedMaintenanceModeIoPriorityBwLimitPerDeviceInKbps        int     `json:"protectedMaintenanceModeIoPriorityBwLimitPerDeviceInKbps"`
	ProtectedMaintenanceModeIoPriorityAppIopsPerDeviceThreshold     int     `json:"protectedMaintenanceModeIoPriorityAppIopsPerDeviceThreshold"`
	ProtectedMaintenanceModeIoPriorityAppBwPerDeviceThresholdInKbps int     `json:"protectedMaintenanceModeIoPriorityAppBwPerDeviceThresholdInKbps"`
	ProtectedMaintenanceModeIoPriorityQuietPeriodInMsec             int     `json:"protectedMaintenanceModeIoPriorityQuietPeriodInMsec"`
}

// StoragePoolParam defines struct for StoragePoolParam
type StoragePoolParam struct {
	Name                     string `json:"name"`
	SparePercentage          string `json:"sparePercentage,omitempty"`
	RebuildEnabled           bool   `json:"rebuildEnabled,omitempty"`
	RebalanceEnabled         bool   `json:"rebalanceEnabled,omitempty"`
	ProtectionDomainID       string `json:"protectionDomainId"`
	ZeroPaddingEnabled       string `json:"zeroPaddingEnabled,omitempty"`
	UseRmcache               string `json:"useRmcache,omitempty"`
	UseRfcache               string `json:"useRfcache,omitempty"`
	RmcacheWriteHandlingMode string `json:"rmcacheWriteHandlingMode,omitempty"`
	MediaType                string `json:"mediaType,omitempty"`
}

// ModifyStoragePoolName defines struct for ModifyStoragePoolName
type ModifyStoragePoolName struct {
	Name string `json:"name"`
}

// StoragePoolMediaType defines struct for StoragePoolMediaType
type StoragePoolMediaType struct {
	MediaType string `json:"mediaType"`
}

// StoragePoolUseRmCache defines struct for StoragePoolUseRmCache
type StoragePoolUseRmCache struct {
	UseRmcache string `json:"useRmcache"`
}

// StoragePoolUseRfCache defines struct for StoragePoolUseRfCache
type StoragePoolUseRfCache struct{}

// StoragePoolZeroPadEnabled defines struct for zero Pad Enablement
type StoragePoolZeroPadEnabled struct {
	ZeroPadEnabled string `json:"zeroPadEnabled"`
}

// ReplicationJournalCapacityParam defines struct for Replication Journal Capacity
type ReplicationJournalCapacityParam struct {
	ReplicationJournalCapacityMaxRatio string `json:"replicationJournalCapacityMaxRatio"`
}

// CapacityAlertThresholdParam defines struct for Capacity Alert Threshold
type CapacityAlertThresholdParam struct {
	CapacityAlertHighThresholdPercent     string `json:"capacityAlertHighThresholdPercent,omitempty"`
	CapacityAlertCriticalThresholdPercent string `json:"capacityAlertCriticalThresholdPercent,omitempty"`
}

// ProtectedMaintenanceModeParam defines struct for Protected Maintenance Mode
type ProtectedMaintenanceModeParam struct {
	Policy                      string `json:"policy"`
	NumOfConcurrentIosPerDevice string `json:"numOfConcurrentIosPerDevice,omitempty"`
	BwLimitPerDeviceInKbps      string `json:"bwLimitPerDeviceInKbps,omitempty"`
}

// RebalanceEnabledParam defines struct for Rebalance Enablement
type RebalanceEnabledParam struct {
	RebalanceEnabled string `json:"rebalanceEnabled"`
}

// SparePercentageParam defines struct for Spare Percentage
type SparePercentageParam struct {
	SparePercentage string `json:"sparePercentage"`
}

// RmcacheWriteHandlingModeParam defines struct for Rmcache Write Handling Mode
type RmcacheWriteHandlingModeParam struct {
	RmcacheWriteHandlingMode string `json:"rmcacheWriteHandlingMode"`
}

// RebuildEnabledParam defines struct for Rebuild Enablement
type RebuildEnabledParam struct {
	RebuildEnabled string `json:"rebuildEnabled"`
}

// RebuildRebalanceParallelismParam defines struct for Rebuild Rebalance Parallelism
type RebuildRebalanceParallelismParam struct {
	Limit string `json:"limit"`
}

// FragmentationParam defines struct for fragmentation
type FragmentationParam struct{}

// StoragePoolResp defines struct for StoragePoolResp
type StoragePoolResp struct {
	ID string `json:"id"`
}

// MappedSdcInfo defines struct for MappedSdcInfo
type MappedSdcInfo struct {
	SdcID                 string `json:"sdcId"`
	SdcIP                 string `json:"sdcIp"`
	LimitIops             int    `json:"limitIops"`
	LimitBwInMbps         int    `json:"limitBwInMbps"`
	SdcName               string `json:"sdcName"`
	AccessMode            string `json:"accessMode"`
	IsDirectBufferMapping bool   `json:"isDirectBufferMapping"`
}

// Volume defines struct for Volume
type Volume struct {
	StoragePoolID                      string           `json:"storagePoolId"`
	UseRmCache                         bool             `json:"useRmcache"`
	MappingToAllSdcsEnabled            bool             `json:"mappingToAllSdcsEnabled"`
	MappedSdcInfo                      []*MappedSdcInfo `json:"mappedSdcInfo"`
	IsObfuscated                       bool             `json:"isObfuscated"`
	VolumeType                         string           `json:"volumeType"`
	ConsistencyGroupID                 string           `json:"consistencyGroupId"`
	VTreeID                            string           `json:"vtreeId"`
	AncestorVolumeID                   string           `json:"ancestorVolumeId"`
	MappedScsiInitiatorInfo            string           `json:"mappedScsiInitiatorInfo"`
	SizeInKb                           int              `json:"sizeInKb"`
	CreationTime                       int              `json:"creationTime"`
	Name                               string           `json:"name"`
	ID                                 string           `json:"id"`
	DataLayout                         string           `json:"dataLayout"`
	NotGenuineSnapshot                 bool             `json:"notGenuineSnapshot"`
	AccessModeLimit                    string           `json:"accessModeLimit"`
	SecureSnapshotExpTime              int              `json:"secureSnapshotExpTime"`
	ManagedBy                          string           `json:"managedBy"`
	LockedAutoSnapshot                 bool             `json:"lockedAutoSnapshot"`
	LockedAutoSnapshotMarkedForRemoval bool             `json:"lockedAutoSnapshotMarkedForRemoval"`
	CompressionMethod                  string           `json:"compressionMethod"`
	TimeStampIsAccurate                bool             `json:"timeStampIsAccurate"`
	OriginalExpiryTime                 int              `json:"originalExpiryTime"`
	VolumeReplicationState             string           `json:"volumeReplicationState"`
	ReplicationJournalVolume           bool             `json:"replicationJournalVolume"`
	ReplicationTimeStamp               int              `json:"replicationTimeStamp"`
	Links                              []*Link          `json:"links"`
}

// VolumeParam defines struct for VolumeParam
type VolumeParam struct {
	ProtectionDomainID string    `json:"protectionDomainId,omitempty"`
	StoragePoolID      string    `json:"storagePoolId,omitempty"`
	UseRmCache         string    `json:"useRmcache,omitempty"`
	VolumeType         string    `json:"volumeType,omitempty"`
	VolumeSizeInKb     string    `json:"volumeSizeInKb,omitempty"`
	Name               string    `json:"name,omitempty"`
	CompressionMethod  string    `json:"compressionMethod,omitempty"`
	once               sync.Once // creates the metadata value once.
	metadata           http.Header
}

// MetaData returns the metadata headers.
func (vp *VolumeParam) MetaData() http.Header {
	vp.once.Do(func() {
		vp.metadata = make(http.Header)
	})
	return vp.metadata
}

// SetVolumeSizeParam defines struct for SetVolumeSizeParam
type SetVolumeSizeParam struct {
	SizeInGB string `json:"sizeInGB,omitempty"`
}

// SetVolumeNameParam defines struct for SetVolumeNameParam
type SetVolumeNameParam struct {
	NewName string `json:"newName,omitempty"`
}

// VolumeResp defines struct for SetVolumeNameParam
type VolumeResp struct {
	ID string `json:"id"`
}

// VolumeQeryIDByKeyParam defines struct for VolumeQeryIDByKeyParam
type VolumeQeryIDByKeyParam struct {
	Name string `json:"name"`
}

// VolumeQeryBySelectedIDsParam defines struct for VolumeQeryBySelectedIDsParam
type VolumeQeryBySelectedIDsParam struct {
	IDs []string `json:"ids"`
}

// MapVolumeSdcParam defines struct for MapVolumeSdcParam
type MapVolumeSdcParam struct {
	SdcID                 string `json:"sdcId,omitempty"`
	AllowMultipleMappings string `json:"allowMultipleMappings,omitempty"`
	AllSdcs               string `json:"allSdcs,omitempty"`
	AccessMode            string `json:"accessMode,omitempty"`
}

// UnmapVolumeSdcParam defines struct for UnmapVolumeSdcParam
type UnmapVolumeSdcParam struct {
	SdcID                string `json:"sdcId,omitempty"`
	IgnoreScsiInitiators string `json:"ignoreScsiInitiators,omitempty"`
	AllSdcs              string `json:"allSdcs,omitempty"`
}

// SetMappedSdcLimitsParam defines struct for SetMappedSdcLimitsParam
type SetMappedSdcLimitsParam struct {
	SdcID                string `json:"sdcId,omitempty"`
	BandwidthLimitInKbps string `json:"bandwidthLimitInKbps,omitempty"`
	IopsLimit            string `json:"iopsLimit,omitempty"`
}

// RenameSdcParam defines struct for RenameSdc
type RenameSdcParam struct {
	SdcName string `json:"sdcName,omitempty"`
}

// GetSdcIDByIPParam defines struct for SDC ID to get by IP
type GetSdcIDByIPParam struct {
	IP string `json:"ip,omitempty"`
}

// SnapshotDef defines struct for SnapshotDef
type SnapshotDef struct {
	VolumeID     string `json:"volumeId,omitempty"`
	SnapshotName string `json:"snapshotName,omitempty"`
}

// SnapshotVolumesParam defines struct for SnapshotVolumesParam
type SnapshotVolumesParam struct {
	SnapshotDefs         []*SnapshotDef `json:"snapshotDefs"`
	RetentionPeriodInMin string         `json:"retentionPeriodInMin,omitempty"`
	AccessMode           string         `json:"accessModeLimit,omitempty"`
	AllowOnExtManagedVol bool           `json:"allowOnExtManagedVol,omitempty"`
}

// SnapshotVolumesResp defines struct for SnapshotVolumesResp
type SnapshotVolumesResp struct {
	VolumeIDList    []string `json:"volumeIdList"`
	SnapshotGroupID string   `json:"snapshotGroupId"`
}

// VTree defines struct for VTree
type VTree struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	BaseVolumeID  string  `json:"baseVolumeId"`
	StoragePoolID string  `json:"storagePoolId"`
	Links         []*Link `json:"links"`
}

// RemoveVolumeParam defines struct for RemoveVolumeParam
type RemoveVolumeParam struct {
	RemoveMode string `json:"removeMode"`
}

// EmptyPayload defines struct for EmptyPayload
type EmptyPayload struct{}

// SnapshotPolicy defines the struct for SnapshotPolicy
type SnapshotPolicy struct {
	SnapshotPolicyState                   string  `json:"snapshotPolicyState"`
	AutoSnapshotCreationCadenceInMin      int     `json:"autoSnapshotCreationCadenceInMin"`
	MaxVTreeAutoSnapshots                 int     `json:"maxVTreeAutoSnapshots"`
	NumOfSourceVolumes                    int     `json:"numOfSourceVolumes"`
	NumOfExpiredButLockedSnapshots        int     `json:"numOfExpiredButLockedSnapshots"`
	NumOfCreationFailures                 int     `json:"numOfCreationFailures"`
	NumOfRetainedSnapshotsPerLevel        []int   `json:"numOfRetainedSnapshotsPerLevel"`
	SnapshotAccessMode                    string  `json:"snapshotAccessMode"`
	SecureSnapshots                       bool    `json:"secureSnapshots"`
	TimeOfLastAutoSnapshot                int     `json:"timeOfLastAutoSnapshot"`
	NextAutoSnapshotCreationTime          int     `json:"nextAutoSnapshotCreationTime"`
	TimeOfLastAutoSnapshotCreationFailure int     `json:"timeOfLastAutoSnapshotCreationFailure"`
	LastAutoSnapshotCreationFailureReason string  `json:"lastAutoSnapshotCreationFailureReason"`
	LastAutoSnapshotFailureInFirstLevel   bool    `json:"lastAutoSnapshotFailureInFirstLevel"`
	NumOfAutoSnapshots                    int     `json:"numOfAutoSnapshots"`
	NumOfLockedSnapshots                  int     `json:"numOfLockedSnapshots"`
	SystemID                              string  `json:"systemId"`
	Name                                  string  `json:"name"`
	ID                                    string  `json:"id"`
	Links                                 []*Link `json:"links"`
}

// SnapshotPolicyCreateParam defines the struct for creating a Snapshot Policy
type SnapshotPolicyCreateParam struct {
	AutoSnapshotCreationCadenceInMin string   `json:"autoSnapshotCreationCadenceInMin"`
	NumOfRetainedSnapshotsPerLevel   []string `json:"numOfRetainedSnapshotsPerLevel"`
	SnapshotAccessMode               string   `json:"snapshotAccessMode,omitempty"`
	SecureSnapshots                  string   `json:"secureSnapshots,omitempty"`
	Name                             string   `json:"name"`
	Paused                           string   `json:"paused,omitempty"`
}

// SnapShotPolicyCreateResp defines struct for the response when snapshot policy is created successfully
type SnapShotPolicyCreateResp struct {
	ID string `json:"id"`
}

// SnapshotPolicyRenameParam defines the struct for renaming a Snapshot Policy
type SnapshotPolicyRenameParam struct {
	NewName string `json:"newName"`
}

// SnapshotPolicyModifyParam defines the struct for modifying a Snapshot Policy
type SnapshotPolicyModifyParam struct {
	AutoSnapshotCreationCadenceInMin string   `json:"autoSnapshotCreationCadenceInMin"`
	NumOfRetainedSnapshotsPerLevel   []string `json:"numOfRetainedSnapshotsPerLevel"`
}

// AssignVolumeToSnapshotPolicyParam defines the struct for assigning volume to a Snapshot Policy
type AssignVolumeToSnapshotPolicyParam struct {
	SourceVolumeID            string `json:"sourceVolumeId"`
	AutoSnapshotRemovalAction string `json:"autoSnapshotRemovalAction,omitempty"`
}

// SnapshotPolicyQueryIDByKeyParam defines struct for SnapshotPolicyQueryIDByKeyParam
type SnapshotPolicyQueryIDByKeyParam struct {
	Name string `json:"name"`
}

// PeerMDM defines a replication peer system.
type PeerMDM struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Port                int             `json:"port"`
	PeerSystemID        string          `json:"peerSystemId"`
	SystemID            string          `json:"systemId"`
	SoftwareVersionInfo string          `json:"softwareVersionInfo"`
	MembershipState     string          `json:"membershipState"`
	PerfProfile         string          `json:"perfProfile"`
	NetworkType         string          `json:"networkType"`
	CouplingRC          string          `json:"couplingRC"`
	IPList              []*IPListNoRole `json:"ipList"`
}

// ModifyPeerMdmIPParam defines struct for ModifyPeerMdmIPParam
type ModifyPeerMdmIPParam struct {
	NewPeerMDMIps []map[string]interface{} `json:"newPeerSystemIps"`
}

// ModifyPeerMDMNameParam defines struct for ModifyPeerMDMNameParam
type ModifyPeerMDMNameParam struct {
	NewName string `json:"newName"`
}

// ModifyPeerMDMPortParam defines struct for ModifyPeerMDMPortParam
type ModifyPeerMDMPortParam struct {
	NewPort string `json:"newPort"`
}

// ModifyPeerMdmPerformanceParametersParam defines struct for ModifyPeerMdmPerformanceParametersParam
type ModifyPeerMdmPerformanceParametersParam struct {
	NewPreformanceProfile string `json:"perfProfile"`
}

// AddPeerMdm defines struct for AddPeerMdm
type AddPeerMdm struct {
	PeerSystemID  string   `json:"peerSystemId"`
	PeerSystemIps []string `json:"peerSystemIps"`
	Port          string   `json:"port"`
	Name          string   `json:"name"`
}

// AddPeerMdmParam defines struct for AddPeerMdm
type AddPeerMdmParam struct {
	PeerSystemID  string                   `json:"peerSystemId"`
	PeerSystemIps []map[string]interface{} `json:"peerSystemIps"`
	Port          string                   `json:"port"`
	Name          string                   `json:"name"`
}

// ReplicationConsistencyGroup (RCG) has information about a replication session
type ReplicationConsistencyGroup struct {
	ID                       string `json:"id,omitempty"`
	Name                     string `json:"name"`
	RpoInSeconds             int    `json:"rpoInSeconds"`
	ProtectionDomainID       string `json:"protectionDomainId"`
	RemoteProtectionDomainID string `json:"remoteProtectionDomainId"`
	DestinationSystemID      string `json:"destinationSystemId,omitempty"`
	PeerMdmID                string `json:"peerMdmId,omitempty"`

	RemoteID                    string `json:"remoteId,omitempty"`
	RemoteMdmID                 string `json:"remoteMdmId,omitempty"`
	ReplicationDirection        string `json:"replicationDirection,omitempty"`
	CurrConsistMode             string `json:"currConsistMode,omitempty"`
	FreezeState                 string `json:"freezeState,omitempty"`
	PauseMode                   string `json:"pauseMode,omitempty"`
	LifetimeState               string `json:"lifetimeState,omitempty"`
	SnapCreationInProgress      bool   `json:"snapCreationInProgress,omitempty"`
	LastSnapGroupID             string `json:"lastSnapGroupId,omitempty"`
	Type                        string `json:"type,omitempty"`
	DisasterRecoveryState       string `json:"disasterRecoveryState,omitempty"`
	RemoteDisasterRecoveryState string `json:"remoteDisasterRecoveryState,omitempty"`
	TargetVolumeAccessMode      string `json:"targetVolumeAccessMode,omitempty"`
	FailoverType                string `json:"failoverType,omitempty"`
	FailoverState               string `json:"failoverState,omitempty"`
	ActiveLocal                 bool   `json:"activeLocal,omitempty"`
	ActiveRemote                bool   `json:"activeRemote,omitempty"`
	AbstractState               string `json:"abstractState,omitempty"`
	Error                       int    `json:"error,omitempty"`
	LocalActivityState          string `json:"localActivityState,omitempty"`
	RemoteActivityState         string `json:"remoteActivityState,omitempty"`
	InactiveReason              int    `json:"inactiveReason,omitempty"`

	Links []*Link `json:"links"`
}

// ReplicationConsistencyGroupCreatePayload works around a problem where the RpoInSeconds must be enclosed
// in quotes when creating an RCG, but is treated as an integer when it is returned.
// This is a bug in the PowerFlex REST implementation.
// This information was obtained from Bubis, Zeev <Zeev.Bubis@dell.com>.
type ReplicationConsistencyGroupCreatePayload struct {
	Name                     string `json:"name"`
	RpoInSeconds             string `json:"rpoInSeconds"` // note this field different
	ProtectionDomainID       string `json:"protectionDomainId"`
	RemoteProtectionDomainID string `json:"remoteProtectionDomainId"`
	DestinationSystemID      string `json:"destinationSystemId,omitempty"`
	PeerMdmID                string `json:"peerMdmId,omitempty"`
}

// ReplicationConsistencyGroupResp response from adding ReplicationConsistencyGroup.
type ReplicationConsistencyGroupResp struct {
	ID string `json:"id"`
}

// RemoveReplicationConsistencyGroupParam defines struct for RemoveReplicationConsistencyGroupParam.
type RemoveReplicationConsistencyGroupParam struct {
	ForceIgnoreConsistency string `json:"forceIgnoreConsistency,omitempty"`
}

// ReplicationPair represents a pair of volumes in a replication relationship.
type ReplicationPair struct {
	ID                                 string `json:"id"`
	Name                               string `json:"name"`
	RemoteID                           string `json:"remoteId"`
	UserRequestedPauseTransmitInitCopy bool   `json:"userRequestedPauseTransmitInitCopy"`
	RemoteCapacityInMB                 int    `json:"remoteCapacityInMB"`
	LocalVolumeID                      string `json:"localVolumeId"`
	RemoteVolumeID                     string `json:"remoteVolumeId"`
	RemoteVolumeName                   string `json:"remoteVolumeName"`
	ReplicationConsistencyGroupID      string `json:"replicationConsistencyGroupId"`
	CopyType                           string `json:"copyType"`
	LifetimeState                      string `json:"lifetimeState"`
	PeerSystemName                     string `json:"peerSystemName"`
	InitialCopyState                   string `json:"initialCopyState"`
	InitialCopyPriority                int    `json:"initialCopyPriority"`
}

// RemoveReplicationPair defines struct for RemoveReplicationPair
type RemoveReplicationPair struct {
	Force string `json:"force,omitempty"`
}

// CreateReplicationConsistencyGroupSnapshot defines struct for CreateReplicationConsistencyGroupSnapshot.
type CreateReplicationConsistencyGroupSnapshot struct {
	Force bool `json:"force,omitempty"`
}

// CreateReplicationConsistencyGroupSnapshotResp defines struct for CreateReplicationConsistencyGroupSnapshotResp.
type CreateReplicationConsistencyGroupSnapshotResp struct {
	SnapshotGroupID string `json:"snapshotGroupId"`
}

// QueryReplicationPair used for querying replication pair.
type QueryReplicationPair struct {
	Name                          string `json:"name"`
	SourceVolumeID                string `json:"sourceVolumeId"`
	DestinationVolumeID           string `json:"destinationVolumeId"`
	ReplicationConsistencyGroupID string `json:"replicationConsistencyGroupId"`
	CopyType                      string `json:"copyType"`
}

// QueryReplicationPairStatistics used for querying the statistics of a replication pair.
type QueryReplicationPairStatistics struct {
	InitialCopyProgress float64 `json:"initialCopyProgress"`
}

// NASServerOperationalStatusEnum NAS lifecycle state.
type NASServerOperationalStatusEnum string

// operational status of NAS
const (
	Stopped  NASServerOperationalStatusEnum = "Stopped"
	Starting NASServerOperationalStatusEnum = "Starting"
	Started  NASServerOperationalStatusEnum = "Started"
	Stopping NASServerOperationalStatusEnum = "Stopping"
	Failover NASServerOperationalStatusEnum = "Failover"
	Degraded NASServerOperationalStatusEnum = "Degraded"
	Unknown  NASServerOperationalStatusEnum = "Unknown"
)

// NFSServerInstance in NAS server
type NFSServerInstance struct {
	// Unique identifier for NFS server
	ID string `json:"id"`
	// HostName will be used by NFS clients to connect to this NFS server.
	HostName string `json:"host_name,omitempty"`
	// IsNFSv4Enabled is set to true if nfsv4 is enabled on NAS server
	IsNFSv4Enabled bool `json:"is_nfsv4_enabled,omitempty"`
	// IsNFSv4Enabled is set to true if nfsv4 is enabled on NAS server
	IsNFSv3Enabled bool `json:"is_nfsv3_enabled,omitempty"`
}

// FileInterface defines struct for FileInterface.
type FileInterface struct {
	// Unique id of the file interface
	ID string `json:"id"`
	// Ip address of file interface
	IPAddress string `json:"ip_address"`
}

// NAS defines struct for NAS.
type NAS struct {
	ID                              string                         `json:"id,omitempty"`
	Description                     string                         `json:"description,omitempty"`
	Name                            string                         `json:"name,omitempty"`
	ProtectionDomainID              string                         `json:"protection_domain_id,omitempty"`
	StoragePoolID                   string                         `json:"storage_pool_id,omitempty"`
	PrimaryNodeID                   string                         `json:"primary_node_id,omitempty"`
	BackUpNodeID                    string                         `json:"backup_node_id,omitempty"`
	OperationalStatus               NASServerOperationalStatusEnum `json:"operational_status,omitempty"`
	CurrentPreferredIPv4InterfaceID string                         `json:"current_preferred_IPv4_interface_id"`
	NfsServers                      []NFSServerInstance            `json:"nfs_servers"`
	CurrentNodeID                   string                         `json:"current_node_id,omitempty"`
	DefaultUnixUser                 string                         `json:"default_unix_user,omitempty"`
	DefaultWindowsUser              string                         `json:"default_windows_user,omitempty"`
	CurrentUnixDirectoryService     string                         `json:"current_unix_directory_service,omitempty"`
	IsUsernameTranslationEnabled    bool                           `json:"is_username_translation_enabled,omitempty"`
	IsAutoUserMappingEnabled        bool                           `json:"is_auto_user_mapping_enabled,omitempty"`
	ProductionIPv4InterfaceID       string                         `json:"production_IPv4_interface_id,omitempty"`
	ProductionIPv6InterfaceID       string                         `json:"production_IPv6_interface_id,omitempty"`
	BackupIPv4InterfaceID           string                         `json:"backup_IPv4_interface_id,omitempty"`
	BackupIPv6InterfaceID           string                         `json:"backup_IPv6_interface_id,omitempty"`
	CurrentPreferredIPv6InterfaceID string                         `json:"current_preferred_IPv6_interface_id,omitempty"`
	OperationalStatusl10n           string                         `json:"operational_status_l10n,omitempty"`
	CurrentUnixDirectoryServicel10n string                         `json:"current_unix_directory_service_l10n,omitempty"`
}

// CreateNASResponse defines the struct for CreateNASResponse
type CreateNASResponse struct {
	ID string `json:"id"`
}

// PingNASParam defines the struct (payload) for Ping NAS
type PingNASParam struct {
	DestinationAddress string `json:"destination_address"`
	IsIPV6             bool   `json:"is_ipv6"`
}

// CreateNASParam defines the struct for CreateNASParam
type CreateNASParam struct {
	Name                         string `json:"name"`
	ProtectionDomainID           string `json:"protection_domain_id"`
	Description                  string `json:"description,omitempty"`
	CurrentUnixDirectoryService  string `json:"current_unix_directory_service,omitempty"`
	DefaultUnixUser              string `json:"default_unix_user,omitempty"`
	DefaultWindowsUser           string `json:"default_windows_user,omitempty"`
	IsUsernameTranslationEnabled bool   `json:"is_username_translation_enabled,omitempty"`
	IsAutoUserMappingEnabled     bool   `json:"is_auto_user_mapping_enabled,omitempty"`
}

// FileSystem defines struct for PowerFlex FileSystem
type FileSystem struct {
	ID                         string `json:"id"`
	Name                       string `json:"name"`
	Description                string `json:"description"`
	StoragePoolID              string `json:"storage_pool_id"`
	NasServerID                string `json:"nas_server_id"`
	ParentID                   string `json:"parent_id"`
	StorageWwn                 string `json:"storage_wwn"`
	ExportFsID                 string `json:"export_fsid"`
	Type                       string `json:"type"`
	SizeTotal                  int    `json:"size_total"`
	SizeUsed                   int    `json:"size_used"`
	IsReadOnly                 bool   `json:"is_read_only"`
	ProtectionPolicyID         string `json:"protection_policy_id"`
	AccessPolicy               string `json:"access_policy"`
	LockingPolicy              string `json:"locking_policy"`
	FolderRenamePolicy         string `json:"folder_rename_policy"`
	IsSmbSyncWritesEnabled     bool   `json:"is_smb_sync_writes_enabled"`
	IsSmbOpLocksEnabled        bool   `json:"is_smb_op_locks_enabled"`
	IsSmbNoNotifyEnabled       bool   `json:"is_smb_no_notify_enabled"`
	IsSmbNotifyOnAccessEnabled bool   `json:"is_smb_notify_on_access_enabled"`
	IsSmbNotifyOnWriteEnabled  bool   `json:"is_smb_notify_on_write_enabled"`
	SmbNotifyOnChangeDirDepth  int    `json:"smb_notify_on_change_dir_depth"`
	IsAsyncMTimeEnabled        bool   `json:"is_async_MTime_enabled"`
	IsFlrEnabled               bool   `json:"is_flr_enabled"`
	IsQuotaEnabled             bool   `json:"is_quota_enabled"`
	GracePeriod                int    `json:"grace_period"`
	DefaultHardLimit           int    `json:"default_hard_limit"`
	DefaultSoftLimit           int    `json:"default_soft_limit"`
	CreationTimestamp          string `json:"creation_timestamp"`
	ExpirationTimestamp        string `json:"expiration_timestamp"`
	LastRefreshTimestamp       string `json:"last_refresh_timestamp"`
	LastWritableTimestamp      string `json:"last_writable_timestamp"`
	IsModified                 bool   `json:"is_modified"`
	AccessType                 string `json:"access_type"`
	CreatorType                string `json:"creator_type"`
}

// CreateFileSystemSnapshotParam defines struct for  FileSystem Snapshot Request
type CreateFileSystemSnapshotParam struct {
	Name                       string `json:"name,omitempty"`
	Description                string `json:"description,omitempty"`
	ExpirationTimestamp        string `json:"expiration_timestamp,omitempty"`
	AccessType                 string `json:"access_type,omitempty"`
	AccessPolicy               string `json:"access_policy,omitempty"`
	LockingPolicy              string `json:"locking_policy,omitempty"`
	FolderRenamePolicy         string `json:"folder_rename_policy,omitempty"`
	IsSmbSyncWritesEnabled     bool   `json:"is_smb_sync_writes_enabled,omitempty"`
	IsSmbNoNotifyEnabled       bool   `json:"is_smb_no_notify_enabled,omitempty"`
	IsSmbOpLocksEnabled        bool   `json:"is_smb_op_locks_enabled,omitempty"`
	IsSmbNotifyOnAccessEnabled bool   `json:"is_smb_notify_on_access_enabled,omitempty"`
	IsSmbNotifyOnWriteEnabled  bool   `json:"is_smb_notify_on_write_enabled,omitempty"`
	SmbNotifyOnChangeDirDepth  int    `json:"smb_notify_on_change_dir_depth,omitempty"`
	IsAsyncMTimeEnabled        bool   `json:"is_async_MTime_enabled,omitempty"`
}

// CreateFileSystemSnapshotResponse defines struct for FileSystem Snapshot Response
type CreateFileSystemSnapshotResponse struct {
	ID string `json:"id,omitempty"`
}

// FsCreate defines struct for creating a PowerFlex FileSystem
type FsCreate struct {
	Name                       string `json:"name"`
	Description                string `json:"description,omitempty"`
	SizeTotal                  int    `json:"size_total"`
	StoragePoolID              string `json:"storage_pool_id"`
	NasServerID                string `json:"nas_server_id"`
	IsReadOnly                 bool   `json:"is_read_only,omitempty"`
	AccessPolicy               string `json:"access_policy,omitempty"`
	LockingPolicy              string `json:"locking_policy,omitempty"`
	FolderRenamePolicy         string `json:"folder_rename_policy,omitempty"`
	IsSmbSyncWritesEnabled     bool   `json:"is_smb_sync_writes_enabled,omitempty"`
	IsSmbNoNotifyEnabled       bool   `json:"is_smb_no_notify_enabled,omitempty"`
	IsSmbOpLocksEnabled        bool   `json:"is_smb_op_locks_enabled,omitempty"`
	IsSmbNotifyOnAccessEnabled bool   `json:"is_smb_notify_on_access_enabled,omitempty"`
	IsSmbNotifyOnWriteEnabled  bool   `json:"is_smb_notify_on_write_enabled,omitempty"`
	SmbNotifyOnChangeDirDepth  int    `json:"smb_notify_on_change_dir_depth,omitempty"`
	IsAsyncMTimeEnabled        bool   `json:"is_async_MTime_enabled,omitempty"`
}

// FSModify defines struct for modify FS
type FSModify struct {
	Size             int    `json:"size_total,omitempty"`
	Description      string `json:"description,omitempty"`
	IsQuotaEnabled   bool   `json:"is_quota_enabled,omitempty"`
	GracePeriod      int    `json:"grace_period,omitempty"`
	DefaultHardLimit int    `json:"default_hard_limit,omitempty"`
	DefaultSoftLimit int    `json:"default_soft_limit,omitempty"`
}

// FileSystemResp defines struct for FileSystemResp
type FileSystemResp struct {
	ID string `json:"id"`
}

// RestoreFsSnapParam defines struct for Restoring filesytem from snapshot
type RestoreFsSnapParam struct {
	SnapshotID string `json:"snapshot_id"`
	CopyName   string `json:"copy_name,omitempty"`
}

// RestoreFsSnapResponse defines struct for Restore Filesystem snapshot response
type RestoreFsSnapResponse struct {
	ID string `json:"id"`
}

// NFSExportDefaultAccessEnum defines default access
type NFSExportDefaultAccessEnum string

// Default access const
const (
	NoAccess     NFSExportDefaultAccessEnum = "No_Access"
	ReadOnly     NFSExportDefaultAccessEnum = "Read_Only"
	ReadWrite    NFSExportDefaultAccessEnum = "Read_Write"
	Root         NFSExportDefaultAccessEnum = "Root"
	ReadOnlyRoot NFSExportDefaultAccessEnum = "Read_Only_Root "
)

// NFSExport defines the struct for NFSExport
type NFSExport struct {
	ID                 string                     `json:"id,omitempty"`
	FileSystemID       string                     `json:"file_system_id,omitempty"`
	Name               string                     `json:"name,omitempty"`
	Description        string                     `json:"description,omitempty"`
	DefaultAccess      NFSExportDefaultAccessEnum `json:"default_access,omitempty"`
	Path               string                     `json:"path,omitempty"`
	ReadWriteHosts     []string                   `json:"read_write_hosts,omitempty"`
	ReadOnlyHosts      []string                   `json:"read_only_hosts,omitempty"`
	ReadWriteRootHosts []string                   `json:"read_write_root_hosts,omitempty"`
	ReadOnlyRootHosts  []string                   `json:"read_only_root_hosts,omitempty"`
}

// NFSExportCreateResponse defines struct for response
type NFSExportCreateResponse struct {
	ID string `json:"id"`
}

// TreeQuotaCreateResponse defines struct for response
type TreeQuotaCreateResponse struct {
	ID string `json:"id"`
}

// NFSExportCreate defines struct for Create NFS Export
type NFSExportCreate struct {
	Name               string   `json:"name"`
	FileSystemID       string   `json:"file_system_id"`
	Path               string   `json:"path"`
	NoAccessHosts      []string `json:"no_access_hosts,omitempty"`
	ReadOnlyHosts      []string `json:"read_only_hosts,omitempty"`
	ReadWriteHosts     []string `json:"read_write_hosts,omitempty"`
	ReadOnlyRootHosts  []string `json:"read_only_root_hosts,omitempty"`
	ReadWriteRootHosts []string `json:"read_write_root_hosts,omitempty"`
	AnonymousUID       int      `json:"anonymous_UID,omitempty"`
	AnonymousGID       int      `json:"anonymous_GID,omitempty"`
	IsNoSUID           bool     `json:"is_no_SUID,omitempty"`
}

// TreeQuotaCreate defines a struct for Create Tree Quota
type TreeQuotaCreate struct {
	FileSystemID        string `json:"file_system_id"`
	Path                string `json:"path"`
	Description         string `json:"description,omitempty"`
	HardLimit           int    `json:"hard_limit,omitempty"`
	SoftLimit           int    `json:"soft_limit,omitempty"`
	IsUserQuotaEnforced bool   `json:"is_user_quotas_enforced,omitempty"`
	GracePeriod         int    `json:"grace_period,omitempty"`
}

// TreeQuota defines a struct for tree quota
type TreeQuota struct {
	ID                   string `json:"id,omitempty"`
	FileSysytemID        string `json:"file_system_id"`
	Path                 string `json:"path"`
	Description          string `json:"description,omitempty"`
	HardLimit            int    `json:"hard_limit,omitempty"`
	SoftLimit            int    `json:"soft_limit,omitempty"`
	IsUserQuotaEnforced  bool   `json:"is_user_quotas_enforced,omitempty"`
	GracePeriod          int    `json:"grace_period,omitempty"`
	State                string `json:"state,omitempty"`
	RemainingGracePeriod int    `json:"remaining_grace_period,omitempty"`
	SizeUsed             int    `json:"size_used,omitempty"`
}

// TreeQuotaModify defines struct for Modify Tree Quota
type TreeQuotaModify struct {
	Description          string `json:"description,omitempty"`
	HardLimit            int    `json:"hard_limit,omitempty"`
	SoftLimit            int    `json:"soft_limit,omitempty"`
	IsUserQuotasEnforced bool   `json:"is_user_quotas_enforced,omitempty"`
	GracePeriod          int    `json:"grace_period,omitempty"`
}

// NFSExportModify defines struct for Modify NFS Export
type NFSExportModify struct {
	Description              string   `json:"description,omitempty"`
	DefaultAccess            string   `json:"default_access,omitempty"`
	NoAccessHosts            []string `json:"no_access_hosts,omitempty"`
	AddNoAccessHosts         []string `json:"add_no_access_hosts,omitempty"`
	RemoveNoAccessHosts      []string `json:"remove_no_access_hosts,omitempty"`
	ReadOnlyHosts            []string `json:"read_only_hosts,omitempty"`
	AddReadOnlyHosts         []string `json:"add_read_only_hosts,omitempty"`
	RemoveReadOnlyHosts      []string `json:"remove_read_only_hosts,omitempty"`
	ReadOnlyRootHosts        []string `json:"read_only_root_hosts,omitempty"`
	AddReadOnlyRootHosts     []string `json:"add_read_only_root_hosts,omitempty"`
	RemoveReadOnlyRootHosts  []string `json:"remove_read_only_root_hosts,omitempty"`
	ReadWriteHosts           []string `json:"read_write_hosts,omitempty"`
	AddReadWriteHosts        []string `json:"add_read_write_hosts,omitempty"`
	RemoveReadWriteHosts     []string `json:"remove_read_write_hosts,omitempty"`
	ReadWriteRootHosts       []string `json:"read_write_root_hosts,omitempty"`
	AddReadWriteRootHosts    []string `json:"add_read_write_root_hosts,omitempty"`
	RemoveReadWriteRootHosts []string `json:"remove_read_write_root_hosts,omitempty"`
}

// UploadPackageParam defines struct for Upload Package
type UploadPackageParam struct {
	FilePath string `json:"file_path"`
}

// PackageDetails defines struct for Package Details Response
type PackageDetails struct {
	Filename        string `json:"filename"`
	OperatingSystem string `json:"operatingSystem"`
	LinuxFlavour    string `json:"linuxFlavour"`
	Version         string `json:"version"`
	SioPatchNumber  int    `json:"sioPatchNumber"`
	Label           string `json:"label"`
	Type            string `json:"type"`
	Size            int    `json:"size"`
	Latest          bool   `json:"latest"`
}

// GatewayResponse defines struct for Gateway API Response
type GatewayResponse struct {
	Message        string             `json:"message,omitempty"`
	Data           string             `json:"data,omitempty"`
	ClusterDetails MDMTopologyDetails `json:"clusterDetails,omitempty"`
	StatusCode     int                `json:"httpStatusCode,omitempty"`
	ErrorCode      int                `json:"errorCode,omitempty"`
}

// MDMTopologyParam defines struct for Validate MDM Topology
type MDMTopologyParam struct {
	MdmIps                []string                     `json:"mdmIps"`
	MdmUser               string                       `json:"mdmUser"`
	MdmPassword           string                       `json:"mdmPassword"`
	SecurityConfiguration SecurityConfigurationDetails `json:"securityConfiguration"`
}

// SecurityConfigurationDetails defines struct for Security Details MDM Validation
type SecurityConfigurationDetails struct {
	AllowNonSecureCommunicationWithMdm bool `json:"allowNonSecureCommunicationWithMdm"`
	AllowNonSecureCommunicationWithLia bool `json:"allowNonSecureCommunicationWithLia"`
	DisableNonMgmtComponentsAuth       bool `json:"disableNonMgmtComponentsAuth"`
}

// MDMTopologyDetails defines struct for Validated MDM Topology Details
type MDMTopologyDetails struct {
	MdmIPs            []string            `json:"mdmIPs,omitempty"`
	SdsAndMdmIps      []string            `json:"sdsAndMdmIps,omitempty"`
	SdcIps            []string            `json:"sdcIps,omitempty"`
	VirtualIPs        []string            `json:"virtualIps,omitempty"`
	SystemVersionName string              `json:"systemVersionName,omitempty"`
	SdsList           []SdsList           `json:"sdsList,omitempty"`
	SdcList           []SdcList           `json:"sdcList,omitempty"`
	SdtList           []SdtList           `json:"sdtList,omitempty"`
	ProtectionDomains []ProtectionDomains `json:"protectionDomains,omitempty"`
	SdrList           []SdrList           `json:"sdrList,omitempty"`
	VasaProviderList  []any               `json:"vasaProviderList,omitempty"`
	MasterMdm         MasterMdm           `json:"masterMdm,omitempty"`
	SlaveMdmSet       []SlaveMdmSet       `json:"slaveMdmSet,omitempty"`
	TbSet             []TbSet             `json:"tbSet,omitempty"`
	StandbyMdmSet     []StandbyMdmSet     `json:"standbyMdmSet,omitempty"`
	StandbyTbSet      []StandbyTbSet      `json:"standbyTbSet,omitempty"`
}

// MasterMdm defines struct for Mster MDM Details
type MasterMdm struct {
	Node            Node     `json:"node,omitempty"`
	MdmIPs          []string `json:"mdmIPs,omitempty"`
	Name            string   `json:"name,omitempty"`
	ID              string   `json:"id,omitempty"`
	IPForActor      any      `json:"ipForActor,omitempty"`
	ManagementIPs   []string `json:"managementIPs,omitempty"`
	VirtIPIntfsList []string `json:"virtIpIntfsList,omitempty"`
}

// SlaveMdmSet defines struct for Slave MDMs Details
type SlaveMdmSet struct {
	Node            Node     `json:"node,omitempty"`
	MdmIPs          []string `json:"mdmIPs,omitempty"`
	Name            string   `json:"name,omitempty"`
	ID              string   `json:"id,omitempty"`
	IPForActor      any      `json:"ipForActor,omitempty"`
	ManagementIPs   []string `json:"managementIPs,omitempty"`
	VirtIPIntfsList []string `json:"virtIpIntfsList,omitempty"`
}

// StandbyMdmSet defines struct for Standby MDMs Details
type StandbyMdmSet struct {
	Node            Node     `json:"node,omitempty"`
	MdmIPs          []string `json:"mdmIPs,omitempty"`
	Name            string   `json:"name,omitempty"`
	ID              string   `json:"id,omitempty"`
	IPForActor      any      `json:"ipForActor,omitempty"`
	ManagementIPs   []string `json:"managementIPs,omitempty"`
	VirtIPIntfsList []string `json:"virtIpIntfsList,omitempty"`
}

// TbSet defines struct for TB MDMs Details
type TbSet struct {
	Node   Node     `json:"node,omitempty"`
	MdmIPs []string `json:"mdmIPs,omitempty"`
	Name   string   `json:"name,omitempty"`
	ID     string   `json:"id,omitempty"`
	TbIPs  []string `json:"tbIPs,omitempty"`
}

// StandbyTbSet defines struct for Standby TB MDMs Details
type StandbyTbSet struct {
	Node   Node     `json:"node,omitempty"`
	MdmIPs []string `json:"mdmIPs,omitempty"`
	Name   string   `json:"name,omitempty"`
	ID     string   `json:"id,omitempty"`
	TbIPs  []string `json:"tbIPs,omitempty"`
}

// Node defines struct for Node Information
type Node struct {
	Ostype   string   `json:"ostype,omitempty"`
	NodeName string   `json:"nodeName,omitempty"`
	NodeIPs  []string `json:"nodeIPs,omitempty"`
}

// Devices defines struct for Device Details
type Devices struct {
	DevicePath      string `json:"devicePath,omitempty"`
	StoragePool     string `json:"storagePool,omitempty"`
	DeviceName      string `json:"deviceName,omitempty"`
	MaxCapacityInKb int    `json:"maxCapacityInKb,omitempty"`
}

// SdsList defines struct for SDS Details
type SdsList struct {
	Node                 Node      `json:"node,omitempty"`
	SdsName              string    `json:"sdsName,omitempty"`
	ProtectionDomain     string    `json:"protectionDomain,omitempty"`
	ProtectionDomainID   string    `json:"protectionDomainId,omitempty"`
	FaultSet             string    `json:"faultSet,omitempty"`
	FaultSetID           string    `json:"faultSetId,omitempty"`
	AllIPs               []string  `json:"allIPs,omitempty"`
	SdsOnlyIPs           []string  `json:"sdsOnlyIPs,omitempty"`
	SdcOnlyIPs           []string  `json:"sdcOnlyIPs,omitempty"`
	Devices              []Devices `json:"devices,omitempty"`
	RfCached             bool      `json:"rfCached,omitempty"`
	RfCachedPools        []any     `json:"rfCachedPools,omitempty"`
	RfCachedDevices      []any     `json:"rfCachedDevices,omitempty"`
	RfCacheType          any       `json:"rfCacheType,omitempty"`
	FlashAccDevices      []any     `json:"flashAccDevices,omitempty"`
	NvdimmAccDevices     []any     `json:"nvdimmAccDevices,omitempty"`
	UseRmCache           bool      `json:"useRmCache,omitempty"`
	Optimized            bool      `json:"optimized,omitempty"`
	OptimizedNumOfIOBufs int       `json:"optimizedNumOfIOBufs,omitempty"`
	Port                 int       `json:"port,omitempty"`
	ID                   string    `json:"id,omitempty"`
}

// SdtList defines struct for SDT Details
type SdtList struct {
	Node               Node     `json:"node,omitempty"`
	SdtName            string   `json:"sdtName,omitempty"`
	ProtectionDomain   string   `json:"protectionDomain,omitempty"`
	ProtectionDomainID string   `json:"protectionDomainId,omitempty"`
	AllIPs             []string `json:"allIPs,omitempty"`
	StorageOnlyIPs     []string `json:"storageOnlyIPs,omitempty"`
	HostOnlyIPs        []string `json:"hostOnlyIPs,omitempty"`
	StoragePort        int      `json:"storagePort,omitempty"`
	NvmePort           int      `json:"nvmePort,omitempty"`
	DiscoveryPort      int      `json:"discoveryPort,omitempty"`
	Optimized          bool     `json:"optimized,omitempty"`
	ID                 string   `json:"id,omitempty"`
}

// SdcList defines struct for SDC Details
type SdcList struct {
	Node      Node   `json:"node,omitempty"`
	GUID      string `json:"guid,omitempty"`
	SdcName   string `json:"sdcName,omitempty"`
	Optimized bool   `json:"optimized,omitempty"`
	ID        string `json:"id,omitempty"`
}

// SdrList defines struct for SDR Details
type SdrList struct {
	Node                      Node     `json:"node,omitempty"`
	ProtectionDomain          string   `json:"protectionDomain,omitempty"`
	ProtectionDomainID        string   `json:"protectionDomainId,omitempty"`
	ApplicationOnlyIPs        []string `json:"applicationOnlyIPs,omitempty"`
	StorageOnlyIPs            []string `json:"storageOnlyIPs,omitempty"`
	ExternalOnlyIPs           []string `json:"externalOnlyIPs,omitempty"`
	ApplicationAndStorageIPs  []string `json:"applicationAndStorageIPs,omitempty"`
	ApplicationAndExternalIPs []string `json:"applicationAndExternalIPs,omitempty"`
	StorageAndExternalIPs     []string `json:"storageAndExternalIPs,omitempty"`
	AllIPs                    []string `json:"allIPs,omitempty"`
	SupersetIPs               any      `json:"supersetIPs,omitempty"`
	SdrName                   string   `json:"sdrName,omitempty"`
	SdrPort                   int      `json:"sdrPort,omitempty"`
	Optimized                 bool     `json:"optimized,omitempty"`
	ID                        string   `json:"id,omitempty"`
}

// ProtectionDomains defines struct for ProtectionDomains Details
type ProtectionDomains struct {
	Name              string         `json:"name,omitempty"`
	StoragePools      []StoragePools `json:"storagePools,omitempty"`
	AccelerationPools []any          `json:"accelerationPools,omitempty"`
}

// StoragePools defines struct for StoragePools Details
type StoragePools struct {
	Name                     string `json:"name,omitempty"`
	MediaType                string `json:"mediaType,omitempty"`
	ExternalAccelerationType string `json:"externalAccelerationType,omitempty"`
	DataLayout               string `json:"dataLayout,omitempty"`
	CompressionMethod        string `json:"compressionMethod,omitempty"`
	SpefAccPoolName          any    `json:"spefAccPoolName,omitempty"`
	ShouldApplyZeroPadding   bool   `json:"shouldApplyZeroPadding,omitempty"`
	WriteAtomicitySize       any    `json:"writeAtomicitySize,omitempty"`
	OverProvisioningFactor   any    `json:"overProvisioningFactor,omitempty"`
	MaxCompressionRatio      any    `json:"maxCompressionRatio,omitempty"`
	PerfProfile              any    `json:"perfProfile,omitempty"`
	RplJournalCapacity       int    `json:"rplJournalCapacity,omitempty"`
}

// InstallerPhaseDetail defines struct for Current and Next Phase Details
type InstallerPhaseDetail struct {
	Phase                    PhaseDetails `json:"phase,omitempty"`
	NextPhase                PhaseDetails `json:"nextPhase,omitempty"`
	Operation                string       `json:"operation,omitempty"`
	UpgradePersistenceRecord any          `json:"upgradePersistenceRecord,omitempty"`
	RollbackEnabled          bool         `json:"rollbackEnabled,omitempty"`
	Message                  string       `json:"message,omitempty"`
	StatusCode               int          `json:"httpStatusCode,omitempty"`
	ErrorCode                int          `json:"errorCode,omitempty"`
}

// PhaseDetails defines struct for specific phase details
type PhaseDetails struct {
	Name            string `json:"name,omitempty"`
	PreludeMessage  any    `json:"preludeMessage,omitempty"`
	PrologueMessage any    `json:"prologueMessage,omitempty"`
	AutoStart       bool   `json:"autoStart,omitempty"`
}

// MDMQueueCommandDetails defines struct for In Queue command details
type MDMQueueCommandDetails struct {
	CommandName            string    `json:"commandName,omitempty"`
	MdmIPs                 []string  `json:"mdmIPs,omitempty"`
	CommandState           string    `json:"commandState,omitempty"`
	StartTime              time.Time `json:"startTime,omitempty"`
	CompletionTime         time.Time `json:"completionTime,omitempty"`
	Message                string    `json:"message,omitempty"`
	NodeIPs                []string  `json:"nodeIPs,omitempty"`
	CommandParameters      []string  `json:"commandParameters,omitempty"`
	TargetEntityIdentifier string    `json:"targetEntityIdentifier,omitempty"`
	AllowedPhase           string    `json:"allowedPhase,omitempty"`
}

// SystemLimits defines struct for system limits
type SystemLimits struct {
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	MaxVal      string `json:"maxVal,omitempty"`
}

// QuerySystemLimitsResponse defines struct for system limit response
type QuerySystemLimitsResponse struct {
	SystemLimitEntryList []SystemLimits `json:"systemLimitEntryList"`
}

// QuerySystemLimitsParam is the parameters required to query system limits
type QuerySystemLimitsParam struct{}

// FaultSetParam is the parameters required to create a fault set
type FaultSetParam struct {
	ProtectionDomainID string `json:"protectionDomainId"`
	Name               string `json:"name"`
}

// FaultSetResp defines struct for the response when fault set is created successfully
type FaultSetResp struct {
	ID string `json:"id"`
}

// FaultSet defines struct for reading the fault set
type FaultSet struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	ProtectionDomainID string  `json:"protectionDomainId"`
	Links              []*Link `json:"links"`
}

// FaultSetRename defines struct for renaming the fault set
type FaultSetRename struct {
	NewName string `json:"newName"`
}

// UploadComplianceParam defines struct for uploading the compliance file
type UploadComplianceParam struct {
	SourceLocation string `json:"sourceLocation"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
}

// UploadComplianceTopologyDetails defines struct which will hold the details of the compliance file upload
type UploadComplianceTopologyDetails struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SourceLocation string `json:"sourceLocation"`
	DiskLocation   string `json:"diskLocation"`
	Filename       string `json:"filename"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	DefaultCatalog bool   `json:"defaultCatalog"`
	State          string `json:"state"`
}

// FirmwareRepositoryDetails holds the entire details of firmware repository
type FirmwareRepositoryDetails struct {
	ID                  string      `json:"id"`
	Name                string      `json:"name"`
	SourceLocation      string      `json:"sourceLocation"`
	SourceType          string      `json:"sourceType"`
	DiskLocation        string      `json:"diskLocation"`
	Filename            string      `json:"filename"`
	Username            string      `json:"username"`
	Password            string      `json:"password"`
	DownloadStatus      string      `json:"downloadStatus"`
	CreatedDate         string      `json:"createdDate"`
	CreatedBy           string      `json:"createdBy"`
	UpdatedDate         string      `json:"updatedDate"`
	UpdatedBy           string      `json:"updatedBy"`
	DefaultCatalog      bool        `json:"defaultCatalog"`
	Embedded            bool        `json:"embedded"`
	State               string      `json:"state"`
	SoftwareComponents  []Component `json:"softwareComponents"`
	SoftwareBundles     []Bundle    `json:"softwareBundles"`
	BundleCount         int         `json:"bundleCount"`
	ComponentCount      int         `json:"componentCount"`
	UserBundleCount     int         `json:"userBundleCount"`
	Minimal             bool        `json:"minimal"`
	DownloadProgress    int         `json:"downloadProgress"`
	ExtractProgress     int         `json:"extractProgress"`
	FileSizeInGigabytes float64     `json:"fileSizeInGigabytes"`
	Signature           string      `json:"signature"`
	Custom              bool        `json:"custom"`
	NeedsAttention      bool        `json:"needsAttention"`
	JobID               string      `json:"jobId"`
	Rcmapproved         bool        `json:"rcmapproved"`
}

// Component holds the details of firmware components
type Component struct {
	ID                  string   `json:"id"`
	PackageID           string   `json:"packageId"`
	DellVersion         string   `json:"dellVersion"`
	VendorVersion       string   `json:"vendorVersion"`
	ComponentID         string   `json:"componentId"`
	DeviceID            string   `json:"deviceId"`
	SubDeviceID         string   `json:"subDeviceId"`
	VendorID            string   `json:"vendorId"`
	SubVendorID         string   `json:"subVendorId"`
	CreatedDate         string   `json:"createdDate"`
	CreatedBy           string   `json:"createdBy"`
	UpdatedDate         string   `json:"updatedDate"`
	UpdatedBy           string   `json:"updatedBy"`
	Path                string   `json:"path"`
	HashMd5             string   `json:"hashMd5"`
	Name                string   `json:"name"`
	Category            string   `json:"category"`
	ComponentType       string   `json:"componentType"`
	OperatingSystem     string   `json:"operatingSystem"`
	SystemIDs           []string `json:"systemIDs"`
	Custom              bool     `json:"custom"`
	NeedsAttention      bool     `json:"needsAttention"`
	Ignore              bool     `json:"ignore"`
	OriginalComponentID string   `json:"originalComponentId"`
	FirmwareRepoName    string   `json:"firmwareRepoName"`
}

// Bundle holds the details of firmware bundles
type Bundle struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	Version            string      `json:"version"`
	BundleDate         string      `json:"bundleDate"`
	CreatedDate        string      `json:"createdDate"`
	CreatedBy          string      `json:"createdBy"`
	UpdatedDate        string      `json:"updatedDate"`
	UpdatedBy          string      `json:"updatedBy"`
	Description        string      `json:"description"`
	UserBundle         bool        `json:"userBundle"`
	UserBundlePath     string      `json:"userBundlePath"`
	DeviceType         string      `json:"deviceType"`
	DeviceModel        string      `json:"deviceModel"`
	FwRepositoryID     string      `json:"fwRepositoryId"`
	BundleType         string      `json:"bundleType"`
	Custom             bool        `json:"custom"`
	NeedsAttention     bool        `json:"needsAttention"`
	SoftwareComponents []Component `json:"softwareComponents"`
}

// IntString is a custom type that represents an integer value, while string is used by PowerFlex API.
type IntString int

// MarshalJSON is a custom JSON marshaler for the IntString type.
// It converts the IntString value to a string representation and marshals it into a JSON byte slice.
func (i IntString) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%d", i))
}

// NvmeHost defines struct for PowerFlex NVMe host
type NvmeHost struct {
	SystemID       string  `json:"systemId"`
	Name           string  `json:"name"`
	Nqn            string  `json:"nqn"`
	MaxNumPaths    int     `json:"maxNumPaths"`
	MaxNumSysPorts int     `json:"maxNumSysPorts"`
	HostType       string  `json:"hostType"`
	ID             string  `json:"id"`
	Links          []*Link `json:"links"`
}

// NvmeHostResp defines struct for response of creating NvmeHost
type NvmeHostResp struct {
	ID string `json:"id"`
}

// ChangeNvmeHostNameParam defines struct for renaming the NVMe host
type ChangeNvmeHostNameParam ChangeSdcNameParam

// ChangeNvmeMaxNumPathsParam defines struct for new max number paths
type ChangeNvmeMaxNumPathsParam struct {
	MaxNumPaths IntString `json:"newMaxNumPaths"`
}

// ChangeNvmeHostMaxNumSysPortsParam defines struct for new max number system ports
type ChangeNvmeHostMaxNumSysPortsParam struct {
	MaxNumSysPorts IntString `json:"newMaxNumSysPorts"`
}

// NvmeHostParam defines struct for creating an NVMe host
type NvmeHostParam struct {
	Name           string    `json:"name,omitempty"`
	Nqn            string    `json:"nqn"`
	MaxNumPaths    IntString `json:"maxNumPaths,omitempty"`
	MaxNumSysPorts IntString `json:"maxNumSysPorts,omitempty"`
}

// Sdt defines struct for Sdt
type Sdt struct {
	ID                                string          `json:"id"`
	Name                              string          `json:"name,omitempty"`
	ProtectionDomainID                string          `json:"protectionDomainId"`
	IPList                            []*SdtIP        `json:"ipList"`
	StoragePort                       int             `json:"storagePort,omitempty"`
	NvmePort                          int             `json:"nvmePort,omitempty"`
	DiscoveryPort                     int             `json:"discoveryPort,omitempty"`
	SdtState                          string          `json:"sdtState"`
	SystemID                          string          `json:"systemId"`
	MdmConnectionState                string          `json:"mdmConnectionState"`
	MembershipState                   string          `json:"membershipState"`
	FaultSetID                        string          `json:"faultSetId,omitempty"`
	SoftwareVersionInfo               string          `json:"softwareVersionInfo,omitempty"`
	MaintenanceState                  string          `json:"maintenanceState,omitempty"`
	AuthenticationError               string          `json:"authenticationError,omitempty"`
	PersistentDiscoveryControllersNum int             `json:"persistentDiscoveryControllersNum,omitempty"`
	CertificateInfo                   CertificateInfo `json:"certificateInfo,omitempty"`
	Links                             []*Link         `json:"links"`
}

// SdtIP defines struct for SdtIP
type SdtIP struct {
	IP   string `json:"ip"`
	Role string `json:"role"`
}

// SdtParam defines struct for SdtParam
type SdtParam struct {
	Name               string    `json:"name,omitempty"`
	IPList             []*SdtIP  `json:"ips"`
	StoragePort        IntString `json:"storagePort,omitempty"`
	NvmePort           IntString `json:"nvmePort,omitempty"`
	DiscoveryPort      IntString `json:"discoveryPort,omitempty"`
	ProtectionDomainID string    `json:"protectionDomainId"`
}

// SdtResp defines struct for response of creating sdt
type SdtResp struct {
	ID string `json:"id"`
}

// SdtRenameParam defines the struct for renaming an sdt
type SdtRenameParam struct {
	NewName string `json:"newName"`
}

// SdtNvmePortParam defines the struct for modifying NVMe port for the sdt
type SdtNvmePortParam struct {
	NewNvmePort IntString `json:"newNvmePort"`
}

// SdtStoragePortParam defines the struct for modifying StoragePort for the sdt
type SdtStoragePortParam struct {
	NewStoragePort IntString `json:"newStoragePort"`
}

// SdtDiscoveryPortParam defines the struct for modifying DiscoveryPort for the sdt
type SdtDiscoveryPortParam struct {
	NewDiscoveryPort IntString `json:"newDiscoveryPort"`
}

// SdtRemoveIPParam defines the struct for removing IP from the sdt
type SdtRemoveIPParam struct {
	IP string `json:"ip"`
}

// SdtIPRoleParam defines the struct for modifying IP Role for the sdt
type SdtIPRoleParam struct {
	IP   string `json:"ip"`
	Role string `json:"newRole"`
}

// NvmeController defines struct for NVMe controller
type NvmeController struct {
	Name         string `json:"name"`
	IsConnected  bool   `json:"isConnected"`
	SdtID        string `json:"sdtId"`
	HostIP       string `json:"hostIp"`
	HostID       string `json:"hostId"`
	ControllerID int    `json:"controllerId"`
	SysPortID    int    `json:"sysPortId"`
	SysPortIP    string `json:"sysPortIp"`
	Subsystem    string `json:"subsystem"`
	IsAssigned   bool   `json:"isAssigned"`
	ID           string `json:"id"`
}

// LcmStatus defines struct for PFMP status
type LcmStatus struct {
	LcmStatus      string `json:"lcmStatus"`
	ClusterVersion string `json:"clusterVersion"`
	ClusterBuild   string `json:"clusterBuild"`
}

type NodeCredentialWrapper struct {
	XMLName          xml.Name         `xml:"asmCredential"`
	ServerCredential ServerCredential `xml:"serverCredential"`
}

type ServerCredential struct {
	XMLName  xml.Name `xml:"serverCredential"`
	Username string   `xml:"username"`
	Password string   `xml:"password"`
	Label    string   `xml:"label"`
	// Needed for SNMPv2
	SNMPv2CommunityString string `xml:"snmpCommunityString,omitempty"`
	// If SNMPv2 is Set this should be set to SSH
	SNMPv2Protocol string `xml:"protocol,omitempty"`
	// Needed for SNMPv3
	SNMPv3SecurityName string `xml:"snmpv3UserName,omitempty"`
	// Sets the level 1 2 or 3 for SNMPv3
	SNMPv3SecurityLevel string `xml:"securityLevel,omitempty"`
	// Required for SNMPv3 level 2 and 3
	SNMPv3MD5AuthenticationPassword string `xml:"md5AuthenticationPassword,omitempty"`
	// Required for SNMPv3 level 3
	SNMPv3DesPrivatePassword string `xml:"desPrivacyPassword,omitempty"`
	// Private Key Stringified in the .pem format
	SSHPrivateKey string `xml:"sshPrivateKey,omitempty"`
	// Required if Private Key is set
	KeyPairName string `xml:"keyPairName,omitempty"`
}

type SwitchCredentialWrapper struct {
	XMLName       xml.Name      `xml:"asmCredential"`
	IomCredential IomCredential `xml:"iomCredential"`
}

type IomCredential struct {
	XMLName  xml.Name `xml:"iomCredential"`
	Username string   `xml:"username"`
	Password string   `xml:"password"`
	Label    string   `xml:"label"`
	// Needed for SNMPv2
	SNMPv2CommunityString string `xml:"snmpCommunityString"`
	// For SNMPv2 this should be set to SSH
	SNMPv2Protocol string `xml:"protocol"`
	// Private Key Stringified in the .pem format
	SSHPrivateKey string `xml:"sshPrivateKey,omitempty"`
	// Required if Private Key is set
	KeyPairName string `xml:"keyPairName,omitempty"`
}

type VCenterCredentialWrapper struct {
	XMLName           xml.Name          `xml:"asmCredential"`
	VCenterCredential VCenterCredential `xml:"vCenterCredential"`
}

type VCenterCredential struct {
	XMLName  xml.Name `xml:"vCenterCredential"`
	Username string   `xml:"username"`
	Password string   `xml:"password"`
	Label    string   `xml:"label"`
	Domain   string   `xml:"domain"`
}

type ElementManagerCredentialWrapper struct {
	XMLName      xml.Name     `xml:"asmCredential"`
	EMCredential EMCredential `xml:"emCredential"`
}

type EMCredential struct {
	XMLName  xml.Name `xml:"emCredential"`
	Username string   `xml:"username"`
	Password string   `xml:"password"`
	Domain   string   `xml:"domain"`
	Label    string   `xml:"label"`
	// Needed for SNMPv2
	SNMPv2CommunityString string `xml:"snmpCommunityString"`
	// If SNMPv2 is Set this should be set to SSH
	SNMPv2Protocol string `xml:"protocol"`
}

type GatewayCredentialWrapper struct {
	XMLName           xml.Name          `xml:"asmCredential"`
	ScaleIOCredential ScaleIOCredential `xml:"scaleIOCredential"`
}

type ScaleIOCredential struct {
	XMLName       xml.Name `xml:"scaleIOCredential"`
	AdminUsername string   `xml:"username"`
	AdminPassword string   `xml:"password"`
	Label         string   `xml:"label"`
	OSUsername    string   `xml:"osUsername"`
	OSPassword    string   `xml:"osPassword"`
}

type PresentationServerCredentialWrapper struct {
	XMLName      xml.Name     `xml:"asmCredential"`
	PSCredential PSCredential `xml:"PSCredential"`
}

type PSCredential struct {
	XMLName  xml.Name `xml:"PSCredential"`
	Label    string   `xml:"label"`
	Username string   `xml:"username"`
	Password string   `xml:"password"`
}

type OsAdminCredentialWrapper struct {
	XMLName           xml.Name          `xml:"asmCredential"`
	OSAdminCredential OSAdminCredential `xml:"OSCredential"`
}

type OSAdminCredential struct {
	XMLName xml.Name `xml:"OSCredential"`
	// Gets defaulted to root if not set
	Username string `xml:"username"`
	Password string `xml:"password"`
	Label    string `xml:"label"`
	// Private Key Stringified in the .pem format
	SSHPrivateKey string `xml:"sshPrivateKey,omitempty"`
	// Required if Private Key is set
	KeyPairName string `xml:"keyPairName,omitempty"`
}

type OsUserCredentialWrapper struct {
	XMLName          xml.Name         `xml:"asmCredential"`
	OSUserCredential OSUserCredential `xml:"OSUserCredential"`
}

type OSUserCredential struct {
	XMLName  xml.Name `xml:"OSUserCredential"`
	Username string   `xml:"username"`
	Password string   `xml:"password"`
	Domain   string   `xml:"domain"`
	Label    string   `xml:"label"`
	// Private Key Stringified in the .pem format
	SSHPrivateKey string `xml:"sshPrivateKey,omitempty"`
	// Required if Private Key is set
	KeyPairName string `xml:"keyPairName,omitempty"`
}
