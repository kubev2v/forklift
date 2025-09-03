package io.konveyor.forklift.vmware
import future.keywords.in


# Minimum free space required, converted to bytes for easy comparison.
# Source of the requirements https://libguestfs.org/virt-v2v.1.html#disk-space
MIN_SPACE_ROOT_BYTES := 100 * 1024 * 1024
MIN_SPACE_BOOT_BYTES := 50 * 1024 * 1024
MIN_SPACE_WINDOWS_C_BYTES := 100 * 1024 * 1024
MIN_SPACE_OTHER_BYTES := 10 * 1024 * 1024

failing_disks_by_space[info] {
    some idx
    disk := input.guestDisks[idx]
    path := disk.diskPath
    free_space := disk.freeSpace

    path == "/"
    free_space < MIN_SPACE_ROOT_BYTES
    info := {"disk": disk, "required_mb": 100}
}

failing_disks_by_space[info] {
    some idx
    disk := input.guestDisks[idx]
    path := disk.diskPath
    free_space := disk.freeSpace

    path == "/boot"
    free_space < MIN_SPACE_BOOT_BYTES
    info := {"disk": disk, "required_mb": 50}
}

failing_disks_by_space[info] {
    some idx
    disk := input.guestDisks[idx]
    path := disk.diskPath
    free_space := disk.freeSpace

    lower(path) == "c:\\"
    free_space < MIN_SPACE_WINDOWS_C_BYTES
    info := {"disk": disk, "required_mb": 100}
}

failing_disks_by_space[info] {
    some idx
    disk := input.guestDisks[idx]
    path := disk.diskPath
    free_space := disk.freeSpace

    # Check any other mountable filesystem not covered by the rules above.
    not is_special_disk(path)
    free_space < MIN_SPACE_OTHER_BYTES
    info := {"disk": disk, "required_mb": 10}
}

# Helper function to identify the specific disk paths that have their own rules.
is_special_disk(path) {
    path == "/"
} else {
    path == "/boot"
} else {
    lower(path) == "c:\\"
}

concerns[flag] {
    failing_disks_by_space[info]

    disk := info.disk
    required_mb := info.required_mb
    free_mb := round(disk.freeSpace / (1024 * 1024))
    flag := {
        "id": "vmware.guestDisks.freespace",
        "category": "Critical",
        "label": sprintf("Insufficient free space for conversion on '%s'", [disk.diskPath]),
        "assessment": sprintf(
            "The guest filesystem '%s' has %v MB of free space, but a minimum of %v MB is required for conversion. Free up space on this filesystem before migration.",
            [disk.diskPath, free_mb, required_mb]
        )
    }
}