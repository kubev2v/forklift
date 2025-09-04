package io.konveyor.forklift.ovirt

import rego.v1

default has_cpu_affinity := false

has_cpu_affinity if {
	count(input.cpuAffinity) != 0
}

concerns contains flag if {
	has_cpu_affinity
	flag := {
		"id": "ovirt.cpu.tuning.detected",
		"category": "Warning",
		"label": "CPU tuning detected",
		"assessment": "CPU tuning other than 1 vCPU - 1 pCPU is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have this feature in the target environment.",
	}
}
