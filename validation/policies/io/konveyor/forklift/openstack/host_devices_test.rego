package io.konveyor.forklift.openstack

test_without_host_devices {
	mock_vm := {
		"name": "test",
		"flavor": {"extraSpecs": {}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_host_devices {
	mock_vm := {
		"name": "test",
		"flavor": {"extraSpecs": {"pci_passthrough:alias": "alias1:2"}},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
