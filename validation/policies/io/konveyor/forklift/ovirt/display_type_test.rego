package io.konveyor.forklift.ovirt

import rego.v1

test_without_spice_enabled if {
	mock_vm := {
		"name": "test",
		"display": "vnc",
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_spice_enabled if {
	mock_vm := {
		"name": "test",
		"display": "spice",
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
