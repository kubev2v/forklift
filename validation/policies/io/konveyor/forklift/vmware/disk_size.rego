package io.konveyor.forklift.vmware
import future.keywords.in

# Match any disk with zero or negative capacity
invalid_disks[idx] {
    some idx
    input.disks[idx].capacity <= 0
}

# Raise a concern for each invalid disk
concerns[flag] {
    invalid_disks[idx]
    disk := input.disks[idx]
    flag := {
        "id": "vmware.disk.capacity.invalid",
        "category": "Critical",
        "label": sprintf("Disk '%v' has an invalid capacity of %v bytes", [disk.file, disk.capacity]),
        "assessment": sprintf("Disk '%v' has a capacity of %v bytes, which is not allowed. Capacity must be greater than zero.", [disk.file, disk.capacity])
    }
}
