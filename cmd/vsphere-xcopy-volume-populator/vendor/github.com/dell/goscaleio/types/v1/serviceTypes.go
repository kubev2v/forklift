// Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package goscaleio

// ServiceFailedResponse represents the response when a service fails.
type ServiceFailedResponse struct {
	DetailMessage string     `json:"detailMessage,omitempty"`
	Status        int        `json:"status,omitempty"`
	StatusCode    int        `json:"statusCode,omitempty"`
	Timestamp     string     `json:"timestamp,omitempty"`
	Error         string     `json:"error,omitempty"`
	Path          string     `json:"path,omitempty"`
	Messages      []Messages `json:"messages,omitempty"`
}

// DeploymentPayload represents the payload for deploying a service.
type DeploymentPayload struct {
	DeploymentName        string          `json:"deploymentName,omitempty"`
	DeploymentDescription string          `json:"deploymentDescription,omitempty"`
	ServiceTemplate       TemplateDetails `json:"serviceTemplate,omitempty"`
	UpdateServerFirmware  bool            `json:"updateServerFirmware,omitempty"`
	FirmwareRepositoryID  string          `json:"firmwareRepositoryId,omitempty"`
	Status                string          `json:"status,omitempty"`
}

// ServiceResponse represents the response from a service operation.
type ServiceResponse struct {
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
	ServiceTemplate              TemplateDetails              `json:"serviceTemplate"`
	ScheduleDate                 string                       `json:"scheduleDate,omitempty"`
	Status                       string                       `json:"status,omitempty"`
	Compliant                    bool                         `json:"compliant,omitempty"`
	DeploymentDevice             []DeploymentDevice           `json:"deploymentDevice,omitempty"`
	Vms                          []Vms                        `json:"vms,omitempty"`
	UpdateServerFirmware         bool                         `json:"updateServerFirmware,omitempty"`
	UseDefaultCatalog            bool                         `json:"useDefaultCatalog,omitempty"`
	FirmwareRepository           FirmwareRepository           `json:"firmwareRepository,omitempty"`
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
	OverallDeviceHealth          string                       `json:"overallDeviceHealth,omitempty"`
	Vds                          bool                         `json:"vds,omitempty"`
	ScaleUp                      bool                         `json:"scaleUp,omitempty"`
	LifecycleMode                bool                         `json:"lifecycleMode,omitempty"`
	CanMigratevCLSVMs            bool                         `json:"canMigratevCLSVMs,omitempty"`
	TemplateValid                bool                         `json:"templateValid,omitempty"`
	ConfigurationChange          bool                         `json:"configurationChange,omitempty"`
	DetailMessage                string                       `json:"detailMessage,omitempty"`
	Timestamp                    string                       `json:"timestamp,omitempty"`
	Error                        string                       `json:"error,omitempty"`
	Path                         string                       `json:"path,omitempty"`
	Messages                     []Messages                   `json:"messages,omitempty"`
	StatusCode                   int                          `json:"statusCode,omitempty"`
}

// ComplianceReport defines the compliance report object for a service.
type ComplianceReport struct {
	ServiceTag                 string                       `json:"serviceTag,omitempty"`
	IPAddress                  string                       `json:"ipAddress,omitempty"`
	FirmwareRepositoryName     string                       `json:"firmwareRepositoryName,omitempty"`
	ComplianceReportComponents []ComplianceReportComponents `json:"firmwareComplianceReportComponents,omitempty"`
	Compliant                  bool                         `json:"compliant,omitempty"`
	DeviceType                 string                       `json:"deviceType,omitempty"`
	Model                      string                       `json:"model,omitempty"`
	Available                  bool                         `json:"available,omitempty"`
	ManagedState               string                       `json:"managedState,omitempty"`
	EmbeddedReport             bool                         `json:"embeddedReport,omitempty"`
	DeviceState                string                       `json:"deviceState,omitempty"`
	ID                         string                       `json:"id,omitempty"`
	HostName                   string                       `json:"hostname,omitempty"`
	CanUpdate                  bool                         `json:"canUpdate,omitempty"`
}

// ComplianceReportComponents defines the components in the compliance report.
type ComplianceReportComponents struct {
	ID              string                               `json:"id,omitempty"`
	Name            string                               `json:"name,omitempty"`
	CurrentVersion  ComplianceReportComponentVersionInfo `json:"currentVersion,omitempty"`
	TargetVersion   ComplianceReportComponentVersionInfo `json:"targetVersion,omitempty"`
	Vendor          string                               `json:"vendor,omitempty"`
	OperatingSystem string                               `json:"operatingSystem,omitempty"`
	Compliant       bool                                 `json:"compliant,omitempty"`
	Software        bool                                 `json:"software,omitempty"`
	RPM             bool                                 `json:"rpm,omitempty"`
	Oscapable       bool                                 `json:"oscompatible,omitempty"`
}

// ComplianceReportComponentVersionInfo defines the version info in the compliance report component.
type ComplianceReportComponentVersionInfo struct {
	ID                 string `json:"id,omitempty"`
	FirmwareName       string `json:"firmwareName,omitempty"`
	FirmwareType       string `json:"firmwareType,omitempty"`
	FirmwareVersion    string `json:"firmwareVersion,omitempty"`
	FirmwareLastUpdate string `json:"firmwareLastUpdate,omitempty"`
	FirmwareLevel      string `json:"firmwareLevel,omitempty"`
}
