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
		"assessment": "RDM disk detected. RDM is supported via the RDMAsLun option at the plan or per-VM level. If RDMAsLun is not enabled, the VM cannot be migrated unless the RDM disks are removed.",
	}
}
