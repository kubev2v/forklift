// Copyright (C) 2026 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ValidationPhase describes the controller lifecycle, independently of the
// result of the validation checks.
// +kubebuilder:validation:Enum=Pending;Running;Completed;Cancelled;Error
type ValidationPhase string

const (
	ValidationPhasePending   ValidationPhase = "Pending"
	ValidationPhaseRunning   ValidationPhase = "Running"
	ValidationPhaseCompleted ValidationPhase = "Completed"
	ValidationPhaseCancelled ValidationPhase = "Cancelled"
	ValidationPhaseError     ValidationPhase = "Error"
)

// ValidationVerdict is the interpretation of the observed validation results.
// It is deliberately not a product support statement.
// +kubebuilder:validation:Enum=Unknown;Valid;ValidWithWarnings;Invalid;Inconclusive
type ValidationVerdict string

const (
	ValidationVerdictUnknown           ValidationVerdict = "Unknown"
	ValidationVerdictValid             ValidationVerdict = "Valid"
	ValidationVerdictValidWithWarnings ValidationVerdict = "ValidWithWarnings"
	ValidationVerdictInvalid           ValidationVerdict = "Invalid"
	ValidationVerdictInconclusive      ValidationVerdict = "Inconclusive"
)

// ValidationOutcome is the state of one check execution or workflow step.
// +kubebuilder:validation:Enum=Pending;Running;Passed;Failed;TimedOut;Error;Skipped;Cancelled
type ValidationOutcome string

const (
	ValidationOutcomePending   ValidationOutcome = "Pending"
	ValidationOutcomeRunning   ValidationOutcome = "Running"
	ValidationOutcomePassed    ValidationOutcome = "Passed"
	ValidationOutcomeFailed    ValidationOutcome = "Failed"
	ValidationOutcomeTimedOut  ValidationOutcome = "TimedOut"
	ValidationOutcomeError     ValidationOutcome = "Error"
	ValidationOutcomeSkipped   ValidationOutcome = "Skipped"
	ValidationOutcomeCancelled ValidationOutcome = "Cancelled"
)

// ValidationSeverity determines whether an unsuccessful execution affects the
// aggregate verdict.
// +kubebuilder:validation:Enum=Required;Advisory
type ValidationSeverity string

const (
	ValidationSeverityRequired ValidationSeverity = "Required"
	ValidationSeverityAdvisory ValidationSeverity = "Advisory"
)

// TargetIdentity identifies the target for consumers. It is descriptive only
// and does not affect how the controller connects to the target.
type TargetIdentity struct {
	// ID is a caller-provided stable target identifier.
	ID string `json:"id,omitempty"`
	// DisplayName is a human-readable target name.
	DisplayName string `json:"displayName,omitempty"`
}

// CredentialSecretReference describes a same-namespace Secret containing the
// target bearer token and optional CA certificate.
type CredentialSecretReference struct {
	// +kubebuilder:validation:MinLength=1
	// Name is the same-namespace Secret name.
	Name string `json:"name"`
	// +kubebuilder:default:=token
	// TokenKey identifies the bearer-token entry in the Secret.
	TokenKey string `json:"tokenKey,omitempty"`
	// CAKey identifies the CA entry in the Secret. Omit it or set it to an empty
	// string to use the container system trust store instead.
	CAKey string `json:"caKey,omitempty"`
}

// ValidationToolDownloads configures optional target-specific CLI download
// endpoints. Explicit URLs avoid assuming an api.<domain> API hostname or
// enabled console and virtctl download Routes.
type ValidationToolDownloads struct {
	// ClusterDomain is the target ingress domain used only when an explicit
	// download URL is not supplied.
	// +kubebuilder:validation:MaxLength=253
	ClusterDomain string `json:"clusterDomain,omitempty"`
	// OCURL is the HTTPS archive URL for the target-compatible oc client.
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^https://`
	OCURL string `json:"ocURL,omitempty"`
	// VirtctlURL is the HTTPS archive URL for target-compatible virtctl.
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^https://`
	VirtctlURL string `json:"virtctlURL,omitempty"`
}

