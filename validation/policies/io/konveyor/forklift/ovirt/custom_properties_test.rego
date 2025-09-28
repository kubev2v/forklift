package io.konveyor.forklift.ovirt

import rego.v1

test_without_vm_custom_properties if {
	mock_vm := {
		"name": "test",
		"properties": [],
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_vm_custom_properties if {
	mock_vm := {
		"name": "test",
		"properties": [{
			"name": "viodiskcache",
			"value": "writeback",
		}],
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
