package io.konveyor.forklift.vmware

import rego.v1

has_dpm_enabled if {
	input.host.cluster.dpmEnabled
}

concerns contains flag if {
	has_dpm_enabled
	flag := {
		"id": "vmware.dpm.enabled",
		"category": "Information",
		"label": "vSphere DPM detected",
		"assessment": "Distributed Power Management is not currently supported by OpenShift Virtualization. The VM can be migrated but it will not have this feature in the target environment. ",
	}
}
