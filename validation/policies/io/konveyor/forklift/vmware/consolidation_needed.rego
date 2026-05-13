package io.konveyor.forklift.vmware

import rego.v1

consolidation_needed if {
	input.consolidationNeeded == true
}

concerns contains flag if {
	consolidation_needed
	flag := {
		"id": "vmware.consolidation_needed",
		"category": "Information",
		"label": "Snapshot consolidation required",
		"assessment": "VM has snapshots that require consolidation. This may cause delays between precopies."
	}
}
