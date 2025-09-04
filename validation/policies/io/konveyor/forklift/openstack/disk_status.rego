package io.konveyor.forklift.openstack

import rego.v1

valid_disk_status contains i if {
	some i
	regex.match(`available|in-use`, input.volumes[i].status)
}

concerns contains flag if {
	count(valid_disk_status) != count(input.volumes)
	flag := {
		"id": "openstack.disk.status.unsupported",
		"category": "Critical",
		"label": "VM has one or more disks with an unsupported status",
		"assessment": "One or more of the VM's disks has an unsupported status condition. The VM disk transfer is likely to fail.",
	}
}
