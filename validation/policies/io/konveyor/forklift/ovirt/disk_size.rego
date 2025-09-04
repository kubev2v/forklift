package io.konveyor.forklift.ovirt

import rego.v1

# Match any disk with zero or negative provisioned size
invalid_disks contains idx if {
	some idx
	input.diskAttachments[idx].disk.provisionedSize <= 0
}

# Raise a concern for each invalid disk
concerns contains flag if {
	invalid_disks[idx]
	disk := input.diskAttachments[idx].disk
	flag := {
		"id": "ovirt.disk.capacity.invalid",
		"category": "Critical",
		"label": sprintf("Disk has an invalid capacity of %v bytes", [disk.provisionedSize]),
		"assessment": sprintf("Disk has a provisioned size of %v bytes, which is not allowed. Capacity must be greater than zero.", [disk.provisionedSize]),
	}
}
