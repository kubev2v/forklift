package io.konveyor.forklift.ova

import rego.v1

has_cpu_affinity if {
	count(input.cpuAffinity) != 0
}

concerns contains flag if {
	has_cpu_affinity
	flag := {
		"id": "ova.cpu_affinity.detected",
		"category": "Warning",
		"label": "CPU affinity detected",
		"assessment": "The VM will be migrated without CPU affinity, but administrators can set it after migration.",
	}
}
