package io.konveyor.forklift.openstack

import rego.v1

test_without_host_devices if {
	mock_vm := {
		"name": "test",
		"flavor": {"extraSpecs": {}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_host_devices if {
	mock_vm := {
		"name": "test",
		"flavor": {"extraSpecs": {"pci_passthrough:alias": "alias1:2"}},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