// ValidationTarget describes the remote Kubernetes API being validated.
type ValidationTarget struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^https://`
	// URL is the absolute HTTPS API endpoint for the target cluster.
	URL string `json:"url"`
	// CredentialsSecret supplies target authentication and optional CA data.
	CredentialsSecret CredentialSecretReference `json:"credentialsSecret"`
	// InsecureSkipTLS disables target certificate verification. Use only for
	// short-lived troubleshooting; prefer CAKey or system trust.
	InsecureSkipTLS bool `json:"insecureSkipTLS,omitempty"`
	// Identity is descriptive metadata for users and consumers.
	Identity TargetIdentity `json:"identity,omitempty"`
	// ToolDownloads optionally overrides target CLI discovery.
	ToolDownloads *ValidationToolDownloads `json:"toolDownloads,omitempty"`
}

// WorkloadDataSourceReference is a namespaced CDI DataSource used to create a
// canary VM. APIGroup and Kind make the reference self-describing for clients.
type WorkloadDataSourceReference struct {
	// +kubebuilder:default:=cdi.kubevirt.io
	// APIGroup is the API group of the referenced data source.
	APIGroup string `json:"apiGroup,omitempty"`
	// +kubebuilder:default:=DataSource
	// Kind is the kind of the referenced data source.
	Kind string `json:"kind,omitempty"`
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// Namespace contains the data source.
	Namespace string `json:"namespace"`
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// Name is the data source name.
	Name string `json:"name"`
}

// ValidationWorkload describes target resources used by profile checks.
type ValidationWorkload struct {
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// Namespace is where validation canary resources are created on the target.
	Namespace string `json:"namespace"`
	// DataSource is used to create the validation canary VM.
	DataSource WorkloadDataSourceReference `json:"dataSource"`
}

// +kubebuilder:validation:XValidation:rule="self.executionTimeoutSeconds <= self.suiteTimeoutSeconds",message="executionTimeoutSeconds must not exceed suiteTimeoutSeconds"
type ValidationRun struct {
	// ID is an optional caller-chosen execution identity. Changing it starts a
	// new run without deleting and recreating this resource.
	// +kubebuilder:default:=initial
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	ID string `json:"id,omitempty"`
	// Profile is a versioned validation suite. The current controller supports
	// basic-v1 and basic-all-nodes-v1, which verify authentication, CNV
	// installation, validation namespace quota, and canary VM workflows.
	// +kubebuilder:validation:Pattern=`^[a-z][a-z0-9-]*-v[0-9]+$`
	Profile string `json:"profile"`
	// SuiteTimeoutSeconds limits the entire validation Job, including all
	// profile executions.
	// +kubebuilder:default:=900
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=86400
	SuiteTimeoutSeconds int64 `json:"suiteTimeoutSeconds,omitempty"`
	// ExecutionTimeoutSeconds limits one profile execution, such as one canary
	// VM on one target node.
	// +kubebuilder:default:=180
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3600
	ExecutionTimeoutSeconds int64 `json:"executionTimeoutSeconds,omitempty"`
	// CleanupPolicy controls retention of target workload resources created by
	// validation checks. Controller-owned Job and report resources are unaffected.
	// +kubebuilder:default:=Always
	// +kubebuilder:validation:Enum=Always;OnFailure;Never
	CleanupPolicy string `json:"cleanupPolicy,omitempty"`
	// MaxConcurrentExecutions limits concurrent profile execution units. When
	// omitted, the executor uses its own default.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=64
	MaxConcurrentExecutions int32 `json:"maxConcurrentExecutions,omitempty"`
	// QueueTimeoutSeconds limits how long this request may wait for the
	// target-scoped lease. Zero uses the suite timeout.
	// +kubebuilder:default:=0
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=86400
	QueueTimeoutSeconds int64 `json:"queueTimeoutSeconds,omitempty"`
	// Cancel requests one-way cancellation of this validation. A cancelled
	// validation cannot be resumed; create a new resource to run it again.
	// +kubebuilder:default:=false
	Cancel bool `json:"cancel,omitempty"`
}

