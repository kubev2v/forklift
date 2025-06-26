// Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
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

// TemplateDetails defines the details of a template.
type TemplateDetails struct {
	ID                     string               `json:"id,omitempty"`
	TemplateName           string               `json:"templateName,omitempty"`
	TemplateDescription    string               `json:"templateDescription,omitempty"`
	TemplateType           string               `json:"templateType,omitempty"`
	TemplateVersion        string               `json:"templateVersion,omitempty"`
	OriginalTemplateID     string               `json:"originalTemplateId,omitempty"`
	TemplateValid          TemplateValid        `json:"templateValid,omitempty"`
	TemplateLocked         bool                 `json:"templateLocked,omitempty"`
	InConfiguration        bool                 `json:"inConfiguration,omitempty"`
	CreatedDate            string               `json:"createdDate,omitempty"`
	CreatedBy              string               `json:"createdBy,omitempty"`
	UpdatedDate            string               `json:"updatedDate,omitempty"`
	LastDeployedDate       string               `json:"lastDeployedDate,omitempty"`
	UpdatedBy              string               `json:"updatedBy,omitempty"`
	ManageFirmware         bool                 `json:"manageFirmware,omitempty"`
	UseDefaultCatalog      bool                 `json:"useDefaultCatalog,omitempty"`
	FirmwareRepository     FirmwareRepository   `json:"firmwareRepository,omitempty"`
	LicenseRepository      LicenseRepository    `json:"licenseRepository,omitempty"`
	AssignedUsers          []AssignedUsers      `json:"assignedUsers,omitempty"`
	AllUsersAllowed        bool                 `json:"allUsersAllowed,omitempty"`
	Category               string               `json:"category,omitempty"`
	Components             []Components         `json:"components,omitempty"`
	Configuration          ConfigurationDetails `json:"configuration,omitempty"`
	ServerCount            int                  `json:"serverCount,omitempty"`
	StorageCount           int                  `json:"storageCount,omitempty"`
	ClusterCount           int                  `json:"clusterCount,omitempty"`
	ServiceCount           int                  `json:"serviceCount,omitempty"`
	SwitchCount            int                  `json:"switchCount,omitempty"`
	VMCount                int                  `json:"vmCount,omitempty"`
	SdnasCount             int                  `json:"sdnasCount,omitempty"`
	BrownfieldTemplateType string               `json:"brownfieldTemplateType,omitempty"`
	Networks               []Networks           `json:"networks,omitempty"`
	NetworksMap            map[string]Networks  `json:"networksMap,omitempty"`
	Draft                  bool                 `json:"draft,omitempty"`
}

// Messages defines a struct for messages.
type Messages struct {
	ID              string `json:"id,omitempty"`
	MessageCode     string `json:"messageCode,omitempty"`
	MessageBundle   string `json:"messageBundle,omitempty"`
	Severity        string `json:"severity,omitempty"`
	Category        string `json:"category,omitempty"`
	DisplayMessage  string `json:"displayMessage,omitempty"`
	ResponseAction  string `json:"responseAction,omitempty"`
	DetailedMessage string `json:"detailedMessage,omitempty"`
	CorrelationID   string `json:"correlationId,omitempty"`
	AgentID         string `json:"agentId,omitempty"`
	TimeStamp       string `json:"timeStamp,omitempty"`
	SequenceNumber  int    `json:"sequenceNumber,omitempty"`
}

// TemplateValid defines the validity of a template.
type TemplateValid struct {
	Valid    bool       `json:"valid,omitempty"`
	Messages []Messages `json:"messages,omitempty"`
}

// SoftwareComponents defines the details of a software component.
type SoftwareComponents struct {
	ID                  string   `json:"id,omitempty"`
	PackageID           string   `json:"packageId,omitempty"`
	DellVersion         string   `json:"dellVersion,omitempty"`
	VendorVersion       string   `json:"vendorVersion,omitempty"`
	ComponentID         string   `json:"componentId,omitempty"`
	DeviceID            string   `json:"deviceId,omitempty"`
	SubDeviceID         string   `json:"subDeviceId,omitempty"`
	VendorID            string   `json:"vendorId,omitempty"`
	SubVendorID         string   `json:"subVendorId,omitempty"`
	CreatedDate         string   `json:"createdDate,omitempty"`
	CreatedBy           string   `json:"createdBy,omitempty"`
	UpdatedDate         string   `json:"updatedDate,omitempty"`
	UpdatedBy           string   `json:"updatedBy,omitempty"`
	Path                string   `json:"path,omitempty"`
	HashMd5             string   `json:"hashMd5,omitempty"`
	Name                string   `json:"name,omitempty"`
	Category            string   `json:"category,omitempty"`
	ComponentType       string   `json:"componentType,omitempty"`
	OperatingSystem     string   `json:"operatingSystem,omitempty"`
	SystemIDs           []string `json:"systemIDs,omitempty"`
	Custom              bool     `json:"custom,omitempty"`
	NeedsAttention      bool     `json:"needsAttention,omitempty"`
	Ignore              bool     `json:"ignore,omitempty"`
	OriginalVersion     string   `json:"originalVersion,omitempty"`
	OriginalComponentID string   `json:"originalComponentId,omitempty"`
	FirmwareRepoName    string   `json:"firmwareRepoName,omitempty"`
}

// SoftwareBundles defines a struct for software bundles.
type SoftwareBundles struct {
	ID                 string               `json:"id,omitempty"`
	Name               string               `json:"name,omitempty"`
	Version            string               `json:"version,omitempty"`
	BundleDate         string               `json:"bundleDate,omitempty"`
	CreatedDate        string               `json:"createdDate,omitempty"`
	CreatedBy          string               `json:"createdBy,omitempty"`
	UpdatedDate        string               `json:"updatedDate,omitempty"`
	UpdatedBy          string               `json:"updatedBy,omitempty"`
	Description        string               `json:"description,omitempty"`
	UserBundle         bool                 `json:"userBundle,omitempty"`
	UserBundlePath     string               `json:"userBundlePath,omitempty"`
	UserBundleHashMd5  string               `json:"userBundleHashMd5,omitempty"`
	DeviceType         string               `json:"deviceType,omitempty"`
	DeviceModel        string               `json:"deviceModel,omitempty"`
	Criticality        string               `json:"criticality,omitempty"`
	FwRepositoryID     string               `json:"fwRepositoryId,omitempty"`
	Link               Link                 `json:"link,omitempty"`
	BundleType         string               `json:"bundleType,omitempty"`
	Custom             bool                 `json:"custom,omitempty"`
	NeedsAttention     bool                 `json:"needsAttention,omitempty"`
	SoftwareComponents []SoftwareComponents `json:"softwareComponents,omitempty"`
}

// DeploymentValid defines the validity of a deployment.
type DeploymentValid struct {
	Valid    bool       `json:"valid,omitempty"`
	Messages []Messages `json:"messages,omitempty"`
}

