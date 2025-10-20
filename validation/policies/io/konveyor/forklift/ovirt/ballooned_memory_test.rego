package io.konveyor.forklift.ovirt

import rego.v1

test_without_ballooned_memory if {
	mock_vm := {
		"name": "test",
		"balloonedMemory": false,
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_ballooned_memory if {
	mock_vm := {
		"name": "test",
		"balloonedMemory": true,
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
