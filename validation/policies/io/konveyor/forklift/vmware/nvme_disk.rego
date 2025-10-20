package io.konveyor.forklift.vmware

import rego.v1

has_nvme_bus if {
	some i
	input.disks[i].bus == "nvme"
}

concerns contains flag if {
	has_nvme_bus
	flag := {
		"id": "vmware.disk.nvme.detected",
		"category": "Critical",
		"label": "Disk NVMe was detected",
		"assessment": "NVMe disks are not currently supported by MTV. The VM cannot be migrated",
	}
}
