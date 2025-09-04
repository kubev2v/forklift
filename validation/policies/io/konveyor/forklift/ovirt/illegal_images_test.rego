package io.konveyor.forklift.ovirt

import rego.v1

test_without_illegal_images if {
	mock_vm := {
		"name": "test",
		"hasIllegalImages": false,
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_illegal_images if {
	mock_vm := {
		"name": "test",
		"hasIllegalImages": true,
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