// DeploymentDevice defines the details of a device in a deployment.
type DeploymentDevice struct {
	RefID            string `json:"refId,omitempty"`
	RefType          string `json:"refType,omitempty"`
	LogDump          string `json:"logDump,omitempty"`
	Status           string `json:"status,omitempty"`
	StatusEndTime    string `json:"statusEndTime,omitempty"`
	StatusStartTime  string `json:"statusStartTime,omitempty"`
	DeviceHealth     string `json:"deviceHealth,omitempty"`
	HealthMessage    string `json:"healthMessage,omitempty"`
	CompliantState   string `json:"compliantState,omitempty"`
	BrownfieldStatus string `json:"brownfieldStatus,omitempty"`
	DeviceType       string `json:"deviceType,omitempty"`
	DeviceGroupName  string `json:"deviceGroupName,omitempty"`
	IPAddress        string `json:"ipAddress,omitempty"`
	CurrentIPAddress string `json:"currentIpAddress,omitempty"`
	ServiceTag       string `json:"serviceTag,omitempty"`
	ComponentID      string `json:"componentId,omitempty"`
	StatusMessage    string `json:"statusMessage,omitempty"`
	Model            string `json:"model,omitempty"`
	CloudLink        bool   `json:"cloudLink,omitempty"`
	DasCache         bool   `json:"dasCache,omitempty"`
	DeviceState      string `json:"deviceState,omitempty"`
	PuppetCertName   string `json:"puppetCertName,omitempty"`
	Brownfield       bool   `json:"brownfield,omitempty"`
}

// Vms defines a struct for virtual machines.
type Vms struct {
	CertificateName string `json:"certificateName,omitempty"`
	VMModel         string `json:"vmModel,omitempty"`
	VMIpaddress     string `json:"vmIpaddress,omitempty"`
	VMManufacturer  string `json:"vmManufacturer,omitempty"`
	VMServiceTag    string `json:"vmServiceTag,omitempty"`
}

// LicenseRepository defines the details of a license repository.
type LicenseRepository struct {
	ID           string `json:"id,omitempty"`
	Name         string `json:"name,omitempty"`
	Type         string `json:"type,omitempty"`
	DiskLocation string `json:"diskLocation,omitempty"`
	Filename     string `json:"filename,omitempty"`
	State        string `json:"state,omitempty"`
	CreatedDate  string `json:"createdDate,omitempty"`
	CreatedBy    string `json:"createdBy,omitempty"`
	UpdatedDate  string `json:"updatedDate,omitempty"`
	UpdatedBy    string `json:"updatedBy,omitempty"`
	Templates    []any  `json:"templates,omitempty"`
	LicenseData  string `json:"licenseData,omitempty"`
}

// AssignedUsers defines a struct for assigned users.
type AssignedUsers struct {
	UserSeqID      int      `json:"userSeqId,omitempty"`
	UserName       string   `json:"userName,omitempty"`
	Password       string   `json:"password,omitempty"`
	UpdatePassword bool     `json:"updatePassword,omitempty"`
	DomainName     string   `json:"domainName,omitempty"`
	GroupDN        string   `json:"groupDN,omitempty"`
	GroupName      string   `json:"groupName,omitempty"`
	FirstName      string   `json:"firstName,omitempty"`
	LastName       string   `json:"lastName,omitempty"`
	Email          string   `json:"email,omitempty"`
	PhoneNumber    string   `json:"phoneNumber,omitempty"`
	Enabled        bool     `json:"enabled,omitempty"`
	SystemUser     bool     `json:"systemUser,omitempty"`
	CreatedDate    string   `json:"createdDate,omitempty"`
	CreatedBy      string   `json:"createdBy,omitempty"`
	UpdatedDate    string   `json:"updatedDate,omitempty"`
	UpdatedBy      string   `json:"updatedBy,omitempty"`
	Link           Link     `json:"link,omitempty"`
	Role           string   `json:"role,omitempty"`
	UserPreference string   `json:"userPreference,omitempty"`
	ID             string   `json:"id,omitempty"`
	Roles          []string `json:"roles,omitempty"`
}

