/*
Copyright 2019 Red Hat Inc.

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

// Package v1alpha1 contains API Schema definitions for the migration v1alpha1 API group
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/kubev2v/forklift/pkg/apis
// +k8s:defaulter-gen=TypeMeta
// +groupName=forklift.konveyor.io
package v1beta1

import libcnd "github.com/kubev2v/forklift/pkg/lib/condition"

// GlanceSource represents an image of which the source is Glance, to be used in storage mapping
const GlanceSource = "glance"

// Conditions
const (
	ConditionExecuting = "Executing"
	ConditionRunning   = "Running"
	ConditionPending   = "Pending"
	ConditionCanceled  = "Canceled"
	ConditionSucceeded = "Succeeded"
	ConditionFailed    = "Failed"
	ConditionBlocked   = "Blocked"
	ConditionDeleted   = "Deleted"
)

// Condition categories
const (
	CategoryRequired = libcnd.Required
	CategoryAdvisory = libcnd.Advisory
	CategoryCritical = libcnd.Critical
	CategoryError    = libcnd.Error
	CategoryWarn     = libcnd.Warn
)

// VM Phases:
//
// Common phases.
const (
	PhaseStarted   = "Started"
	PhasePreHook   = "PreHook"
	PhasePostHook  = "PostHook"
	PhaseCompleted = "Completed"
)

// Warm and cold phases.
const (
	PhaseAddCheckpoint                     = "AddCheckpoint"
	PhaseAddFinalCheckpoint                = "AddFinalCheckpoint"
	PhaseAllocateDisks                     = "AllocateDisks"
	PhaseConvertGuest                      = "ConvertGuest"
	PhaseConvertOpenstackSnapshot          = "ConvertOpenstackSnapshot"
	PhaseCopyDisks                         = "CopyDisks"
	PhaseCopyDisksVirtV2V                  = "CopyDisksVirtV2V"
	PhaseCopyingPaused                     = "CopyingPaused"
	PhaseCreateDataVolumes                 = "CreateDataVolumes"
	PhaseCreateFinalSnapshot               = "CreateFinalSnapshot"
	PhaseCreateGuestConversionPod          = "CreateGuestConversionPod"
	PhaseCreateInitialSnapshot             = "CreateInitialSnapshot"
	PhaseCreateSnapshot                    = "CreateSnapshot"
	PhaseCreateVM                          = "CreateVM"
	PhaseFinalize                          = "Finalize"
	PhasePreflightInspection               = "PreflightInspection"
	PhasePowerOffSource                    = "PowerOffSource"
	PhaseRemoveFinalSnapshot               = "RemoveFinalSnapshot"
	PhaseRemovePenultimateSnapshot         = "RemovePenultimateSnapshot"
	PhaseRemovePreviousSnapshot            = "RemovePreviousSnapshot"
	PhaseStoreInitialSnapshotDeltas        = "StoreInitialSnapshotDeltas"
	PhaseStorePowerState                   = "StorePowerState"
	PhaseStoreSnapshotDeltas               = "StoreSnapshotDeltas"
	PhaseWaitForFinalSnapshot              = "WaitForFinalSnapshot"
	PhaseWaitForFinalSnapshotRemoval       = "WaitForFinalSnapshotRemoval"
	PhaseWaitForInitialSnapshot            = "WaitForInitialSnapshot"
	PhaseWaitForPenultimateSnapshotRemoval = "WaitForPenultimateSnapshotRemoval"
	PhaseWaitForPowerOff                   = "WaitForPowerOff"
	PhaseWaitForPreviousSnapshotRemoval    = "WaitForPreviousSnapshotRemoval"
	PhaseWaitForSnapshot                   = "WaitForSnapshot"
)

// Step/task phases.
const (
	StepStarted   = "Started"
	StepPending   = "Pending"
	StepRunning   = "Running"
	StepCompleted = "Completed"
)

// Annotations.
const (
	AnnDiskSource = "forklift.konveyor.io/disk-source"
	AnnSource     = "forklift.konveyor.io/source"
)
