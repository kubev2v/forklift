package io.konveyor.forklift.vmware

import rego.v1

# Match any disk with zero or negative capacity
invalid_disks contains idx if {
	some idx
	input.disks[idx].capacity <= 0
}

# Raise a concern for each invalid disk
concerns contains flag if {
	invalid_disks[idx]
	disk := input.disks[idx]
	flag := {
		"id": "vmware.disk.capacity.invalid",
		"category": "Critical",
		"label": sprintf("Disk '%v' has an invalid capacity of %v bytes", [disk.file, disk.capacity]),
		"assessment": sprintf("Disk '%v' has a capacity of %v bytes, which is not allowed. Capacity must be greater than zero.", [disk.file, disk.capacity]),
	}
}
