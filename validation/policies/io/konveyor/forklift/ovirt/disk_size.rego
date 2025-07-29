package io.konveyor.forklift.ovirt
import future.keywords.in

# Match any disk with zero or negative provisioned size
invalid_disks[idx] {
    some idx
    input.diskAttachments[idx].disk.provisionedSize <= 0
}

# Raise a concern for each invalid disk
concerns[flag] {
    invalid_disks[idx]
    disk := input.diskAttachments[idx].disk
    flag := {
        "id": "ovirt.disk.capacity.invalid",
        "category": "Critical",
        "label": sprintf("Disk has an invalid capacity of %v bytes", [disk.provisionedSize]),
        "assessment": sprintf("Disk has a provisioned size of %v bytes, which is not allowed. Capacity must be greater than zero.", [disk.provisionedSize])
    }
} 