package io.konveyor.forklift.vmware

import rego.v1

has_snapshot if {
	input.snapshot.kind == "VirtualMachineSnapshot"
}

concerns contains flag if {
	has_snapshot
	flag := {
		"id": "vmware.snapshot.detected",
		"category": "Information",
		"label": "VM snapshot detected",
		"assessment": "Online snapshots are not currently supported by OpenShift Virtualization. VM will be migrated with current snapshot.",
	}
}
