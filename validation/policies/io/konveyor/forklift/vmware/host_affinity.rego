package io.konveyor.forklift.vmware

import rego.v1

has_host_affinity if {
	some i
	input.host.cluster.hostAffinityVms[i].id == input.id
}

concerns contains flag if {
	has_host_affinity
	flag := {
		"id": "vmware.host_affinity.detected",
		"category": "Warning",
		"label": "VM-Host affinity detected",
		"assessment": "The VM will be migrated without node affinity, but administrators can set it after migration.",
	}
}