type VirtualizationValidationSpec struct {
	// Target is the remote cluster connection request.
	Target ValidationTarget `json:"target"`
	// Workload defines target resources required by the selected profile.
	Workload ValidationWorkload `json:"workload"`
	// Run configures this immutable validation execution.
	Run ValidationRun `json:"run"`
}

// VirtualizationValidationSummary contains CTRF result counts.
type VirtualizationValidationSummary struct {
	// Total is the number of executions in the report.
	Total int32 `json:"total,omitempty"`
	// Passed is the number of passed executions.
	Passed int32 `json:"passed,omitempty"`
	// Failed is the number of failed executions.
	Failed int32 `json:"failed,omitempty"`
	// Pending is the number of executions not yet complete.
	Pending int32 `json:"pending,omitempty"`
	// Skipped is the number of skipped executions.
	Skipped int32 `json:"skipped,omitempty"`
	// Other is the number of CTRF executions with an unclassified result.
	Other int32 `json:"other,omitempty"`
}

// ValidationSubjectReference identifies the target object or scope for an
// execution without making consumers parse execution IDs or runner paths.
type ValidationSubjectReference struct {
	// APIGroup is the subject API group when applicable.
	APIGroup string `json:"apiGroup,omitempty"`
	// Kind is the subject kind.
	Kind string `json:"kind"`
	// Namespace is the subject namespace when applicable.
	Namespace string `json:"namespace,omitempty"`
	// Name is the subject name.
	Name string `json:"name"`
}

