package io.konveyor.forklift.vmware
import future.keywords.in

# Find guest disks with BTRFS filesystem
btrfs_disks[idx] {
    some idx
    
    # Check if the filesystem type contains 'btrfs' (case insensitive)
    lower_fs_type := lower(object.get(input.guestDisks[idx], "filesystemType", ""))
    contains(lower_fs_type, "btrfs")
}

# Raise a concern for each BTRFS disk
concerns[flag] {
    btrfs_disks[idx]
    disk := input.guestDisks[idx]
    flag := {
        "id": sprintf("vmware.guestDisks.btrfs.unsupported.%v", [idx]),
        "category": "Warning",
        "label": "BTRFS filesystem detected on disk",
        "assessment": "BTRFS filesystem is not supported and may cause issues during guest conversion"
    }
}
