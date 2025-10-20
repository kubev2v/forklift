package io.konveyor.forklift.ovirt

import rego.v1

default has_numa_affinity := false

has_numa_affinity if {
	count(input.numaNodeAffinity) != 0
}

concerns contains flag if {
	has_numa_affinity
	flag := {
		"id": "ovirt.numa.tuning.detected",
		"category": "Warning",
		"label": "NUMA tuning detected",
		"assessment": "NUMA tuning is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have this NUMA mapping in the target environment.",
	}
}
