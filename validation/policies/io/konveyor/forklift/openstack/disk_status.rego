package io.konveyor.forklift.openstack

valid_disk_status[i] {
	some i
	regex.match(`available|in-use`, input.volumes[i].status)
}

concerns[flag] {
	count(valid_disk_status) != count(input.volumes)
	flag := {
		"id": "openstack.disk.status.unsupported",
		"category": "Critical",
		"label": "VM has one or more disks with an unsupported status",
		"assessment": "One or more of the VM's disks has an unsupported status condition. The VM disk transfer is likely to fail.",
	}
}
