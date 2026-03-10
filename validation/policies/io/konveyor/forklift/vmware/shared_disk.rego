package io.konveyor.forklift.vmware

import rego.v1

has_shared_disk if {
	some i
	input.disks[i].shared == true
}

concerns contains flag if {
	has_shared_disk
	flag := {
		"id": "vmware.disk.shared.detected",
		"category": "Warning",
		"label": "Shared disk detected",
		"assessment": "The VM has a disk that is shared with another VM. Shared disks require special handling during migration.",
	}
}
