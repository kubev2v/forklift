package io.konveyor.forklift.ovirt

import rego.v1

invalid_disks contains idx if {
    some idx
    disk := input.diskAttachments[idx].disk
    is_disk_invalid(disk)
}

is_disk_invalid(disk) if {
    is_lun(disk)
    is_lun_missing_size(disk)
}

is_disk_invalid(disk) if {
    not is_lun(disk)
    disk.provisionedSize <= 0
}

is_lun(disk) if {
    disk.storageType == "lun"
}

is_lun_missing_size(disk) if {
    some unit in disk.lun.logicalUnits.logicalUnit
    unit.size <= 0
}

# Raise a concern for each invalid disk
concerns contains flag if {
    invalid_disks[idx]
    disk := input.diskAttachments[idx].disk

    flag := {
       "id": "ovirt.disk.capacity.invalid",
       "category": "Critical",
       "label": "Disk has an invalid capacity",
       "assessment": "Disk has an invalid capacity",
    }
}
