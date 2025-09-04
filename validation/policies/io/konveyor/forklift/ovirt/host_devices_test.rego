package io.konveyor.forklift.ovirt

import rego.v1

test_without_host_devices if {
	mock_vm := {"name": "test", "hostDevices": []}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_host_devices if {
	mock_vm := {
		"name": "test",
		"hostDevices": [{"capability": "thing"}],
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
