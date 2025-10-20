package io.konveyor.forklift.vmware

import rego.v1

has_rdm_disk if {
	some i
	input.disks[i].rdm
}

concerns contains flag if {
	has_rdm_disk
	flag := {
		"id": "vmware.disk.rdm.detected",
		"category": "Critical",
		"label": "Raw Device Mapped disk detected",
		"assessment": "RDM disks are not currently supported by Migration Toolkit for Virtualization. The VM cannot be migrated unless the RDM disks are removed. You can reattach them to the VM after migration.",
	}
}
