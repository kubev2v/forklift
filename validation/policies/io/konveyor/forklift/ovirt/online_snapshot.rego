package io.konveyor.forklift.ovirt

import rego.v1

online_snapshots contains i if {
	some i
	input.snapshots[i].persistMemory
}

concerns contains flag if {
	count(online_snapshots) > 0
	flag := {
		"id": "ovirt.snapshot.online_memory.detected",
		"category": "Warning",
		"label": "Online (memory) snapshot detected",
		"assessment": "The VM has a snapshot that contains a memory copy. Online snapshots such as this are not curently supported by OpenShift Virtualization.",
	}
}
