package io.konveyor.forklift.vmware

import rego.v1

independent_disk if {
	some i
	input.disks[i].mode in ["independent_persistent", "independent_nonpersistent"]
}

concerns contains flag if {
	independent_disk
	flag := {
		"id": "vmware.disk_mode.independent",
		"category": "Critical",
		"label": "Independent disk detected",
		"assessment": "Independent disks cannot be transferred using recent versions of VDDK. The VM cannot be migrated unless disks are changed to 'Dependent' mode in VMware.",
	}
}
