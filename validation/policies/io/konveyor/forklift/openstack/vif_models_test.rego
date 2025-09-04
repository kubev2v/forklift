package io.konveyor.forklift.openstack

import rego.v1

test_with_no_vif_model if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_supported_e1000 if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"hw_vif_model": "e1000"}},
	}
	results = concerns with input as mock_vm
	count(results) == 0
}

test_with_unsupported_virtual_e1000 if {
	mock_vm := {
		"name": "test",
		"image": {"properties": {"hw_vif_model": "VirtualE1000"}},
	}
	results = concerns with input as mock_vm
	count(results) == 1
}
