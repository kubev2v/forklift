package io.konveyor.forklift.openstack

import rego.v1

test_with_first_valid_disk_interface_type if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"hw_disk_bus": "sata"}},
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_second_valid_disk_interface_type if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"hw_disk_bus": "scsi"}},
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_third_valid_disk_interface_type if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"hw_disk_bus": "virtio"}},
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_invalid_disk_interface_type if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"hw_disk_bus": "ide"}},
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
