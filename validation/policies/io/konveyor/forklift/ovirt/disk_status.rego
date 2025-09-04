package io.konveyor.forklift.ovirt

import rego.v1

invalid_disk_status contains i if {
	some i
	regex.match(`illegal|locked`, input.diskAttachments[i].disk.status)
}

concerns contains flag if {
	count(invalid_disk_status) > 0
	flag := {
		"id": "ovirt.disk.illegal_or_locked_status",
		"category": "Critical",
		"label": "VM has an illegal or locked disk status condition",
		"assessment": "One or more of the VM's disks has an illegal or locked status condition. The VM disk transfer is likely to fail.",
	}
}
