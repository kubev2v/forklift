package io.konveyor.forklift.openstack

test_without_cpushares_defined {
	mock_vm := {
		"name": "test",
		"flavor": {"extraSpecs": {}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_cpushares_enabled {
	mock_vm := {
		"name": "test",
		"flavor": {"extraSpecs": {"quota:cpu_shares": "1000"}},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}

test_with_cpushares_empty {
	mock_vm := {
		"name": "test",
		"flavor": {"extraSpecs": {"quota:cpu_shares": ""}},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