// VirtualizationValidationExecution is one observed execution of a logical
// validation check. Consumers group related executions by CheckID.
type VirtualizationValidationExecution struct {
// CheckID is a stable, profile-independent identifier defined by Forklift.
	// +kubebuilder:validation:MaxLength=128
	CheckID string `json:"checkId"`
	// ExecutionID distinguishes repeated executions of the same check.
	// +kubebuilder:validation:MaxLength=128
	ExecutionID string `json:"executionId,omitempty"`
	// +kubebuilder:validation:MaxLength=256
	// Name is a human-readable check name.
	Name string `json:"name"`
	// Outcome is the observed result state.
	Outcome ValidationOutcome `json:"outcome"`
	// Severity controls this execution's contribution to the verdict.
	Severity ValidationSeverity `json:"severity"`
	// Reason is a stable machine-readable outcome reason, such as
	// ExecutionTimedOut or SnapshotTimedOut.
	// +kubebuilder:validation:MaxLength=128
	Reason string `json:"reason,omitempty"`
	// Subject identifies the target scope of this execution.
	Subject *ValidationSubjectReference `json:"subject,omitempty"`
	// DurationMilliseconds is the execution duration reported by CTRF.
	DurationMilliseconds int64 `json:"durationMilliseconds,omitempty"`
	// StartedAt is when this execution started, when reported by CTRF.
	StartedAt *metav1.Time `json:"startedAt,omitempty"`
	// CompletedAt is when this execution completed, when reported by CTRF.
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`
	// +kubebuilder:validation:MaxLength=512
	// Message is a bounded human-readable execution result.
	Message string `json:"message,omitempty"`
	// +kubebuilder:validation:MaxProperties=16
	// Parameters contains bounded execution inputs and observed properties.
	Parameters map[string]string `json:"parameters,omitempty"`
	// ParametersTruncated reports that some parameter information was omitted.
	ParametersTruncated bool `json:"parametersTruncated,omitempty"`
	// RetryCount is the number of retries reported by CTRF.
	RetryCount int32 `json:"retryCount,omitempty"`
	// Flaky is reported by CTRF when retries produced different outcomes.
	Flaky bool `json:"flaky,omitempty"`
	// Representative reports that this execution is retained as an exemplar for
	// otherwise collapsed executions.
	Representative bool `json:"representative,omitempty"`
	// +kubebuilder:validation:MaxItems=16
	// Steps is the bounded workflow-step projection.
	Steps []ValidationStep `json:"steps,omitempty"`
	// StepsTruncated reports that some workflow steps were omitted.
	StepsTruncated bool `json:"stepsTruncated,omitempty"`
}

// ValidationStep is one bounded CTRF step observed within an execution.
type ValidationStep struct {
	// +kubebuilder:validation:MaxLength=256
	// Name is the human-readable workflow step name.
	Name string `json:"name"`
	// Outcome is the observed step state.
	Outcome ValidationOutcome `json:"outcome"`
	// +kubebuilder:validation:MaxLength=128
	// Reason is a machine-readable step outcome reason.
	Reason string `json:"reason,omitempty"`
	// +kubebuilder:validation:MaxProperties=16
	// Extra contains bounded runner-provided step metadata.
	Extra map[string]string `json:"extra,omitempty"`
	// ExtraTruncated reports that some metadata was omitted.
	ExtraTruncated bool `json:"extraTruncated,omitempty"`
}

// ValidationReportStatus identifies the CTRF snapshot from which status was
// projected. The full report remains in status.artifact.
type ValidationReportStatus struct {
	// SpecVersion is the CTRF specification version.
	SpecVersion string `json:"specVersion,omitempty"`
	// Digest is the SHA-256 digest of the observed artifact content.
	Digest string `json:"digest,omitempty"`
	// Reduction describes information removed from the retained CTRF artifact.
	Reduction ValidationReportReduction `json:"reduction,omitempty"`
}

// ValidationReportReduction describes a monotonic, loss-aware report reduction.
// Counts always refer to executions omitted from the detailed list.
type ValidationReportReduction struct {
	// Level is the highest reduction rung applied, from 0 (none) through 7
	// (summary only).
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=7
	Level int32 `json:"level,omitempty"`
	// TotalExecutions is the total number of executions before reduction.
	TotalExecutions int32 `json:"totalExecutions,omitempty"`
	// PendingCollapsed is the number of pending executions omitted from detail.
	PendingCollapsed int32 `json:"pendingCollapsed,omitempty"`
	// PassedCollapsed is the number of passed executions omitted from detail.
	PassedCollapsed int32 `json:"passedCollapsed,omitempty"`
	// RequiredFailuresOmitted is the number of required failures omitted from detail.
	RequiredFailuresOmitted int32 `json:"requiredFailuresOmitted,omitempty"`
	// AdvisoryFailuresOmitted is the number of advisory failures omitted from detail.
	AdvisoryFailuresOmitted int32 `json:"advisoryFailuresOmitted,omitempty"`
	// RequiredInconclusiveOmitted is the number of required timed out, skipped,
	// cancelled, or errored executions omitted from detail.
	RequiredInconclusiveOmitted int32 `json:"requiredInconclusiveOmitted,omitempty"`
	// AdvisoryInconclusiveOmitted is the number of advisory timed out, skipped,
	// cancelled, or errored executions omitted from detail.
	AdvisoryInconclusiveOmitted int32 `json:"advisoryInconclusiveOmitted,omitempty"`
}

// ValidationArtifactReference locates the CTRF artifact retained by the controller.
type ValidationArtifactReference struct {
	// APIVersion is the artifact API version.
	APIVersion string `json:"apiVersion"`
	// Name is the artifact resource name.
	Name string `json:"name"`
	// Namespace is the artifact resource namespace.
	Namespace string `json:"namespace"`
	// Kind is the artifact resource kind.
	Kind string `json:"kind"`
	// Key is the ConfigMap data key containing the CTRF document.
	Key string `json:"key"`
	// Truncated reports whether the retained artifact was reduced.
	Truncated bool `json:"truncated,omitempty"`
}

// VirtualizationValidationStatus is the controller-owned observed state.
type VirtualizationValidationStatus struct {
	// Phase is the controller lifecycle state.
	Phase ValidationPhase `json:"phase,omitempty"`
	// Verdict is the aggregate validation interpretation.
	Verdict ValidationVerdict `json:"verdict,omitempty"`
	// ObservedGeneration is the generation used to calculate this status.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// ObservedInputHash identifies the immutable run inputs.
	ObservedInputHash string `json:"observedInputHash,omitempty"`
	// Image is the validator image used for the run.
	Image string `json:"image,omitempty"`
	// JobName is the controller-owned validator Job.
	JobName string `json:"jobName,omitempty"`
	// ObservedReportResourceVersion is the last consumed report snapshot.
	ObservedReportResourceVersion string `json:"observedReportResourceVersion,omitempty"`
	// ReportComplete reports whether CTRF has no pending executions.
	ReportComplete bool `json:"reportComplete,omitempty"`
	// Report identifies the observed CTRF document.
	Report *ValidationReportStatus `json:"report,omitempty"`
	// Artifact locates the retained CTRF document.
	Artifact *ValidationArtifactReference `json:"artifact,omitempty"`
	// Reduction describes information removed from the status projection.
	Reduction ValidationReportReduction `json:"reduction,omitempty"`
	// StartedAt is when the controller created the validator Job.
	StartedAt *metav1.Time `json:"startedAt,omitempty"`
	// QueuedAt is when the request first began waiting for the target lease.
	QueuedAt *metav1.Time `json:"queuedAt,omitempty"`
	// CompletedAt is when the lifecycle entered a terminal phase.
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`
	// Summary contains aggregate CTRF counters.
	Summary VirtualizationValidationSummary `json:"summary,omitempty"`
	// +kubebuilder:validation:MaxItems=128
	// Executions is the bounded projection of CTRF check executions.
	Executions []VirtualizationValidationExecution `json:"executions,omitempty"`
	// +listType=map
	// +listMapKey=type
	// Conditions contains standard controller conditions keyed by type.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vv
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.spec.target.url`,description="Target API endpoint"
// +kubebuilder:printcolumn:name="Profile",type=string,JSONPath=`.spec.run.profile`,description="Validation profile"
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`,description="Validation phase"
// +kubebuilder:printcolumn:name="Verdict",type=string,JSONPath=`.status.verdict`,description="Validation verdict"
// +kubebuilder:printcolumn:name="Passed",type=integer,JSONPath=`.status.summary.passed`,description="Checks passed"
// +kubebuilder:printcolumn:name="Failed",type=integer,JSONPath=`.status.summary.failed`,description="Checks failed"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="Time since validation was created"
// VirtualizationValidation requests one immutable remote-cluster validation.
// Invalid configuration enters Error and must be deleted and recreated after it is corrected.
type VirtualizationValidation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf || (self.target == oldSelf.target && self.workload == oldSelf.workload && self.run.profile == oldSelf.run.profile && self.run.suiteTimeoutSeconds == oldSelf.run.suiteTimeoutSeconds && self.run.executionTimeoutSeconds == oldSelf.run.executionTimeoutSeconds && self.run.cleanupPolicy == oldSelf.run.cleanupPolicy && has(self.run.maxConcurrentExecutions) == has(oldSelf.run.maxConcurrentExecutions) && (!has(self.run.maxConcurrentExecutions) || self.run.maxConcurrentExecutions == oldSelf.run.maxConcurrentExecutions) && has(self.run.queueTimeoutSeconds) == has(oldSelf.run.queueTimeoutSeconds) && (!has(self.run.queueTimeoutSeconds) || self.run.queueTimeoutSeconds == oldSelf.run.queueTimeoutSeconds) && ((has(self.run.id) != has(oldSelf.run.id)) || (has(self.run.id) && self.run.id != oldSelf.run.id) || ((!has(oldSelf.run.cancel) || !oldSelf.run.cancel) && self.run.cancel)))",message="VirtualizationValidation spec is immutable except that run.id may change or run.cancel may be set to true"
	Spec   VirtualizationValidationSpec   `json:"spec"`
	Status VirtualizationValidationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type VirtualizationValidationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualizationValidation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualizationValidation{}, &VirtualizationValidationList{})
}
