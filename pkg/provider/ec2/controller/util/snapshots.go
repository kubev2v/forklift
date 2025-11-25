package util

import (
	"fmt"
	"strings"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

// Constants for managing EBS snapshot metadata in migration pipeline steps.
//
// These constants are used to store and retrieve snapshot information in the
// migration plan's step annotations. Annotations provide a way to persist
// snapshot IDs across reconciliation loops without requiring additional CRDs.
const (
	// CreateSnapshotsStepName is the pipeline step name where snapshot information is stored.
	//
	// This constant identifies which step in the VM's migration pipeline contains
	// snapshot-related metadata. The step is created during Pipeline() and accessed
	// throughout the migration to store and retrieve snapshot IDs.
	//
	// Usage:
	//   - Used by StoreSnapshotIDs() to find the step for storing snapshot data
	//   - Used by GetSnapshotIDs() to find the step for retrieving snapshot data
	//   - Used by GetSnapshotStep() to locate the step for status checks
	//
	// The step name must match the constant defined in migrator/phases.go.
	CreateSnapshotsStepName = "CreateSnapshots"

	// SnapshotAnnotationPrefix is the prefix for snapshot ID annotations in step metadata.
	//
	// Snapshot IDs are stored as annotations in the CreateSnapshots step using the format:
	//   snapshot-{volumeID} = {snapshotID}
	//
	// Examples:
	//   "snapshot-vol-0123456789abcdef0" = "snap-0fedcba9876543210"
	//   "snapshot-vol-0abcdef123456789" = "snap-0123456789abcdef0"
	//
	// This prefix allows:
	//   - Multiple snapshots to be stored in the same step annotations
	//   - Easy filtering to find all snapshot-related annotations
	//   - Direct mapping from volume ID to snapshot ID
	//
	// Usage:
	//   - StoreSnapshotIDs() adds annotations with this prefix
	//   - GetSnapshotIDs() searches for annotations starting with this prefix
	//   - removeSnapshots() uses these annotations to find snapshots to delete
	//
	// The prefix must be unique within step annotations to avoid conflicts with
	// other metadata stored in the same step (like progress indicators).
	SnapshotAnnotationPrefix = "snapshot-"
)

// GetSnapshotIDs retrieves EBS snapshot IDs stored in the VM migration step annotations.
// Returns a map where keys are EBS volume IDs and values are their corresponding snapshot IDs.
// Returns an empty map if no snapshots are found, or an error if the step doesn't exist.
func GetSnapshotIDs(vm *planapi.VMStatus, log logging.LevelLogger) (map[string]string, error) {
	log.V(1).Info("Getting snapshot IDs", "vm", vm.Name)

	step, found := vm.FindStep(CreateSnapshotsStepName)
	if !found {
		log.Error(nil, "CreateSnapshots step not found", "vm", vm.Name)
		return nil, fmt.Errorf("CreateSnapshots step not found for VM %s", vm.Name)
	}

	log.V(1).Info("Found CreateSnapshots step",
		"vm", vm.Name,
		"hasAnnotations", step.Annotations != nil,
		"annotationCount", len(step.Annotations))

	result := make(map[string]string)
	if step.Annotations == nil {
		log.Info("Step has no annotations", "vm", vm.Name)
		return result, nil
	}

	for key, value := range step.Annotations {
		if strings.HasPrefix(key, SnapshotAnnotationPrefix) {
			volumeID := strings.TrimPrefix(key, SnapshotAnnotationPrefix)
			result[volumeID] = value
			log.V(1).Info("Found snapshot annotation",
				"vm", vm.Name,
				"volumeID", volumeID,
				"snapshotID", value)
		}
	}

	return result, nil
}

// StoreSnapshotIDs saves EBS snapshot IDs to the VM migration step annotations.
// Creates one annotation per volume using the format: snapshot-{volumeID} = snapshotID.
// The volumeIDs and snapshotIDs slices must have the same length and order.
// Also adds metadata annotations about sparse copy and thin provisioning.
func StoreSnapshotIDs(vm *planapi.VMStatus, volumeIDs, snapshotIDs []string, log logging.LevelLogger) error {
	step, found := vm.FindStep(CreateSnapshotsStepName)
	if !found {
		return fmt.Errorf("CreateSnapshots step not found for VM %s", vm.Name)
	}

	if step.Annotations == nil {
		step.Annotations = make(map[string]string)
	}

	for i, volumeID := range volumeIDs {
		key := fmt.Sprintf("%s%s", SnapshotAnnotationPrefix, volumeID)
		step.Annotations[key] = snapshotIDs[i]
		log.V(1).Info("Stored snapshot ID in step annotation",
			"vm", vm.Name,
			"volumeID", volumeID,
			"snapshotID", snapshotIDs[i],
			"key", key)
	}

	step.Annotations["sparse-copy"] = "true"
	step.Annotations["thin-provision-note"] = "PVCs sized to match source volumes but sparse copy and thin provisioning mean actual storage used is much less"

	return nil
}

// GetSnapshotStep retrieves the CreateSnapshots pipeline step for the given VM.
// Returns an error if the step is not found in the VM's migration pipeline.
func GetSnapshotStep(vm *planapi.VMStatus) (*planapi.Step, error) {
	step, found := vm.FindStep(CreateSnapshotsStepName)
	if !found {
		return nil, fmt.Errorf("CreateSnapshots step not found in pipeline")
	}
	return step, nil
}
