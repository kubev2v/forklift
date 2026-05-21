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
		"category": "Warning",
		"label": "Raw Device Mapped disk detected",
		"assessment": "RDM disks are not supported when using VDDK transfer. If copy-offload (XCOPY) is enabled in the migration plan, RDM disks can be migrated. Otherwise, the RDM disks must be removed before migration and reattached after.",
	}
}
