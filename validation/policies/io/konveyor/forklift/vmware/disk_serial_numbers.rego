package io.konveyor.forklift.vmware

disk_uuid_enabled {
    some i
    input.diskEnableUuid == true
    input.disks[i].bus == "scsi"
}

concerns[flag] {
    disk_uuid_enabled
    flag := {
        "category": "Information",
	"label": "Disk serial numbers may be truncated",
	"assessment": "This VM is configured with at least one SCSI disk and the disk.EnableUUID parameter is set to TRUE. This may indicate a need for consistent SCSI disk serial numbers, but be advised that these serial numbers will be truncated after migration."
    }
}
