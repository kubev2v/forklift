package io.konveyor.forklift.openstack

test_with_no_vif_model {
	mock_vm := {
		"name": "test",
		"image": {"properties": {}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_supported_e1000 {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"hw_vif_model": "e1000"}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_unsupported_virtual_e1000 {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"hw_vif_model": "VirtualE1000"}},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
