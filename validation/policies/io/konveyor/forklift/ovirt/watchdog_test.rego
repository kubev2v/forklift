package io.konveyor.forklift.ovirt

import rego.v1

test_without_watchdog if {
	mock_vm := {
		"name": "test",
		"watchDogs": [],
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_watchdog if {
	mock_vm := {
		"name": "test",
		"watchDogs": [{"model": "i6300esb", "action": "reset"}],
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
