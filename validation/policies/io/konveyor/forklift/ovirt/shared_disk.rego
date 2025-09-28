package io.konveyor.forklift.ovirt

import rego.v1

shared_disks contains i if {
	some i
	input.diskAttachments[i].disk.shared == true
}

concerns contains flag if {
	count(shared_disks) > 0
	flag := {
		"id": "ovirt.disk.shared.detected",
		"category": "Warning",
		"label": "Shared disk detected",
		"assessment": "The VM has a disk that is shared. Shared disks are not currently supported by OpenShift Virtualization.",
	}
}
