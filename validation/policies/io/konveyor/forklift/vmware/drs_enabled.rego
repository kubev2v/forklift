package io.konveyor.forklift.vmware

import rego.v1

has_drs_enabled if {
	input.host.cluster.drsEnabled
}

concerns contains flag if {
	has_drs_enabled
	flag := {
		"id": "vmware.drs.enabled",
		"category": "Information",
		"label": "VM running in a DRS-enabled cluster",
		"assessment": "Distributed resource scheduling is not currently supported by Migration Toolkit for Virtualization. The VM can be migrated but it will not have this feature in the target environment.",
	}
}
