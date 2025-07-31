package io.konveyor.forklift.vmware
import future.keywords.in

# Match any windows disk with missing mappings
invalid_guest_disk_mappings[idx] {
    some idx

    lower_id := lower(input.guestId)
    is_windows := regex.match(`.*windows.*`, lower_id)
    missing_disk_mapping := input.guestDisks[idx].key == 0

    is_windows
    missing_disk_mapping
}

# Raise a concern for each invalid disk
concerns[flag] {
    invalid_guest_disk_mappings[idx]
    disk := input.guestDisks[idx]
    flag := {
        "id": "vmware.guestDisks.key.not_found",
        "category": "Information",
        "label": sprintf("Disk '%v' has an invalid disk key mapping", [disk.diskPath]),
        "assessment": sprintf("Disk '%v' has a disk key of 0, indicating a missing or invalid disk key mapping. This will prevent the winDriveLetter key from being properly resolved in PVC name templates.", [disk.diskPath])
    }
}
