package io.konveyor.forklift.openstack

import rego.v1

# CNV limits the number of disks per VM to 165 (CNV-91554).
max_disk_count := 165

has_too_many_disks if {
	count(input.volumes) > max_disk_count
}

concerns contains flag if {
	has_too_many_disks
	flag := {
		"id": "openstack.disk.count.exceeds.limit",
		"category": "Warning",
		"label": sprintf("VM has %d disks, which exceeds the limit of %d", [count(input.volumes), max_disk_count]),
		"assessment": sprintf("The VM has %d disks but the target platform supports a maximum of %d disks per VM. Migration may result in network connectivity issues.", [count(input.volumes), max_disk_count]),
	}
}
