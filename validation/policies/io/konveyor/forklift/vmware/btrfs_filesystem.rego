package io.konveyor.forklift.vmware

import rego.v1

# Find guest disks with BTRFS filesystem
btrfs_disks contains idx if {
	some idx

	# Check if the filesystem type contains 'btrfs' (case insensitive)
	lower_fs_type := lower(object.get(input.guestDisks[idx], "filesystemType", ""))
	contains(lower_fs_type, "btrfs")
}

# Raise a concern for each BTRFS disk
concerns contains flag if {
	btrfs_disks[idx]
	disk := input.guestDisks[idx]
	flag := {
		"id": "vmware.guestDisks.btrfs.unsupported",
		"category": "Warning",
		"label": "BTRFS filesystem detected on disk",
		"assessment": "BTRFS filesystem is not supported and may cause issues during guest conversion",
	}
}
