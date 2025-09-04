package io.konveyor.forklift.openstack

import rego.v1

test_without_boot_menu_enabled if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"hw_boot_menu": "false"}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_boot_menu_enabled if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"hw_boot_menu": "true"}},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
