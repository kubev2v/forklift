package io.konveyor.forklift.vmware

import rego.v1

# Test data: Windows VM with valid disk mappings (should not trigger)
test_windows_vm_valid_disks_no_concerns if {
	count(concerns) == 0 with input as {
		"guestId": "windows2019Server_64Guest",
		"guestDisks": [
			{
				"key": 2000,
				"diskPath": "[datastore1] VM1/VM1.vmdk",
				"capacity": 21474836480,
			},
			{
				"key": 2001,
				"diskPath": "[datastore1] VM1/VM1_1.vmdk",
				"capacity": 42949672960,
			},
		],
	}
}

# Test: Windows VM with invalid disk mapping (key == 0) should trigger concern
test_windows_vm_invalid_disk_creates_concern if {
	count(concerns) == 1 with input as {
		"guestId": "windows2019Server_64Guest",
		"guestDisks": [{
			"key": 0,
			"diskPath": "[datastore1] VM1/VM1.vmdk",
			"capacity": 21474836480,
		}],
	}
}

# Test: Windows VM with missing key property should trigger concern
test_windows_vm_missing_key_property_creates_concern if {
	count(concerns) == 1 with input as {
		"guestId": "windows2019Server_64Guest",
		"guestDisks": [{
			# "key" intentionally omitted
			"diskPath": "[datastore1] VM1/VM1.vmdk",
			"capacity": 21474836480,
		}],
	}
}

# Test: Non-Windows VM with invalid disk mapping (key == 0) should not trigger
test_non_windows_vm_invalid_disk_no_concern if {
	count(concerns) == 0 with input as {
		"guestId": "rhel10_64guest",
		"guestDisks": [{
			"key": 0,
			"diskPath": "[datastore1] VM1/VM1.vmdk",
			"capacity": 21474836480,
		}],
	}
}

# Test: Windows VM with mixed valid/invalid disks should only flag invalid ones
test_windows_vm_mixed_disks_partial_concerns if {
	count(concerns) == 2 with input as {
		"guestId": "windows2016Server_64Guest",
		"guestDisks": [
			{
				"key": 2000,
				"diskPath": "[datastore1] VM1/VM1.vmdk",
				"capacity": 21474836480,
			},
			{
				"key": 0,
				"diskPath": "[datastore1] VM1/VM1_1.vmdk",
				"capacity": 42949672960,
			},
			{
				"key": 2002,
				"diskPath": "[datastore1] VM1/VM1_2.vmdk",
				"capacity": 10737418240,
			},
			{
				"key": 0,
				"diskPath": "[datastore1] VM1/VM1_3.vmdk",
				"capacity": 5368709120,
			},
		],
	}
}

# Test: Windows VM identifier case insensitive matching
test_windows_vm_case_insensitive_matching if {
	count(concerns) == 1 with input as {
		"guestId": "WINDOWS2019SERVER_64GUEST",
		"guestDisks": [{
			"key": 0,
			"diskPath": "[datastore1] VM1/VM1.vmdk",
			"capacity": 21474836480,
		}],
	}
}

# Test: Various Windows guest ID patterns should all match
test_various_windows_guest_ids if {
	# Test Windows Server
	count(concerns) == 1 with input as {
		"guestId": "windows2022Server_64Guest",
		"guestDisks": [{"key": 0, "diskPath": "[ds1] vm/disk.vmdk", "capacity": 1000}],
	}

	# Test Windows 10
	count(concerns) == 1 with input as {
		"guestId": "windows10_64Guest",
		"guestDisks": [{"key": 0, "diskPath": "[ds1] vm/disk.vmdk", "capacity": 1000}],
	}

	# Test Windows 11
	count(concerns) == 1 with input as {
		"guestId": "windows11_64Guest",
		"guestDisks": [{"key": 0, "diskPath": "[ds1] vm/disk.vmdk", "capacity": 1000}],
	}
}

# Test: Empty guest disks array should not cause errors
test_empty_guest_disks_no_concerns if {
	count(concerns) == 0 with input as {
		"guestId": "windows2019Server_64Guest",
		"guestDisks": [],
	}
}

# Test: Verify concern structure and content
test_concern_structure_and_content if {
	concerns[_].id == "vmware.guestDisks.key.not_found" with input as {
		"guestId": "windows2019Server_64Guest",
		"guestDisks": [{
			"key": 0,
			"diskPath": "[datastore1] VM1/VM1.vmdk",
			"capacity": 21474836480,
		}],
	}

	concerns[_].category == "Information" with input as {
		"guestId": "windows2019Server_64Guest",
		"guestDisks": [{
			"key": 0,
			"diskPath": "[datastore1] VM1/VM1.vmdk",
			"capacity": 21474836480,
		}],
	}

	contains(concerns[_].label, "Missing disk key mapping") with input as {
		"guestId": "windows2019Server_64Guest",
		"guestDisks": [{
			"key": 0,
			"diskPath": "[datastore1] VM1/VM1.vmdk",
			"capacity": 21474836480,
		}],
	}

	contains(concerns[_].assessment, "winDriveLetter cannot be resolved") with input as {
		"guestId": "windows2019Server_64Guest",
		"guestDisks": [{
			"key": 0,
			"diskPath": "[datastore1] VM1/VM1.vmdk",
			"capacity": 21474836480,
		}],
	}
}

# Test: Verify invalid_guest_disk_mappings rule works correctly
test_invalid_guest_disk_mappings_rule if {
	# Should find invalid mappings for Windows VM
	count(invalid_guest_disk_mappings) == 1 with input as {
		"guestId": "windows2019Server_64Guest",
		"guestDisks": [{
			"key": 0,
			"diskPath": "[datastore1] VM1/VM1.vmdk",
			"capacity": 21474836480,
		}],
	}

	# Should not find invalid mappings for non-Windows VM
	count(invalid_guest_disk_mappings) == 0 with input as {
		"guestId": "ubuntu64Guest",
		"guestDisks": [{
			"key": 0,
			"diskPath": "[datastore1] VM1/VM1.vmdk",
			"capacity": 21474836480,
		}],
	}

	# Should not find invalid mappings for Windows VM with valid keys
	count(invalid_guest_disk_mappings) == 0 with input as {
		"guestId": "windows2019Server_64Guest",
		"guestDisks": [{
			"key": 2000,
			"diskPath": "[datastore1] VM1/VM1.vmdk",
			"capacity": 21474836480,
		}],
	}
}
