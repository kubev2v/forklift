package io.konveyor.forklift.vmware

import rego.v1

# Test: VM with no BTRFS disks should not trigger concerns
test_vm_without_btrfs_no_concerns if {
	count(concerns) == 0 with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [
			{
				"key": 2000,
				"diskPath": "/dev/sda1",
				"capacity": 21474836480,
				"filesystemType": "ext4",
			},
			{
				"key": 2001,
				"diskPath": "/dev/sda2",
				"capacity": 42949672960,
				"filesystemType": "xfs",
			},
		],
	}
}

# Test: VM with BTRFS disk should trigger concern
test_vm_with_btrfs_creates_concern if {
	count(concerns) == 1 with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [{
			"key": 2000,
			"diskPath": "/dev/sda1",
			"capacity": 21474836480,
			"filesystemType": "btrfs",
		}],
	}
}

# Test: Case insensitive BTRFS detection
test_btrfs_case_insensitive if {
	count(concerns) == 1 with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [
			{
				"key": 2000,
				"diskPath": "/dev/sda1",
				"capacity": 21474836480,
				"filesystemType": "btrfs",
			},
			{
				"key": 2001,
				"diskPath": "/dev/sda2",
				"capacity": 42949672960,
				"filesystemType": "BTRFS",
			},
			{
				"key": 2002,
				"diskPath": "/dev/sda3",
				"capacity": 10737418240,
				"filesystemType": "BtrFS",
			},
		],
	}
}

# Test: Mixed filesystem types should only flag BTRFS disks
test_mixed_filesystems_partial_concerns if {
	count(concerns) == 1 with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [
			{
				"key": 2000,
				"diskPath": "/dev/sda1",
				"capacity": 21474836480,
				"filesystemType": "ext4",
			},
			{
				"key": 2001,
				"diskPath": "/dev/sda2",
				"capacity": 42949672960,
				"filesystemType": "btrfs",
			},
			{
				"key": 2002,
				"diskPath": "/dev/sda3",
				"capacity": 10737418240,
				"filesystemType": "xfs",
			},
			{
				"key": 2003,
				"diskPath": "/dev/sda4",
				"capacity": 5368709120,
				"filesystemType": "BTRFS",
			},
		],
	}
}

# Test: Empty guest disks array should not cause errors
test_empty_guest_disks_no_concerns if {
	count(concerns) == 0 with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [],
	}
}

# Test: Missing filesystemType field should not trigger concern
test_missing_filesystem_type_no_concern if {
	count(concerns) == 0 with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [{
			"key": 2000,
			"diskPath": "/dev/sda1",
			"capacity": 21474836480,
			# "filesystemType" intentionally omitted
		}],
	}
}

# Test: Empty filesystemType should not trigger concern
test_empty_filesystem_type_no_concern if {
	count(concerns) == 0 with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [{
			"key": 2000,
			"diskPath": "/dev/sda1",
			"capacity": 21474836480,
			"filesystemType": "",
		}],
	}
}

# Test: Verify concern structure and content
test_concern_structure_and_content if {
	startswith(concerns[_].id, "vmware.guestDisks.btrfs.unsupported") with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [{
			"key": 2000,
			"diskPath": "/dev/sda1",
			"capacity": 21474836480,
			"filesystemType": "btrfs",
		}],
	}

	concerns[_].category == "Warning" with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [{
			"key": 2000,
			"diskPath": "/dev/sda1",
			"capacity": 21474836480,
			"filesystemType": "btrfs",
		}],
	}

	contains(concerns[_].label, "BTRFS filesystem detected") with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [{
			"key": 2000,
			"diskPath": "/dev/sda1",
			"capacity": 21474836480,
			"filesystemType": "btrfs",
		}],
	}

	contains(concerns[_].assessment, "is not supported") with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [{
			"key": 2000,
			"diskPath": "/dev/sda1",
			"capacity": 21474836480,
			"filesystemType": "btrfs",
		}],
	}
}

# Test: Verify btrfs_disks rule works correctly
test_btrfs_disks_rule if {
	# Should find BTRFS disks
	count(btrfs_disks) == 1 with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [{
			"key": 2000,
			"diskPath": "/dev/sda1",
			"capacity": 21474836480,
			"filesystemType": "btrfs",
		}],
	}

	# Should not find non-BTRFS disks
	count(btrfs_disks) == 0 with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [{
			"key": 2000,
			"diskPath": "/dev/sda1",
			"capacity": 21474836480,
			"filesystemType": "ext4",
		}],
	}

	# Should handle multiple BTRFS disks
	count(btrfs_disks) == 2 with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [
			{
				"key": 2000,
				"diskPath": "/dev/sda1",
				"capacity": 21474836480,
				"filesystemType": "btrfs",
			},
			{
				"key": 2001,
				"diskPath": "/dev/sda2",
				"capacity": 42949672960,
				"filesystemType": "BTRFS",
			},
		],
	}
}