// JobDetails defines the details of a job.
type JobDetails struct {
	Level       string `json:"level,omitempty"`
	Message     string `json:"message,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
	ExecutionID string `json:"executionId,omitempty"`
	ComponentID string `json:"componentId,omitempty"`
}

// DeploymentValidationResponse defines the response of deployment validation.
type DeploymentValidationResponse struct {
	Nodes                  int      `json:"nodes,omitempty"`
	StoragePools           int      `json:"storagePools,omitempty"`
	DrivesPerStoragePool   int      `json:"drivesPerStoragePool,omitempty"`
	MaxScalability         int      `json:"maxScalability,omitempty"`
	VirtualMachines        int      `json:"virtualMachines,omitempty"`
	NumberOfServiceVolumes int      `json:"numberOfServiceVolumes,omitempty"`
	CanDeploy              bool     `json:"canDeploy,omitempty"`
	WarningMessages        []string `json:"warningMessages,omitempty"`
	StoragePoolDiskType    []string `json:"storagePoolDiskType,omitempty"`
	Hostnames              []string `json:"hostnames,omitempty"`
	NewNodeDiskTypes       []string `json:"newNodeDiskTypes,omitempty"`
	NoOfFaultSets          int      `json:"noOfFaultSets,omitempty"`
	NodesPerFaultSet       int      `json:"nodesPerFaultSet,omitempty"`
	ProtectionDomain       string   `json:"protectionDomain,omitempty"`
	DiskTypeMismatch       bool     `json:"diskTypeMismatch,omitempty"`
}

// Deployments defines the details of a deployment.
type Deployments struct {
	ID                           string                       `json:"id,omitempty"`
	DeploymentName               string                       `json:"deploymentName,omitempty"`
	DeploymentDescription        string                       `json:"deploymentDescription,omitempty"`
	DeploymentValid              DeploymentValid              `json:"deploymentValid,omitempty"`
	Retry                        bool                         `json:"retry,omitempty"`
	Teardown                     bool                         `json:"teardown,omitempty"`
	TeardownAfterCancel          bool                         `json:"teardownAfterCancel,omitempty"`
	RemoveService                bool                         `json:"removeService,omitempty"`
	CreatedDate                  string                       `json:"createdDate,omitempty"`
	CreatedBy                    string                       `json:"createdBy,omitempty"`
	UpdatedDate                  string                       `json:"updatedDate,omitempty"`
	UpdatedBy                    string                       `json:"updatedBy,omitempty"`
	DeploymentScheduledDate      string                       `json:"deploymentScheduledDate,omitempty"`
	DeploymentStartedDate        string                       `json:"deploymentStartedDate,omitempty"`
	DeploymentFinishedDate       string                       `json:"deploymentFinishedDate,omitempty"`
	ScheduleDate                 string                       `json:"scheduleDate,omitempty"`
	Status                       string                       `json:"status,omitempty"`
	Compliant                    bool                         `json:"compliant,omitempty"`
	DeploymentDevice             []DeploymentDevice           `json:"deploymentDevice,omitempty"`
	Vms                          []Vms                        `json:"vms,omitempty"`
	UpdateServerFirmware         bool                         `json:"updateServerFirmware,omitempty"`
	UseDefaultCatalog            bool                         `json:"useDefaultCatalog,omitempty"`
	FirmwareRepositoryID         string                       `json:"firmwareRepositoryId,omitempty"`
	LicenseRepository            LicenseRepository            `json:"licenseRepository,omitempty"`
	LicenseRepositoryID          string                       `json:"licenseRepositoryId,omitempty"`
	IndividualTeardown           bool                         `json:"individualTeardown,omitempty"`
	DeploymentHealthStatusType   string                       `json:"deploymentHealthStatusType,omitempty"`
	AssignedUsers                []AssignedUsers              `json:"assignedUsers,omitempty"`
	AllUsersAllowed              bool                         `json:"allUsersAllowed,omitempty"`
	Owner                        string                       `json:"owner,omitempty"`
	NoOp                         bool                         `json:"noOp,omitempty"`
	FirmwareInit                 bool                         `json:"firmwareInit,omitempty"`
	DisruptiveFirmware           bool                         `json:"disruptiveFirmware,omitempty"`
	PreconfigureSVM              bool                         `json:"preconfigureSVM,omitempty"`
	PreconfigureSVMAndUpdate     bool                         `json:"preconfigureSVMAndUpdate,omitempty"`
	ServicesDeployed             string                       `json:"servicesDeployed,omitempty"`
	PrecalculatedDeviceHealth    string                       `json:"precalculatedDeviceHealth,omitempty"`
	LifecycleModeReasons         []string                     `json:"lifecycleModeReasons,omitempty"`
	JobDetails                   []JobDetails                 `json:"jobDetails,omitempty"`
	NumberOfDeployments          int                          `json:"numberOfDeployments,omitempty"`
	OperationType                string                       `json:"operationType,omitempty"`
	OperationStatus              string                       `json:"operationStatus,omitempty"`
	OperationData                string                       `json:"operationData,omitempty"`
	DeploymentValidationResponse DeploymentValidationResponse `json:"deploymentValidationResponse,omitempty"`
	CurrentStepCount             string                       `json:"currentStepCount,omitempty"`
	TotalNumOfSteps              string                       `json:"totalNumOfSteps,omitempty"`
	CurrentStepMessage           string                       `json:"currentStepMessage,omitempty"`
	CustomImage                  string                       `json:"customImage,omitempty"`
	OriginalDeploymentID         string                       `json:"originalDeploymentId,omitempty"`
	CurrentBatchCount            string                       `json:"currentBatchCount,omitempty"`
	TotalBatchCount              string                       `json:"totalBatchCount,omitempty"`
	Brownfield                   bool                         `json:"brownfield,omitempty"`
	ScaleUp                      bool                         `json:"scaleUp,omitempty"`
	LifecycleMode                bool                         `json:"lifecycleMode,omitempty"`
	OverallDeviceHealth          string                       `json:"overallDeviceHealth,omitempty"`
	Vds                          bool                         `json:"vds,omitempty"`
	TemplateValid                bool                         `json:"templateValid,omitempty"`
	ConfigurationChange          bool                         `json:"configurationChange,omitempty"`
	CanMigratevCLSVMs            bool                         `json:"canMigratevCLSVMs,omitempty"`
}

// FirmwareRepository defines the details of a firmware repository.
type FirmwareRepository struct {
	ID                      string               `json:"id,omitempty"`
	Name                    string               `json:"name,omitempty"`
	SourceLocation          string               `json:"sourceLocation,omitempty"`
	SourceType              string               `json:"sourceType,omitempty"`
	DiskLocation            string               `json:"diskLocation,omitempty"`
	Filename                string               `json:"filename,omitempty"`
	Md5Hash                 string               `json:"md5Hash,omitempty"`
	Username                string               `json:"username,omitempty"`
	Password                string               `json:"password,omitempty"`
	DownloadStatus          string               `json:"downloadStatus,omitempty"`
	CreatedDate             string               `json:"createdDate,omitempty"`
	CreatedBy               string               `json:"createdBy,omitempty"`
	UpdatedDate             string               `json:"updatedDate,omitempty"`
	UpdatedBy               string               `json:"updatedBy,omitempty"`
	DefaultCatalog          bool                 `json:"defaultCatalog,omitempty"`
	Embedded                bool                 `json:"embedded,omitempty"`
	State                   string               `json:"state,omitempty"`
	SoftwareComponents      []SoftwareComponents `json:"softwareComponents,omitempty"`
	SoftwareBundles         []SoftwareBundles    `json:"softwareBundles,omitempty"`
	Deployments             []Deployments        `json:"deployments,omitempty"`
	BundleCount             int                  `json:"bundleCount,omitempty"`
	ComponentCount          int                  `json:"componentCount,omitempty"`
	UserBundleCount         int                  `json:"userBundleCount,omitempty"`
	Minimal                 bool                 `json:"minimal,omitempty"`
	DownloadProgress        int                  `json:"downloadProgress,omitempty"`
	ExtractProgress         int                  `json:"extractProgress,omitempty"`
	FileSizeInGigabytes     int                  `json:"fileSizeInGigabytes,omitempty"`
	SignedKeySourceLocation string               `json:"signedKeySourceLocation,omitempty"`
	Signature               string               `json:"signature,omitempty"`
	Custom                  bool                 `json:"custom,omitempty"`
	NeedsAttention          bool                 `json:"needsAttention,omitempty"`
	JobID                   string               `json:"jobId,omitempty"`
	Rcmapproved             bool                 `json:"rcmapproved,omitempty"`
}

// ComponentValid defines the validity of a component.
type ComponentValid struct {
	Valid    bool       `json:"valid,omitempty"`
	Messages []Messages `json:"messages,omitempty"`
}

// DependenciesDetails defines the details of a dependency.
type DependenciesDetails struct {
	ID               string `json:"id,omitempty"`
	DependencyTarget string `json:"dependencyTarget,omitempty"`
	DependencyValue  string `json:"dependencyValue,omitempty"`
}

// NetworkIPAddressList defines the details of a network IP address.
type NetworkIPAddressList struct {
	ID        string `json:"id,omitempty"`
	IPAddress string `json:"ipAddress,omitempty"`
}

// Partitions defines the details of a partition.
type Partitions struct {
	ID                   string                 `json:"id,omitempty"`
	Name                 string                 `json:"name,omitempty"`
	Networks             []string               `json:"networks,omitempty"`
	NetworkIPAddressList []NetworkIPAddressList `json:"networkIpAddressList,omitempty"`
	Minimum              int                    `json:"minimum,omitempty"`
	Maximum              int                    `json:"maximum,omitempty"`
	LanMacAddress        string                 `json:"lanMacAddress,omitempty"`
	IscsiMacAddress      string                 `json:"iscsiMacAddress,omitempty"`
	IscsiIQN             string                 `json:"iscsiIQN,omitempty"`
	Wwnn                 string                 `json:"wwnn,omitempty"`
	Wwpn                 string                 `json:"wwpn,omitempty"`
	Fqdd                 string                 `json:"fqdd,omitempty"`
	MirroredPort         string                 `json:"mirroredPort,omitempty"`
	MacAddress           string                 `json:"mac_address,omitempty"`
	PortNo               int                    `json:"port_no,omitempty"`
	PartitionNo          int                    `json:"partition_no,omitempty"`
	PartitionIndex       int                    `json:"partition_index,omitempty"`
}

// Interfaces defines an interface.
type Interfaces struct {
	ID            string       `json:"id,omitempty"`
	Name          string       `json:"name,omitempty"`
	Partitioned   bool         `json:"partitioned,omitempty"`
	Partitions    []Partitions `json:"partitions,omitempty"`
	Enabled       bool         `json:"enabled,omitempty"`
	Redundancy    bool         `json:"redundancy,omitempty"`
	Nictype       string       `json:"nictype,omitempty"`
	Fqdd          string       `json:"fqdd,omitempty"`
	MaxPartitions int          `json:"maxPartitions,omitempty"`
	AllNetworks   []string     `json:"allNetworks,omitempty"`
}

// InterfacesDetails defines the details of an interface.
type InterfacesDetails struct {
	ID            string       `json:"id,omitempty"`
	Name          string       `json:"name,omitempty"`
	Redundancy    bool         `json:"redundancy,omitempty"`
	Enabled       bool         `json:"enabled,omitempty"`
	Partitioned   bool         `json:"partitioned,omitempty"`
	Interfaces    []Interfaces `json:"interfaces,omitempty"`
	Nictype       string       `json:"nictype,omitempty"`
	Fabrictype    string       `json:"fabrictype,omitempty"`
	MaxPartitions int          `json:"maxPartitions,omitempty"`
	Nports        int          `json:"nports,omitempty"`
	CardIndex     int          `json:"card_index,omitempty"`
	NictypeSource string       `json:"nictypeSource,omitempty"`
}

// NetworkConfiguration defines the network configuration.
type NetworkConfiguration struct {
	ID           string              `json:"id,omitempty"`
	Interfaces   []InterfacesDetails `json:"interfaces,omitempty"`
	SoftwareOnly bool                `json:"softwareOnly,omitempty"`
}

// ConfigurationDetails defines the details of a configuration.
type ConfigurationDetails struct {
	ID              string       `json:"id,omitempty"`
	Disktype        string       `json:"disktype,omitempty"`
	Comparator      string       `json:"comparator,omitempty"`
	Numberofdisks   int          `json:"numberofdisks,omitempty"`
	Raidlevel       string       `json:"raidlevel,omitempty"`
	VirtualDiskFqdd string       `json:"virtualDiskFqdd,omitempty"`
	ControllerFqdd  string       `json:"controllerFqdd,omitempty"`
	Categories      []Categories `json:"categories,omitempty"`
}

// VirtualDisks defines the details of a virtual disk.
type VirtualDisks struct {
	PhysicalDisks         []string             `json:"physicalDisks,omitempty"`
	VirtualDiskFqdd       string               `json:"virtualDiskFqdd,omitempty"`
	RaidLevel             string               `json:"raidLevel,omitempty"`
	RollUpStatus          string               `json:"rollUpStatus,omitempty"`
	Controller            string               `json:"controller,omitempty"`
	ControllerProductName string               `json:"controllerProductName,omitempty"`
	Configuration         ConfigurationDetails `json:"configuration,omitempty"`
	MediaType             string               `json:"mediaType,omitempty"`
	EncryptionType        string               `json:"encryptionType,omitempty"`
}

// ExternalVirtualDisks defines the details of an external virtual disk.
type ExternalVirtualDisks struct {
	PhysicalDisks         []string             `json:"physicalDisks,omitempty"`
	VirtualDiskFqdd       string               `json:"virtualDiskFqdd,omitempty"`
	RaidLevel             string               `json:"raidLevel,omitempty"`
	RollUpStatus          string               `json:"rollUpStatus,omitempty"`
	Controller            string               `json:"controller,omitempty"`
	ControllerProductName string               `json:"controllerProductName,omitempty"`
	Configuration         ConfigurationDetails `json:"configuration,omitempty"`
	MediaType             string               `json:"mediaType,omitempty"`
	EncryptionType        string               `json:"encryptionType,omitempty"`
}

// RaidConfiguration defines the raid configuration.
type RaidConfiguration struct {
	VirtualDisks         []VirtualDisks         `json:"virtualDisks,omitempty"`
	ExternalVirtualDisks []ExternalVirtualDisks `json:"externalVirtualDisks,omitempty"`
	HddHotSpares         []string               `json:"hddHotSpares,omitempty"`
	SsdHotSpares         []string               `json:"ssdHotSpares,omitempty"`
	ExternalHddHotSpares []string               `json:"externalHddHotSpares,omitempty"`
	ExternalSsdHotSpares []string               `json:"externalSsdHotSpares,omitempty"`
	SizeToDiskMap        map[string]int         `json:"sizeToDiskMap,omitempty"`
}

// OptionsDetails defines the details of an option.
type OptionsDetails struct {
	ID           string                `json:"id,omitempty"`
	Name         string                `json:"name,omitempty"`
	Value        string                `json:"value,omitempty"`
	Dependencies []DependenciesDetails `json:"dependencies,omitempty"`
	Attributes   map[string]string     `json:"attributes,omitempty"`
}

// ScaleIOStoragePoolDisks defines the scaleio storage pool disks.
type ScaleIOStoragePoolDisks struct {
	ProtectionDomainID   string   `json:"protectionDomainId,omitempty"`
	ProtectionDomainName string   `json:"protectionDomainName,omitempty"`
	StoragePoolID        string   `json:"storagePoolId,omitempty"`
	StoragePoolName      string   `json:"storagePoolName,omitempty"`
	DiskType             string   `json:"diskType,omitempty"`
	PhysicalDiskFqdds    []string `json:"physicalDiskFqdds,omitempty"`
	VirtualDiskFqdds     []string `json:"virtualDiskFqdds,omitempty"`
	SoftwareOnlyDisks    []string `json:"softwareOnlyDisks,omitempty"`
}

// ScaleIODiskConfiguration defines the scaleio disk configuration.
type ScaleIODiskConfiguration struct {
	ScaleIOStoragePoolDisks []ScaleIOStoragePoolDisks `json:"scaleIOStoragePoolDisks,omitempty"`
}

// ShortWindow defines a short window.
type ShortWindow struct {
	Threshold       int `json:"threshold,omitempty"`
	WindowSizeInSec int `json:"windowSizeInSec,omitempty"`
}

// MediumWindow defines a medium window.
type MediumWindow struct {
	Threshold       int `json:"threshold,omitempty"`
	WindowSizeInSec int `json:"windowSizeInSec,omitempty"`
}

// LongWindow defines a long window.
type LongWindow struct {
	Threshold       int `json:"threshold,omitempty"`
	WindowSizeInSec int `json:"windowSizeInSec,omitempty"`
}

// SdsDecoupledCounterParameters defines the decoupled parameters for sds counter.
type SdsDecoupledCounterParameters struct {
	ShortWindow  ShortWindow  `json:"shortWindow,omitempty"`
	MediumWindow MediumWindow `json:"mediumWindow,omitempty"`
	LongWindow   LongWindow   `json:"longWindow,omitempty"`
}

// SdsConfigurationFailureCounterParameters defines the sds configuration failure counter parameters.
type SdsConfigurationFailureCounterParameters struct {
	ShortWindow  ShortWindow  `json:"shortWindow,omitempty"`
	MediumWindow MediumWindow `json:"mediumWindow,omitempty"`
	LongWindow   LongWindow   `json:"longWindow,omitempty"`
}

// MdmSdsCounterParameters defines the mdm sds counter parameters.
type MdmSdsCounterParameters struct {
	ShortWindow  ShortWindow  `json:"shortWindow,omitempty"`
	MediumWindow MediumWindow `json:"mediumWindow,omitempty"`
	LongWindow   LongWindow   `json:"longWindow,omitempty"`
}

// SdsSdsCounterParameters defines the sds sds counter parameters.
type SdsSdsCounterParameters struct {
	ShortWindow  ShortWindow  `json:"shortWindow,omitempty"`
	MediumWindow MediumWindow `json:"mediumWindow,omitempty"`
	LongWindow   LongWindow   `json:"longWindow,omitempty"`
}

// SdsReceiveBufferAllocationFailuresCounterParameters defines the sds receive buffer allocation failures counter parameters.
type SdsReceiveBufferAllocationFailuresCounterParameters struct {
	ShortWindow  ShortWindow  `json:"shortWindow,omitempty"`
	MediumWindow MediumWindow `json:"mediumWindow,omitempty"`
	LongWindow   LongWindow   `json:"longWindow,omitempty"`
}

// General defines the general.
type General struct {
	ID                                                  string                                              `json:"id,omitempty"`
	Name                                                string                                              `json:"name,omitempty"`
	SystemID                                            string                                              `json:"systemId,omitempty"`
	ProtectionDomainState                               string                                              `json:"protectionDomainState,omitempty"`
	RebuildNetworkThrottlingInKbps                      int                                                 `json:"rebuildNetworkThrottlingInKbps,omitempty"`
	RebalanceNetworkThrottlingInKbps                    int                                                 `json:"rebalanceNetworkThrottlingInKbps,omitempty"`
	OverallIoNetworkThrottlingInKbps                    int                                                 `json:"overallIoNetworkThrottlingInKbps,omitempty"`
	SdsDecoupledCounterParameters                       SdsDecoupledCounterParameters                       `json:"sdsDecoupledCounterParameters,omitempty"`
	SdsConfigurationFailureCounterParameters            SdsConfigurationFailureCounterParameters            `json:"sdsConfigurationFailureCounterParameters,omitempty"`
	MdmSdsCounterParameters                             MdmSdsCounterParameters                             `json:"mdmSdsCounterParameters,omitempty"`
	SdsSdsCounterParameters                             SdsSdsCounterParameters                             `json:"sdsSdsCounterParameters,omitempty"`
	RfcacheOpertionalMode                               string                                              `json:"rfcacheOpertionalMode,omitempty"`
	RfcachePageSizeKb                                   int                                                 `json:"rfcachePageSizeKb,omitempty"`
	RfcacheMaxIoSizeKb                                  int                                                 `json:"rfcacheMaxIoSizeKb,omitempty"`
	SdsReceiveBufferAllocationFailuresCounterParameters SdsReceiveBufferAllocationFailuresCounterParameters `json:"sdsReceiveBufferAllocationFailuresCounterParameters,omitempty"`
	RebuildNetworkThrottlingEnabled                     bool                                                `json:"rebuildNetworkThrottlingEnabled,omitempty"`
	RebalanceNetworkThrottlingEnabled                   bool                                                `json:"rebalanceNetworkThrottlingEnabled,omitempty"`
	OverallIoNetworkThrottlingEnabled                   bool                                                `json:"overallIoNetworkThrottlingEnabled,omitempty"`
	RfcacheEnabled                                      bool                                                `json:"rfcacheEnabled,omitempty"`
}

// StatisticsDetails defines the statistics details.
type StatisticsDetails struct {
	NumOfDevices                             int `json:"numOfDevices,omitempty"`
	UnusedCapacityInKb                       int `json:"unusedCapacityInKb,omitempty"`
	NumOfVolumes                             int `json:"numOfVolumes,omitempty"`
	NumOfMappedToAllVolumes                  int `json:"numOfMappedToAllVolumes,omitempty"`
	CapacityAvailableForVolumeAllocationInKb int `json:"capacityAvailableForVolumeAllocationInKb,omitempty"`
	VolumeAllocationLimitInKb                int `json:"volumeAllocationLimitInKb,omitempty"`
	CapacityLimitInKb                        int `json:"capacityLimitInKb,omitempty"`
	NumOfUnmappedVolumes                     int `json:"numOfUnmappedVolumes,omitempty"`
	SpareCapacityInKb                        int `json:"spareCapacityInKb,omitempty"`
	CapacityInUseInKb                        int `json:"capacityInUseInKb,omitempty"`
	MaxCapacityInKb                          int `json:"maxCapacityInKb,omitempty"`

	NumOfSds int `json:"numOfSds,omitempty"`

	NumOfStoragePools int `json:"numOfStoragePools,omitempty"`
	NumOfFaultSets    int `json:"numOfFaultSets,omitempty"`

	ThinCapacityInUseInKb  int `json:"thinCapacityInUseInKb,omitempty"`
	ThickCapacityInUseInKb int `json:"thickCapacityInUseInKb,omitempty"`
}

// DiskList defines the disk list.
type DiskList struct {
	ID                     string `json:"id,omitempty"`
	Name                   string `json:"name,omitempty"`
	ErrorState             string `json:"errorState,omitempty"`
	SdsID                  string `json:"sdsId,omitempty"`
	DeviceState            string `json:"deviceState,omitempty"`
	CapacityLimitInKb      int    `json:"capacityLimitInKb,omitempty"`
	MaxCapacityInKb        int    `json:"maxCapacityInKb,omitempty"`
	StoragePoolID          string `json:"storagePoolId,omitempty"`
	DeviceCurrentPathName  string `json:"deviceCurrentPathName,omitempty"`
	DeviceOriginalPathName string `json:"deviceOriginalPathName,omitempty"`
	SerialNumber           string `json:"serialNumber,omitempty"`
	VendorName             string `json:"vendorName,omitempty"`
	ModelName              string `json:"modelName,omitempty"`
}

// MappedSdcInfoDetails defines the mapped sdc info details.
type MappedSdcInfoDetails struct {
	SdcIP         string `json:"sdcIp,omitempty"`
	SdcID         string `json:"sdcId,omitempty"`
	LimitBwInMbps int    `json:"limitBwInMbps,omitempty"`
	LimitIops     int    `json:"limitIops,omitempty"`
}

// VolumeList defines the volume list.
type VolumeList struct {
	ID                string                 `json:"id,omitempty"`
	Name              string                 `json:"name,omitempty"`
	VolumeType        string                 `json:"volumeType,omitempty"`
	StoragePoolID     string                 `json:"storagePoolId,omitempty"`
	DataLayout        string                 `json:"dataLayout,omitempty"`
	CompressionMethod string                 `json:"compressionMethod,omitempty"`
	SizeInKb          int                    `json:"sizeInKb,omitempty"`
	MappedSdcInfo     []MappedSdcInfoDetails `json:"mappedSdcInfo,omitempty"`
	VolumeClass       string                 `json:"volumeClass,omitempty"`
}

// StoragePoolList defines the storage pool list.
type StoragePoolList struct {
	ID                                               string            `json:"id,omitempty"`
	Name                                             string            `json:"name,omitempty"`
	RebuildIoPriorityPolicy                          string            `json:"rebuildIoPriorityPolicy,omitempty"`
	RebalanceIoPriorityPolicy                        string            `json:"rebalanceIoPriorityPolicy,omitempty"`
	RebuildIoPriorityNumOfConcurrentIosPerDevice     int               `json:"rebuildIoPriorityNumOfConcurrentIosPerDevice,omitempty"`
	RebalanceIoPriorityNumOfConcurrentIosPerDevice   int               `json:"rebalanceIoPriorityNumOfConcurrentIosPerDevice,omitempty"`
	RebuildIoPriorityBwLimitPerDeviceInKbps          int               `json:"rebuildIoPriorityBwLimitPerDeviceInKbps,omitempty"`
	RebalanceIoPriorityBwLimitPerDeviceInKbps        int               `json:"rebalanceIoPriorityBwLimitPerDeviceInKbps,omitempty"`
	RebuildIoPriorityAppIopsPerDeviceThreshold       string            `json:"rebuildIoPriorityAppIopsPerDeviceThreshold,omitempty"`
	RebalanceIoPriorityAppIopsPerDeviceThreshold     string            `json:"rebalanceIoPriorityAppIopsPerDeviceThreshold,omitempty"`
	RebuildIoPriorityAppBwPerDeviceThresholdInKbps   int               `json:"rebuildIoPriorityAppBwPerDeviceThresholdInKbps,omitempty"`
	RebalanceIoPriorityAppBwPerDeviceThresholdInKbps int               `json:"rebalanceIoPriorityAppBwPerDeviceThresholdInKbps,omitempty"`
	RebuildIoPriorityQuietPeriodInMsec               int               `json:"rebuildIoPriorityQuietPeriodInMsec,omitempty"`
	RebalanceIoPriorityQuietPeriodInMsec             int               `json:"rebalanceIoPriorityQuietPeriodInMsec,omitempty"`
	ZeroPaddingEnabled                               bool              `json:"zeroPaddingEnabled,omitempty"`
	BackgroundScannerMode                            string            `json:"backgroundScannerMode,omitempty"`
	BackgroundScannerBWLimitKBps                     int               `json:"backgroundScannerBWLimitKBps,omitempty"`
	UseRmcache                                       bool              `json:"useRmcache,omitempty"`
	ProtectionDomainID                               string            `json:"protectionDomainId,omitempty"`
	SpClass                                          string            `json:"spClass,omitempty"`
	UseRfcache                                       bool              `json:"useRfcache,omitempty"`
	SparePercentage                                  int               `json:"sparePercentage,omitempty"`
	RmcacheWriteHandlingMode                         string            `json:"rmcacheWriteHandlingMode,omitempty"`
	ChecksumEnabled                                  bool              `json:"checksumEnabled,omitempty"`
	RebuildEnabled                                   bool              `json:"rebuildEnabled,omitempty"`
	RebalanceEnabled                                 bool              `json:"rebalanceEnabled,omitempty"`
	NumOfParallelRebuildRebalanceJobsPerDevice       int               `json:"numOfParallelRebuildRebalanceJobsPerDevice,omitempty"`
	CapacityAlertHighThreshold                       int               `json:"capacityAlertHighThreshold,omitempty"`
	CapacityAlertCriticalThreshold                   int               `json:"capacityAlertCriticalThreshold,omitempty"`
	Statistics                                       StatisticsDetails `json:"statistics,omitempty"`
	DataLayout                                       string            `json:"dataLayout,omitempty"`
	ReplicationCapacityMaxRatio                      string            `json:"replicationCapacityMaxRatio,omitempty"`
	MediaType                                        string            `json:"mediaType,omitempty"`
	DiskList                                         []DiskList        `json:"disk_list,omitempty"`
	VolumeList                                       []VolumeList      `json:"volume_list,omitempty"`
	FglAccpID                                        string            `json:"fglAccpId,omitempty"`
}

// IPList defines the ip list.
type IPList struct {
	IP   string `json:"ip,omitempty"`
	Role string `json:"role,omitempty"`
}

// SdsListDetails defines the sds list.
type SdsListDetails struct {
	ID                  string   `json:"id,omitempty"`
	Name                string   `json:"name,omitempty"`
	Port                int      `json:"port,omitempty"`
	ProtectionDomainID  string   `json:"protectionDomainId,omitempty"`
	FaultSetID          string   `json:"faultSetId,omitempty"`
	SoftwareVersionInfo string   `json:"softwareVersionInfo,omitempty"`
	SdsState            string   `json:"sdsState,omitempty"`
	MembershipState     string   `json:"membershipState,omitempty"`
	MdmConnectionState  string   `json:"mdmConnectionState,omitempty"`
	DrlMode             string   `json:"drlMode,omitempty"`
	MaintenanceState    string   `json:"maintenanceState,omitempty"`
	PerfProfile         string   `json:"perfProfile,omitempty"`
	OnVMWare            bool     `json:"onVmWare,omitempty"`
	IPList              []IPList `json:"ipList,omitempty"`
}

// SdrListDetails defines the sdr list.
type SdrListDetails struct {
	ID                  string   `json:"id,omitempty"`
	Name                string   `json:"name,omitempty"`
	Port                int      `json:"port,omitempty"`
	ProtectionDomainID  string   `json:"protectionDomainId,omitempty"`
	SoftwareVersionInfo string   `json:"softwareVersionInfo,omitempty"`
	SdrState            string   `json:"sdrState,omitempty"`
	MembershipState     string   `json:"membershipState,omitempty"`
	MdmConnectionState  string   `json:"mdmConnectionState,omitempty"`
	MaintenanceState    string   `json:"maintenanceState,omitempty"`
	PerfProfile         string   `json:"perfProfile,omitempty"`
	IPList              []IPList `json:"ipList,omitempty"`
}

// AccelerationPool defines the acceleration pool.
type AccelerationPool struct {
	ID                 string `json:"id,omitempty"`
	Name               string `json:"name,omitempty"`
	ProtectionDomainID string `json:"protectionDomainId,omitempty"`
	MediaType          string `json:"mediaType,omitempty"`
	Rfcache            bool   `json:"rfcache,omitempty"`
}

// ProtectionDomainSettings defines the protection domain settings.
type ProtectionDomainSettings struct {
	General          General            `json:"general,omitempty"`
	Statistics       StatisticsDetails  `json:"statistics,omitempty"`
	StoragePoolList  []StoragePoolList  `json:"storage_pool_list,omitempty"`
	SdsList          []SdsListDetails   `json:"sds_list,omitempty"`
	SdrList          []SdrListDetails   `json:"sdr_list,omitempty"`
	AccelerationPool []AccelerationPool `json:"acceleration_pool,omitempty"`
}

// FaultSetSettings defines the fault set settings.
type FaultSetSettings struct {
	ProtectionDomainID string `json:"protectionDomainId,omitempty"`
	Name               string `json:"name,omitempty"`
	ID                 string `json:"id,omitempty"`
}

// Datacenter defines the datacenter.
type Datacenter struct {
	VcenterID      string `json:"vcenterId,omitempty"`
	DatacenterID   string `json:"datacenterId,omitempty"`
	DatacenterName string `json:"datacenterName,omitempty"`
}

// PortGroupOptions defines the port group options.
type PortGroupOptions struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// PortGroups defines the port groups.
type PortGroups struct {
	ID               string             `json:"id,omitempty"`
	DisplayName      string             `json:"displayName,omitempty"`
	Vlan             int                `json:"vlan,omitempty"`
	Name             string             `json:"name,omitempty"`
	Value            string             `json:"value,omitempty"`
	PortGroupOptions []PortGroupOptions `json:"portGroupOptions,omitempty"`
}

// VdsSettings defines the vds settings.
type VdsSettings struct {
	ID          string       `json:"id,omitempty"`
	DisplayName string       `json:"displayName,omitempty"`
	Name        string       `json:"name,omitempty"`
	Value       string       `json:"value,omitempty"`
	PortGroups  []PortGroups `json:"portGroups,omitempty"`
}

// VdsNetworkMtuSizeConfiguration defines the vds network mtu size configuration.
type VdsNetworkMtuSizeConfiguration struct {
	ID    string `json:"id,omitempty"`
	Value string `json:"value,omitempty"`
}

// VdsNetworkMTUSizeConfiguration defines the vds network mtu size configuration.
type VdsNetworkMTUSizeConfiguration struct {
	ID    string `json:"id,omitempty"`
	Value string `json:"value,omitempty"`
}

// VdsConfiguration defines the vds configuration.
type VdsConfiguration struct {
	Datacenter                     Datacenter                       `json:"datacenter,omitempty"`
	PortGroupOption                string                           `json:"portGroupOption,omitempty"`
	PortGroupCreationOption        string                           `json:"portGroupCreationOption,omitempty"`
	VdsSettings                    []VdsSettings                    `json:"vdsSettings,omitempty"`
	VdsNetworkMtuSizeConfiguration []VdsNetworkMtuSizeConfiguration `json:"vdsNetworkMtuSizeConfiguration,omitempty"`
	VdsNetworkMTUSizeConfiguration []VdsNetworkMTUSizeConfiguration `json:"vdsNetworkMTUSizeConfiguration,omitempty"`
}

// NodeSelection defines the node selection.
type NodeSelection struct {
	ID            string `json:"id,omitempty"`
	ServiceTag    string `json:"serviceTag,omitempty"`
	MgmtIPAddress string `json:"mgmtIpAddress,omitempty"`
}

// ParametersDetails defines the parameters details.
type ParametersDetails struct {
	GUID                     string                     `json:"guid,omitempty"`
	ID                       string                     `json:"id,omitempty"`
	Type                     string                     `json:"type,omitempty"`
	DisplayName              string                     `json:"displayName,omitempty"`
	Value                    string                     `json:"value,omitempty"`
	ToolTip                  string                     `json:"toolTip,omitempty"`
	Required                 bool                       `json:"required,omitempty"`
	RequiredAtDeployment     bool                       `json:"requiredAtDeployment,omitempty"`
	HideFromTemplate         bool                       `json:"hideFromTemplate,omitempty"`
	Dependencies             []DependenciesDetails      `json:"dependencies,omitempty"`
	Group                    string                     `json:"group,omitempty"`
	ReadOnly                 bool                       `json:"readOnly,omitempty"`
	Generated                bool                       `json:"generated,omitempty"`
	InfoIcon                 bool                       `json:"infoIcon,omitempty"`
	Step                     int                        `json:"step,omitempty"`
	MaxLength                int                        `json:"maxLength,omitempty"`
	Min                      int                        `json:"min,omitempty"`
	Max                      int                        `json:"max,omitempty"`
	NetworkIPAddressList     []NetworkIPAddressList     `json:"networkIpAddressList,omitempty"`
	NetworkConfiguration     NetworkConfiguration       `json:"networkConfiguration,omitempty"`
	RaidConfiguration        RaidConfiguration          `json:"raidConfiguration,omitempty"`
	Options                  []OptionsDetails           `json:"options,omitempty"`
	OptionsSortable          bool                       `json:"optionsSortable,omitempty"`
	PreservedForDeployment   bool                       `json:"preservedForDeployment,omitempty"`
	ScaleIODiskConfiguration ScaleIODiskConfiguration   `json:"scaleIODiskConfiguration,omitempty"`
	ProtectionDomainSettings []ProtectionDomainSettings `json:"protectionDomainSettings,omitempty"`
	FaultSetSettings         []FaultSetSettings         `json:"faultSetSettings,omitempty"`
	Attributes               map[string]string          `json:"attributes,omitempty"`
	VdsConfiguration         VdsConfiguration           `json:"vdsConfiguration,omitempty"`
	NodeSelection            NodeSelection              `json:"nodeSelection,omitempty"`
}

// AdditionalPropDetails defines the additional prop details.
type AdditionalPropDetails struct {
	GUID                     string                     `json:"guid,omitempty"`
	ID                       string                     `json:"id,omitempty"`
	Type                     string                     `json:"type,omitempty"`
	DisplayName              string                     `json:"displayName,omitempty"`
	Value                    string                     `json:"value,omitempty"`
	ToolTip                  string                     `json:"toolTip,omitempty"`
	Required                 bool                       `json:"required,omitempty"`
	RequiredAtDeployment     bool                       `json:"requiredAtDeployment,omitempty"`
	HideFromTemplate         bool                       `json:"hideFromTemplate,omitempty"`
	Dependencies             []DependenciesDetails      `json:"dependencies,omitempty"`
	Group                    string                     `json:"group,omitempty"`
	ReadOnly                 bool                       `json:"readOnly,omitempty"`
	Generated                bool                       `json:"generated,omitempty"`
	InfoIcon                 bool                       `json:"infoIcon,omitempty"`
	Step                     int                        `json:"step,omitempty"`
	MaxLength                int                        `json:"maxLength,omitempty"`
	Min                      int                        `json:"min,omitempty"`
	Max                      int                        `json:"max,omitempty"`
	NetworkIPAddressList     []NetworkIPAddressList     `json:"networkIpAddressList,omitempty"`
	NetworkConfiguration     NetworkConfiguration       `json:"networkConfiguration,omitempty"`
	RaidConfiguration        RaidConfiguration          `json:"raidConfiguration,omitempty"`
	Options                  []Options                  `json:"options,omitempty"`
	OptionsSortable          bool                       `json:"optionsSortable,omitempty"`
	PreservedForDeployment   bool                       `json:"preservedForDeployment,omitempty"`
	ScaleIODiskConfiguration ScaleIODiskConfiguration   `json:"scaleIODiskConfiguration,omitempty"`
	ProtectionDomainSettings []ProtectionDomainSettings `json:"protectionDomainSettings,omitempty"`
	FaultSetSettings         []FaultSetSettings         `json:"faultSetSettings,omitempty"`
	Attributes               map[string]string          `json:"attributes,omitempty"`
	VdsConfiguration         VdsConfiguration           `json:"vdsConfiguration,omitempty"`
	NodeSelection            NodeSelection              `json:"nodeSelection,omitempty"`
}

// Resources defines the resources.
type Resources struct {
	GUID          string              `json:"guid,omitempty"`
	ID            string              `json:"id,omitempty"`
	DisplayName   string              `json:"displayName,omitempty"`
	Parameters    []ParametersDetails `json:"parameters,omitempty"`
	ParametersMap map[string]string   `json:"parametersMap,omitempty"`
}

// Components defines the components.
type Components struct {
	ID                  string            `json:"id,omitempty"`
	ComponentID         string            `json:"componentID,omitempty"`
	Identifier          string            `json:"identifier,omitempty"`
	ComponentValid      ComponentValid    `json:"componentValid,omitempty"`
	Name                string            `json:"name,omitempty"`
	HelpText            string            `json:"helpText,omitempty"`
	ClonedFromID        string            `json:"clonedFromId,omitempty"`
	Teardown            bool              `json:"teardown,omitempty"`
	Type                string            `json:"type,omitempty"`
	SubType             string            `json:"subType,omitempty"`
	RelatedComponents   map[string]string `json:"relatedComponents,omitempty"`
	Resources           []Resources       `json:"resources,omitempty"`
	Brownfield          bool              `json:"brownfield,omitempty"`
	PuppetCertName      string            `json:"puppetCertName,omitempty"`
	OsPuppetCertName    string            `json:"osPuppetCertName,omitempty"`
	ManagementIPAddress string            `json:"managementIpAddress,omitempty"`
	SerialNumber        string            `json:"serialNumber,omitempty"`
	AsmGUID             string            `json:"asmGUID,omitempty"`
	Cloned              bool              `json:"cloned,omitempty"`
	ConfigFile          string            `json:"configFile,omitempty"`
	ManageFirmware      bool              `json:"manageFirmware,omitempty"`
	Instances           int               `json:"instances,omitempty"`
	RefID               string            `json:"refId,omitempty"`
	ClonedFromAsmGUID   string            `json:"clonedFromAsmGuid,omitempty"`
	Changed             bool              `json:"changed,omitempty"`
	IP                  string            `json:"ip,omitempty"`
}

// IPRange defines the ip range.
type IPRange struct {
	ID         string `json:"id,omitempty"`
	StartingIP string `json:"startingIp,omitempty"`
	EndingIP   string `json:"endingIp,omitempty"`
	Role       string `json:"role,omitempty"`
}

// StaticRoute defines the static route.
type StaticRoute struct {
	StaticRouteSourceNetworkID      string `json:"staticRouteSourceNetworkId,omitempty"`
	StaticRouteDestinationNetworkID string `json:"staticRouteDestinationNetworkId,omitempty"`
	StaticRouteGateway              string `json:"staticRouteGateway,omitempty"`
	SubnetMask                      string `json:"subnetMask,omitempty"`
	DestinationIPAddress            string `json:"destinationIpAddress,omitempty"`
}

// StaticNetworkConfiguration defines the static network configuration.
type StaticNetworkConfiguration struct {
	Gateway      string        `json:"gateway,omitempty"`
	Subnet       string        `json:"subnet,omitempty"`
	PrimaryDNS   string        `json:"primaryDns,omitempty"`
	SecondaryDNS string        `json:"secondaryDns,omitempty"`
	DNSSuffix    string        `json:"dnsSuffix,omitempty"`
	IPRange      []IPRange     `json:"ipRange,omitempty"`
	IPAddress    string        `json:"ipAddress,omitempty"`
	StaticRoute  []StaticRoute `json:"staticRoute,omitempty"`
}

// Networks defines the details of a network.
type Networks struct {
	ID                         string                     `json:"id,omitempty"`
	Name                       string                     `json:"name,omitempty"`
	Description                string                     `json:"description,omitempty"`
	VlanID                     int                        `json:"vlanId,omitempty"`
	StaticNetworkConfiguration StaticNetworkConfiguration `json:"staticNetworkConfiguration,omitempty"`
	DestinationIPAddress       string                     `json:"destinationIpAddress,omitempty"`
	Static                     bool                       `json:"static,omitempty"`
	Type                       string                     `json:"type,omitempty"`
}

// Options defines the options.
type Options struct {
	ID           string                `json:"id,omitempty"`
	Name         string                `json:"name,omitempty"`
	Dependencies []DependenciesDetails `json:"dependencies,omitempty"`
	Attributes   map[string]string     `json:"attributes,omitempty"`
}

// Parameters defines the parameters details.
type Parameters struct {
	ID               string                `json:"id,omitempty"`
	Value            string                `json:"value,omitempty"`
	DisplayName      string                `json:"displayName,omitempty"`
	Type             string                `json:"type,omitempty"`
	ToolTip          string                `json:"toolTip,omitempty"`
	Required         bool                  `json:"required,omitempty"`
	HideFromTemplate bool                  `json:"hideFromTemplate,omitempty"`
	DeviceType       string                `json:"deviceType,omitempty"`
	Dependencies     []DependenciesDetails `json:"dependencies,omitempty"`
	Group            string                `json:"group,omitempty"`
	ReadOnly         bool                  `json:"readOnly,omitempty"`
	Generated        bool                  `json:"generated,omitempty"`
	InfoIcon         bool                  `json:"infoIcon,omitempty"`
	Step             int                   `json:"step,omitempty"`
	MaxLength        int                   `json:"maxLength,omitempty"`
	Min              int                   `json:"min,omitempty"`
	Max              int                   `json:"max,omitempty"`
	Networks         []Networks            `json:"networks,omitempty"`
	Options          []Options             `json:"options,omitempty"`
	OptionsSortable  bool                  `json:"optionsSortable,omitempty"`
}

// Categories defines the details of a category.
type Categories struct {
	ID          string       `json:"id,omitempty"`
	DisplayName string       `json:"displayName,omitempty"`
	DeviceType  string       `json:"deviceType,omitempty"`
	Parameters  []Parameters `json:"parameters,omitempty"`
}

// TemplateDetailsFilter defines struct for nodepools
type TemplateDetailsFilter struct {
	TemplateDetails []TemplateDetails `json:"serviceTemplate"`
}

// IPListNoRole model for Peer Mdm
type IPListNoRole struct {
	IP string `json:"ip,omitempty"`
}
