package io.konveyor.forklift.vmware

import rego.v1

disk_uuid_enabled if {
	some i
	input.diskEnableUuid == true
	input.disks[i].bus == "scsi"
}

concerns contains flag if {
	disk_uuid_enabled
	flag := {
		"id": "vmware.disk_serial.truncated",
		"category": "Information",
		"label": "Disk serial numbers may be truncated",
		"assessment": "This VM is configured with at least one SCSI disk and the disk.EnableUUID parameter is set to TRUE. This may indicate a need for consistent SCSI disk serial numbers, but be advised that these serial numbers will be truncated after migration.",
	}
}
