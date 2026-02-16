# Checkpoint (snapshot) check for Hyper-V VMs
# VMs with existing checkpoints should be noted as the checkpoint data will not be migrated

package io.konveyor.forklift.hyperv

import rego.v1

default has_checkpoint := false

has_checkpoint if {
	input.hasCheckpoint == true
}

concerns contains flag if {
	has_checkpoint
	flag := {
		"id": "hyperv.checkpoint.detected",
		"category": "Information",
		"label": "VM checkpoint detected",
		"assessment": "The VM has existing checkpoints/snapshots. Checkpoint data will not be migrated. The VM will be migrated from its current state.",
	}
}
