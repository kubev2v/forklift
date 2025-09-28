package io.konveyor.forklift.ovirt

import rego.v1

default has_ballooned_memory := false

has_ballooned_memory := value if {
	value := input.balloonedMemory
}

concerns contains flag if {
	has_ballooned_memory
	flag := {
		"id": "ovirt.memory.ballooning.enabled",
		"category": "Information",
		"label": "VM has memory ballooning enabled",
		"assessment": "The VM has memory ballooning enabled. This is not currently supported by OpenShift Virtualization.",
	}
}
