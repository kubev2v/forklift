package io.konveyor.forklift.vmware

import rego.v1

null_datastore if {
	some i
	count(input.disks[i].datastore.id) == 0
}

concerns contains flag if {
	null_datastore
	flag := {
		"id": "vmware.datastore.missing",
		"category": "Critical",
		"label": "Disk is not located on a datastore",
		"assessment": "The VM is configured with a disk that is not located on a datastore. The VM cannot be migrated.",
	}
}
